package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"strings"

	"github.com/gnolang/gno/gno.land/pkg/gnoland"
	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	"github.com/gnolang/gno/tm2/pkg/bech32"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/std"
)

// Define the structure for balance files
type balanceFile struct {
	filename string
	balances balanceMap
}

type balanceMap map[crypto.Address]std.Coins

// Compare two balance files and print the differences if any.
func (b *balanceFile) compare(other *balanceFile) bool {
	var diff bool

	fmt.Printf("Comparing %s with %s\n", b.filename, other.filename)

	if len(b.balances) != len(other.balances) {
		diff = true
		fmt.Printf("Balance files differ in length: %d for %s vs %d for %s\n",
			len(b.balances),
			b.filename,
			len(other.balances),
			other.filename,
		)
	}

	for address, coins := range b.balances {
		otherCoins, exists := other.balances[address]
		if !exists {
			diff = true
			fmt.Printf("Address %s found in %s but not in %s\n", address, b.filename, other.filename)
			continue
		}

		if !coins.IsEqual(otherCoins) {
			diff = true
			fmt.Printf("Coins for address %s differ: %s for %s vs %s for %s\n",
				address,
				coins,
				b.filename,
				otherCoins,
				other.filename,
			)
		}
	}

	// NOTE: this used to read `other.balances[address]` on both sides, so the
	// condition was always false and addresses missing from b were never
	// reported. Fixed to look them up in b.
	for address := range other.balances {
		if _, exists := b.balances[address]; !exists {
			diff = true
			fmt.Printf("Address %s found in %s but not in %s\n", address, other.filename, b.filename)
		}
	}

	return diff
}

func (b *balanceFile) addBalances(other *balanceFile) {
	for address, coins := range other.balances {
		b.balances[address] = b.balances[address].Add(coins)
	}
}

// Define a function type for parsing a balance line.
type parserFunc func(string) (*gnoland.Balance, error)

// Parse a gziped balance file using the provided parser function.
func parseBalanceFile(filename string, parseLine parserFunc) (*balanceFile, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var scanner *bufio.Scanner

	// If the file is not gzipped, create a scanner directly.
	if !strings.HasSuffix(filename, ".gz") {
		scanner = bufio.NewScanner(file)
	} else { // Else, create a new gzip reader.
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()

		scanner = bufio.NewScanner(gzReader)
	}

	var (
		balances = make(balanceMap)
		lineNum  = 0
	)

	// Read the file line by line.
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Remove comments and trim spaces.
		line = strings.Split(line, "#")[0]
		line = strings.TrimSpace(line)

		// Skip empty lines.
		if line == "" {
			continue
		}

		// Parse the line into an account balance.
		balance, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("unable to parse line %d: %w", lineNum, err)
		}

		balances[balance.Address] = balance.Amount
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning file failed: %w", err)
	}

	return &balanceFile{
		filename: filename,
		balances: balances,
	}, nil
}

// sourcePrefixes are the human-readable parts that may appear in the source
// column of genbalance.txt.gz: "cosmos" for the Cosmos Hub snapshot, "atone"
// for AtomOne, and "g" for the ten synthetic rows (nt1, nt2, GovDAO T1 and the
// seven founders), whose source column is already the gno address.
var sourcePrefixes = map[string]bool{"cosmos": true, "atone": true, "g": true}

// sourceAddressToGnoAddress re-derives the gno address from a source address by
// re-encoding the same 20-byte payload with the "g" HRP.
func sourceAddressToGnoAddress(sourceAddr string) (string, error) {
	prefix, addr, err := bech32.Decode(sourceAddr)
	if err != nil {
		return "", fmt.Errorf("failed to decode source address: %w", err)
	}

	if !sourcePrefixes[prefix] {
		return "", fmt.Errorf("unexpected prefix %q in %s", prefix, sourceAddr)
	}

	if len(addr) != 20 {
		return "", fmt.Errorf("address %s has %d bytes, expected 20", sourceAddr, len(addr))
	}

	gnoAddress, err := bech32.Encode("g", addr)
	if err != nil {
		return "", fmt.Errorf("failed to encode gno address: %w", err)
	}

	return gnoAddress, nil
}

// validateUgnotAmount checks if the amount is in ugnot and positive.
func validateUgnotAmount(amount std.Coins) error {
	// Check if the amount is not empty.
	if len(amount) == 0 {
		return fmt.Errorf("amount is empty")
	}

	// Check if there is more than one amount.
	if len(amount) > 1 {
		return fmt.Errorf("more than one amount")
	}

	// Check if the amount is not in ugnot.
	if amount[0].Denom != ugnot.Denom {
		return fmt.Errorf("amount is not in ugnot")
	}

	// Check if the amount is negative.
	if amount[0].Amount < 0 {
		return fmt.Errorf("amount is negative")
	}

	return nil
}

func parseGnoBalance(line string) (*gnoland.Balance, error) {
	// Parse the line into an account balance.
	var balance gnoland.Balance
	if err := balance.Parse(line); err != nil {
		return nil, fmt.Errorf("unable to parse gno balance: %w", err)
	}

	// Validate the balance amount.
	if err := validateUgnotAmount(balance.Amount); err != nil {
		return nil, fmt.Errorf("invalid balance: %w", err)
	}

	return &balance, nil
}

// parseConsolidateLine parses one row of genbalance.txt.gz, which has the form
//
//	<source_addr>:<gno_addr>=<amount>ugnot
//
// and independently re-derives <gno_addr> from <source_addr> to confirm the two
// columns describe the same 20-byte key.
func parseConsolidateLine(line string) (*gnoland.Balance, error) {
	sourceAddr, rest, ok := strings.Cut(line, ":")
	if !ok {
		return nil, fmt.Errorf("malformed line, expected <source>:<gno>=<amount>: %q", line)
	}

	gnoBalance, err := parseGnoBalance(rest)
	if err != nil {
		return nil, fmt.Errorf("unable to parse gno balance: %w", err)
	}

	// Check that the source address and the gno address are the same key.
	converted, err := sourceAddressToGnoAddress(sourceAddr)
	if err != nil {
		return nil, fmt.Errorf("unable to convert source address: %w", err)
	}
	if converted != gnoBalance.Address.String() {
		return nil, fmt.Errorf("source address %s does not match gno address %s", sourceAddr, gnoBalance.Address.String())
	}

	return gnoBalance, nil
}

type fileParser struct {
	filename string
	parser   parserFunc
}

func main() {
	var (
		parsers = []fileParser{
			{"../../mkgenesis/balances.txt.gz", parseGnoBalance},
			{"../../mkgenesis/non-airdrop.txt", parseGnoBalance},
			{"../../allocate/genbalance.txt.gz", parseConsolidateLine},
		}
		balanceFiles = make([]*balanceFile, 0, len(parsers))
	)

	// Import all balance files using the parsers defined above.
	//
	// A parse failure is fatal. This used to `continue`, which then indexed
	// balanceFiles[2] unconditionally and panicked with an index-out-of-range
	// that buried the real error.
	for _, parser := range parsers {
		fmt.Printf("Importing balance file: %s\n", parser.filename)
		balanceFile, err := parseBalanceFile(parser.filename, parser.parser)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error importing balance file %s: %v\n", parser.filename, err)
			os.Exit(1)
		}

		balanceFiles = append(balanceFiles, balanceFile)
	}

	// Add non-airdrop balances to the consolidate balance file.
	fmt.Println("Adding non-airdrop to consolidate balance file")
	balanceFiles[2].addBalances(balanceFiles[1])

	// Compare mkgenesis balance file with the consolidate balance file.
	if balanceFiles[0].compare(balanceFiles[2]) {
		fmt.Fprintln(os.Stderr, "Balance files DIFFER.")
		os.Exit(1)
	}
	fmt.Println("Balance files match.")
}
