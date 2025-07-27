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

	for address := range other.balances {
		if _, exists := other.balances[address]; !exists {
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

// Convert a Cosmos address to a Gno address.
func cosmosAddressToGnoAddress(cosmosAddr string) (string, error) {
	prefix, addr, err := bech32.Decode(cosmosAddr)
	if err != nil {
		return "", fmt.Errorf("failed to decode cosmos address: %w", err)
	}

	if prefix != "cosmos" {
		return "", fmt.Errorf("unexpected prefix: %s, expected 'cosmos'", prefix)
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

func parseConsolidateLine(line string) (*gnoland.Balance, error) {
	// Split the line into cosmos address and gno balance.
	parts := strings.Split(line, ":")
	cosmosAddr := parts[0]

	gnoBalance, err := parseGnoBalance(parts[1])
	if err != nil {
		return nil, fmt.Errorf("unable to parse gno balance: %w", err)
	}

	// Check if the cosmos and associated gno addresses match.
	converted, err := cosmosAddressToGnoAddress(cosmosAddr)
	if err != nil {
		return nil, fmt.Errorf("unable to convert cosmos address: %w", err)
	}
	if converted != gnoBalance.Address.String() {
		return nil, fmt.Errorf("cosmos address %s does not match gno address %s", cosmosAddr, gnoBalance.Address.String())
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
			{"../mkgenesis/balances.txt.gz", parseGnoBalance},
			{"../mkgenesis/non-airdrop.txt", parseGnoBalance},
			{"../consolidate/genbalance.txt.gz", parseConsolidateLine},
		}
		balanceFiles = make([]*balanceFile, 0, len(parsers))
	)

	// Import all balance files using the parsers defined above.
	for _, parser := range parsers {
		fmt.Printf("Importing balance file: %s\n", parser.filename)
		balanceFile, err := parseBalanceFile(parser.filename, parser.parser)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error importing balance file %s: %v\n", parser.filename, err)
			continue
		}

		balanceFiles = append(balanceFiles, balanceFile)
	}

	// Add non-airdrop balances to the consolidate balance file.
	fmt.Println("Adding non-airdrop to consolidate balance file")
	balanceFiles[2].addBalances(balanceFiles[1])

	// Compare mkgenesis balance file with the consolidate balance file.
	if !balanceFiles[0].compare(balanceFiles[2]) {
		fmt.Println("Balance files match.")
	}
}
