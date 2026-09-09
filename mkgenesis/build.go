package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

const denom = "ugnot"

// entry is the running total for one address, plus the two facts that decide
// what vesting schedule its row ends up carrying.
//
// unlocked is the part of the balance that is liquid at genesis and therefore
// NOT subject to the common §132 schedule — today that is exactly what came
// from publicsale.txt. It is tracked per address rather than as an exempt-list
// because 25 of the 67 sale participants ALSO hold an airdrop entitlement at
// the same address: exempting the whole row would let their airdrop ride free,
// and vesting the whole row would lock up a sale allocation that is contractually
// liquid. Subtracting gives the only answer that is right on both halves.
//
// schedule is a fully-formed ";vesting=…" suffix declared by an input row, for
// the case the common schedule cannot express — a single US-accredited sale
// participant under a 12-month cliff. It is emitted verbatim.
type entry struct {
	amount   int64
	unlocked int64
	schedule string
}

// row is one output line: an address and the ugnot it holds.
//
// line is carried alongside amount because it is both what gets written and
// what the sort tie-break compares; rendering it once avoids rebuilding it for
// every comparison. addr reuses the map key, so it costs no allocation.
type row struct {
	amount   int64
	unlocked int64
	schedule string
	addr     string
	line     string
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	genbalance := fs.String("genbalance", "../allocate/genbalance.txt.gz", "gzipped airdrop rows, <source>:<addr>=<amount>ugnot")
	premine := fs.String("premine", "non-airdrop.txt", "hand-written premine rows, # starts a comment")
	publicsale := fs.String("publicsale", "publicsale.txt", "public token sale rows, unlocked at genesis; empty to skip")
	out := fs.String("out", "balances.txt", "merged output")
	vestingStart := fs.Int64("vesting-start", 0, "unix seconds GNOT becomes transferrable; 0 disables vesting")
	vestingEnd := fs.Int64("vesting-end", 0, "unix seconds the schedule completes, normally start + 24 months")
	vestingUnlockPct := fs.Int64("vesting-unlock-pct", 4, "percent unlocked at -vesting-start")
	vestingExempt := fs.String("vesting-exempt", "", "addresses that receive no schedule, space- or comma-separated")
	if err := fs.Parse(args); err != nil {
		return err
	}

	vest, err := newVesting(*vestingStart, *vestingEnd, *vestingUnlockPct, *vestingExempt)
	if err != nil {
		return err
	}

	totals := make(map[string]entry)
	if err := readPremine(*premine, totals); err != nil {
		return err
	}
	if *publicsale != "" {
		if err := readPublicSale(*publicsale, totals); err != nil {
			return err
		}
	}
	if err := readGenbalance(*genbalance, totals); err != nil {
		return err
	}

	rows := sortRows(totals)
	if err := writeRows(*out, rows, vest); err != nil {
		return err
	}

	var total int64
	for _, r := range rows {
		total += r.amount
	}
	fmt.Printf("%s: %d rows, %d %s\n", *out, len(rows), total, denom)
	fmt.Println(vest.describe())
	warnDroppedSchedules(rows, vest)
	return nil
}

// warnDroppedSchedules shouts when a row declared its own vesting schedule and
// the build threw it away because vesting is off.
//
// Losing a schedule silently is the one failure mode this pipeline cannot
// afford: the row still looks perfectly well-formed, `gnogenesis verify` still
// passes, and the only symptom is that a transfer restriction someone is legally
// on the hook for does not exist. With vesting off the restriction has to be
// honoured by hand instead, so it has to be said out loud.
func warnDroppedSchedules(rows []row, vest *vesting) {
	if vest != nil {
		return
	}
	for _, r := range rows {
		if r.schedule == "" {
			continue
		}
		fmt.Printf("WARNING: %s declared %q and vesting is OFF, so the schedule was DROPPED — that restriction must now be enforced by hand\n",
			r.addr, r.schedule)
	}
}

// readPremine adds the hand-written rows. Everything from the first '#' is a
// comment, and lines that are blank once it is stripped are skipped — the same
// rows `cut -d'#' -f1 | grep -vE '^\s*$'` used to keep.
func readPremine(path string, totals map[string]entry) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return accumulate(f, path, totals, stripComment, false)
}

// readPublicSale adds the public token sale rows. Same grammar as the premine
// file, with two differences that matter downstream: everything it contributes
// is liquid at genesis, and a row may declare its own vesting schedule.
func readPublicSale(path string, totals map[string]entry) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return accumulate(f, path, totals, stripComment, true)
}

// stripComment drops everything from the first '#', matching what
// `cut -d'#' -f1` kept.
func stripComment(line string) string {
	if i := strings.IndexByte(line, '#'); i >= 0 {
		return line[:i]
	}
	return line
}

// readGenbalance adds the computed airdrop rows. Each line is
// <source-addr>:<gno-addr>=<amount>ugnot, and only the part after the first
// colon is a balance — the same field `cut -d: -f2` used to take.
func readGenbalance(path string, totals map[string]entry) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer zr.Close()

	return accumulate(zr, path, totals, secondColonField, false)
}

// secondColonField reproduces `cut -d: -f2`: the text between the first and
// second colon, or the whole line when there is no colon at all.
func secondColonField(line string) string {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return line
	}
	rest := line[i+1:]
	if j := strings.IndexByte(rest, ':'); j >= 0 {
		return rest[:j]
	}
	return rest
}

// accumulate sums every row into totals. Addresses appearing in several input
// files are added together, matching the gawk pass this replaced — and
// deliberately unlike the consuming side, where LeftMerge is last-write-wins.
//
// unlocked marks a whole file as liquid at genesis; see entry. A declared
// ";vesting=…" suffix is carried through, and a second one for the same address
// is an error rather than a silent overwrite — one address can hold only one
// schedule, so there is no correct way to merge two.
func accumulate(r io.Reader, name string, totals map[string]entry, prepare func(string) string, unlocked bool) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(prepare(sc.Text()))
		if line == "" {
			continue
		}

		addr, amount, err := parseRow(line)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", name, n, err)
		}

		e := totals[addr]
		sum := e.amount + amount
		if sum < e.amount {
			return fmt.Errorf("%s:%d: %s overflows int64", name, n, addr)
		}
		e.amount = sum
		if unlocked {
			e.unlocked += amount
		}
		if _, schedule := splitSchedule(line); schedule != "" {
			if e.schedule != "" {
				return fmt.Errorf("%s:%d: %s already declares the vesting schedule %q; an address carries at most one",
					name, n, addr, e.schedule)
			}
			e.schedule = schedule
		}
		totals[addr] = e
	}

	return sc.Err()
}

// splitSchedule separates a row's balance from an explicitly declared vesting
// schedule: "<addr>=<coins>;vesting=…" becomes "<addr>=<coins>" and
// ";vesting=…". The schedule is drawn from the balance, not added to it.
func splitSchedule(line string) (balance, schedule string) {
	if i := strings.IndexByte(line, ';'); i >= 0 {
		return line[:i], line[i:]
	}
	return line, ""
}

// parseRow splits "<addr>=<amount>ugnot" into its two parts. The shell version
// silently produced a zero for anything malformed; this reports it instead.
func parseRow(line string) (string, int64, error) {
	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return "", 0, fmt.Errorf("missing '=' in %q", line)
	}

	addr := line[:eq]
	if addr == "" {
		return "", 0, fmt.Errorf("empty address in %q", line)
	}

	// A vesting schedule is appended as ";vesting=<coins>,<start>,<end>". The
	// balance is what precedes it; the schedule is drawn from that balance, not
	// added to it, so it must not be counted twice.
	digits := line[eq+1:]
	if semi := strings.IndexByte(digits, ';'); semi >= 0 {
		digits = digits[:semi]
	}
	if !strings.HasSuffix(digits, denom) {
		return "", 0, fmt.Errorf("missing %q suffix in %q", denom, line)
	}
	digits = strings.TrimSuffix(digits, denom)

	amount, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("bad amount in %q: %w", line, err)
	}
	if amount < 0 {
		return "", 0, fmt.Errorf("negative amount in %q", line)
	}

	return addr, amount, nil
}

// sortRows orders by amount descending, breaking ties on the whole rendered
// line in descending byte order.
//
// That reproduces `sort -t = -k 2 -n -r`. The absence of -s there is
// load-bearing: with no stable flag, equal keys fall back to sort's
// last-resort whole-line comparison, and -r reverses that too. Comparing bytes
// here also drops the shell version's unstated dependence on LC_ALL.
func sortRows(totals map[string]entry) []row {
	rows := make([]row, 0, len(totals))
	for addr, e := range totals {
		rows = append(rows, row{
			amount:   e.amount,
			unlocked: e.unlocked,
			schedule: e.schedule,
			addr:     addr,
			line:     addr + "=" + strconv.FormatInt(e.amount, 10) + denom,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].amount != rows[j].amount {
			return rows[i].amount > rows[j].amount
		}
		return rows[i].line > rows[j].line
	})

	return rows
}

// writeRows writes the sorted rows, appending each row's vesting schedule if
// there is one. Vesting is applied here, after the sort, so that the schedule
// suffix cannot influence the row order — the same reason the shell version
// rewrote the file in place once sort had already run.
func writeRows(path string, rows []row, vest *vesting) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, r := range rows {
		if _, err := w.WriteString(vest.apply(r)); err != nil {
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}

	return f.Close()
}
