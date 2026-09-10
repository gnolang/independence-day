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
	investors := fs.String("investors", "investors.txt", "investor/partner distributions, unlocked at genesis; empty to skip")
	out := fs.String("out", "balances.txt", "merged output")
	vestingStart := fs.Int64("vesting-start", genesisVestingStart, "unix seconds GNOT becomes transferrable (§132)")
	vestingEnd := fs.Int64("vesting-end", genesisVestingEnd, "unix seconds the schedule completes, start + 24 months")
	vestingUnlockPct := fs.Int64("vesting-unlock-pct", 4, "percent unlocked at -vesting-start")
	vestingExempt := fs.String("vesting-exempt", genesisVestingExempt, "addresses that receive no schedule, space- or comma-separated")
	noVesting := fs.Bool("no-vesting", false, "build with NO §132 schedules (drafts and tests only)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// §132 covers "All Genesis $GNOT allocations", so a sheet with no schedules is
	// a violation rather than a configuration: every holder would be 100% liquid
	// at block 1 where 96% should be locked. That shipped for months because
	// vesting defaulted to OFF and nothing said so out loud.
	//
	// It is still reachable -- the tests need it, and a draft build may want it --
	// but it has to be asked for by name now.
	if *noVesting {
		*vestingStart, *vestingEnd = 0, 0
	} else if *vestingStart == 0 {
		return fmt.Errorf("refusing to build a sheet with no §132 vesting schedules: " +
			"pass -vesting-start (the genesis timestamp, per §132 \"aka the mainnet\") " +
			"and -vesting-end, or -no-vesting to say you mean it")
	}

	vest, err := newVesting(*vestingStart, *vestingEnd, *vestingUnlockPct, *vestingExempt)
	if err != nil {
		return err
	}

	totals := make(map[string]entry)
	if err := readPremine(*premine, totals); err != nil {
		return err
	}
	// Same treatment as the sale, and for the same reason: both are paid out of
	// the §136 UNLOCKED tranche, so both are liquid at genesis and neither
	// carries a §132 schedule.
	if *investors != "" {
		if err := readPublicSale(*investors, totals); err != nil {
			return err
		}
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
	reportDeclaredSchedules(rows)
	return nil
}

// reportDeclaredSchedules lists the rows that carry a schedule from the input
// data rather than from the §132 pass. These are honoured whether or not vesting
// is on -- they exist because §132 cannot express them, and for the public-sale
// forced-lockup row the restriction is a legal obligation, not a policy choice.
//
// This used to be warnDroppedSchedules, which fired when vesting was off to say
// the schedule had been discarded. That was the wrong trade: the default build
// is vesting-off, so the shipped sheet showed a forced-lockup allocation as
// fully liquid and the only trace was a line in the build log. Now they survive,
// and this prints them so the count is visible in CI output.
func reportDeclaredSchedules(rows []row) {
	n := 0
	for _, r := range rows {
		if r.schedule == "" {
			continue
		}
		n++
		fmt.Printf("declared schedule (honoured regardless of -vesting-*): %s%s\n", r.addr, r.schedule)
	}
	if n > 0 {
		fmt.Printf("declared schedules: %d\n", n)
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
