// Command mkgenesis is the last stage of the pipeline: it merges the computed
// airdrop with the hand-written premine and produces balances.txt, the file a
// chain's genesis builder downloads.
//
// It replaces a shell pipeline (cat | cut | grep | zcat | sed | gawk | sort)
// that used to live in Makefile. The Makefile still drives it and still owns
// the `gzip -n` step: Go's compress/gzip does not reproduce the committed
// container byte-for-byte, and that artifact is a public contract. See ABOUT.md.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "build":
		err = runBuild(os.Args[2:])
	case "readme":
		err = runReadme(os.Args[2:])
	case "supply":
		err = runSupply(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "mkgenesis: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "mkgenesis: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: go run . <command> [flags]

commands:
  build     merge genbalance.txt.gz + non-airdrop.txt into balances.txt
  readme    regenerate README.md from balances.txt
  supply    report rows, totals and sha256 for the committed artifacts

Each command takes -h for its flags.
`)
}
