# mkgenesis — merge the computed airdrop with the hand-written premine.
#
# The pipeline itself lives in Go (see main.go); this file only wires the
# targets together. `gzip -n` stays here on purpose: Go's compress/gzip does
# not reproduce the committed container byte-for-byte, and balances.txt.gz is
# fetched by raw URL from outside this repository. See ABOUT.md.

GO ?= go

all: balances.txt.gz README.md

re: clean all

balances.txt: non-airdrop.txt ../allocate/genbalance.txt.gz
	$(GO) run . build

balances.txt.gz: balances.txt
	rm -f $@
	gzip -n -k balances.txt

README.md: balances.txt
	$(GO) run . readme

clean:
	rm -f balances.txt

.PHONY: all re clean
