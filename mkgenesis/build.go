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

// row is one output line: an address and the ugnot it holds.
//
// line is carried alongside amount because it is both what gets written and
// what the sort tie-break compares; rendering it once avoids rebuilding it for
// every comparison. addr reuses the map key, so it costs no allocation.
type row struct {
	amount int64
	addr   string
	line   string
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	genbalance := fs.String("genbalance", "../allocate/genbalance.txt.gz", "gzipped airdrop rows, <source>:<addr>=<amount>ugnot")
	premine := fs.String("premine", "non-airdrop.txt", "hand-written premine rows, # starts a comment")
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

	totals := make(map[string]int64)
	if err := readPremine(*premine, totals); err != nil {
		return err
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
	return nil
}

// readPremine adds the hand-written rows. Everything from the first '#' is a
// comment, and lines that are blank once it is stripped are skipped — the same
// rows `cut -d'#' -f1 | grep -vE '^\s*$'` used to keep.
func readPremine(path string, totals map[string]int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return accumulate(f, path, totals, func(line string) string {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			return line[:i]
		}
		return line
	})
}

// readGenbalance adds the computed airdrop rows. Each line is
// <source-addr>:<gno-addr>=<amount>ugnot, and only the part after the first
// colon is a balance — the same field `cut -d: -f2` used to take.
func readGenbalance(path string, totals map[string]int64) error {
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

	return accumulate(zr, path, totals, secondColonField)
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

// accumulate sums every row into totals. Addresses appearing in both input
// files are added together, matching the gawk pass this replaced — and
// deliberately unlike the consuming side, where LeftMerge is last-write-wins.
func accumulate(r io.Reader, name string, totals map[string]int64, prepare func(string) string) error {
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

		sum := totals[addr] + amount
		if sum < totals[addr] {
			return fmt.Errorf("%s:%d: %s overflows int64", name, n, addr)
		}
		totals[addr] = sum
	}

	return sc.Err()
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
func sortRows(totals map[string]int64) []row {
	rows := make([]row, 0, len(totals))
	for addr, amount := range totals {
		rows = append(rows, row{
			amount: amount,
			addr:   addr,
			line:   addr + "=" + strconv.FormatInt(amount, 10) + denom,
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
