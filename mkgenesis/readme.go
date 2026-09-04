package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// topAccounts is how many rows the generated report lists.
const topAccounts = 100

func runReadme(args []string) error {
	fs := flag.NewFlagSet("readme", flag.ExitOnError)
	balances := fs.String("balances", "balances.txt", "merged balances to report on")
	out := fs.String("out", "README.md", "generated report")
	if err := fs.Parse(args); err != nil {
		return err
	}

	report, err := readReport(*balances)
	if err != nil {
		return err
	}

	if err := os.WriteFile(*out, []byte(report.render(filepath.Base(*balances))), 0o644); err != nil {
		return err
	}

	fmt.Printf("%s: %d rows, %d %s\n", *out, report.rows, report.total, denom)
	return nil
}

// report is everything the generated README states about a balances file.
type report struct {
	rows  int
	total int64
	top   []string
	dupes []string // lines whose address appears more than once
}

func readReport(path string) (*report, error) {
	counts := make(map[string]int)
	r := &report{}

	if err := scanBalances(path, func(line, addr string, amount int64) error {
		r.rows++
		sum := r.total + amount
		if sum < r.total {
			return fmt.Errorf("total overflows int64 at %q", line)
		}
		r.total = sum

		if len(r.top) < topAccounts {
			r.top = append(r.top, line)
		}
		counts[addr]++
		return nil
	}); err != nil {
		return nil, err
	}

	// Empty by construction when build produced the file: it sums duplicates.
	// The section still exists so the report answers the question for a
	// hand-edited sheet, and a second pass is only paid when there is one.
	duplicated := make(map[string]bool)
	for addr, n := range counts {
		if n > 1 {
			duplicated[addr] = true
		}
	}
	if len(duplicated) > 0 {
		if err := scanBalances(path, func(line, addr string, _ int64) error {
			if duplicated[addr] {
				r.dupes = append(r.dupes, line)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func scanBalances(path string, visit func(line, addr string, amount int64) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		addr, amount, err := parseRow(line)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", path, n, err)
		}
		if err := visit(line, addr, amount); err != nil {
			return fmt.Errorf("%s:%d: %w", path, n, err)
		}
	}

	return sc.Err()
}

// render writes the report in the exact layout the Makefile's shell version
// produced, down to the trailing blank line. balancesName is echoed in the
// "lines" section, where `wc -l <file>` used to print it — formatted here so
// the output no longer differs between GNU and BSD wc.
func (r *report) render(balancesName string) string {
	var b strings.Builder

	b.WriteString("# Genesis\n\n")

	b.WriteString("## lines\n```\n")
	fmt.Fprintf(&b, "%d %s\n", r.rows, balancesName)
	b.WriteString("```\n\n")

	b.WriteString("## sum\n```\n")
	fmt.Fprintf(&b, "%d%s\n", r.total, denom)
	fmt.Fprintf(&b, "%dgnot\n", r.total/1000000)
	b.WriteString("```\n\n")

	b.WriteString("## duplicate accounts\n```\n")
	for _, line := range r.dupes {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("```\n\n")

	b.WriteString("## top 100 accounts\n```\n")
	for _, line := range r.top {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("```\n\n")

	return b.String()
}
