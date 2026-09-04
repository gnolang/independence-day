# mkgenesis — merge the computed airdrop with the hand-written premine.
#
# The pipeline itself lives in Go (see main.go); this file only wires the
# targets together. `gzip -n` stays here on purpose: Go's compress/gzip does
# not reproduce the committed container byte-for-byte, and balances.txt.gz is
# fetched by raw URL from outside this repository. See ABOUT.md.

GO ?= go

# Vesting (Constitution §132-138). OFF by default: with VESTING_START unset the
# build behaves exactly as it did before the vesting pass existed, and
# balances.txt is byte-identical. See ABOUT.md.
#
#   make VESTING_START=1780000000 VESTING_END=1843072000
#
VESTING_START      ?=
VESTING_END        ?=
VESTING_UNLOCK_PCT ?= 4
VESTING_EXEMPT     ?=

VESTING_FLAGS =
ifneq ($(VESTING_START),)
VESTING_FLAGS = -vesting-start=$(VESTING_START) -vesting-end=$(VESTING_END) \
                -vesting-unlock-pct=$(VESTING_UNLOCK_PCT) \
                -vesting-exempt="$(VESTING_EXEMPT)"
endif

all: balances.txt.gz README.md

re: clean all

balances.txt: non-airdrop.txt ../allocate/genbalance.txt.gz
	$(GO) run . build $(VESTING_FLAGS)

balances.txt.gz: balances.txt
	rm -f $@
	gzip -n -k balances.txt

README.md: balances.txt
	$(GO) run . readme

clean:
	rm -f balances.txt

.PHONY: all re clean
