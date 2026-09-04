package main

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// runSupply prints the totals a human should eyeball. Deliberately not an
// assertion: the expected figure is a policy decision, not a fact about the
// code.
func runSupply(args []string) error {
	fs := flag.NewFlagSet("supply", flag.ExitOnError)
	genbalance := fs.String("genbalance", "../allocate/genbalance.txt.gz", "gzipped airdrop rows")
	balances := fs.String("balances", "balances.txt.gz", "gzipped merged balances")
	if err := fs.Parse(args); err != nil {
		return err
	}

	for _, f := range []struct {
		path    string
		prepare func(string) string
	}{
		{*genbalance, secondColonField},
		{*balances, func(line string) string { return line }},
	} {
		rows, total, err := totalGzipped(f.path, f.prepare)
		if err != nil {
			return err
		}
		fmt.Printf("== %s ==\n", f.path)
		fmt.Printf("%d rows\n", rows)
		fmt.Printf("%d %s (%s GNOT)\n", total, denom, gnot(total))
	}

	fmt.Println("== sha256 ==")
	for _, path := range []string{*genbalance, *balances} {
		sum, err := sha256File(path)
		if err != nil {
			return err
		}
		fmt.Printf("%s  %s\n", sum, path)
	}

	return nil
}

// gnot renders ugnot as GNOT with the six decimals the awk version printed,
// without going through a float.
func gnot(ugnot int64) string {
	return fmt.Sprintf("%d.%06d", ugnot/1000000, ugnot%1000000)
}

func totalGzipped(path string, prepare func(string) string) (int, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w", path, err)
	}
	defer zr.Close()

	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var rows int
	var total int64
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(prepare(sc.Text()))
		if line == "" {
			continue
		}

		_, amount, err := parseRow(line)
		if err != nil {
			return 0, 0, fmt.Errorf("%s:%d: %w", path, n, err)
		}

		rows++
		sum := total + amount
		if sum < total {
			return 0, 0, fmt.Errorf("%s:%d: total overflows int64", path, n)
		}
		total = sum
	}

	return rows, total, sc.Err()
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
