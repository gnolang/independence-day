# independence-day — root orchestrator.
#
#   make            regenerate everything from the committed snapshots
#   make verify     run every check (tests + independent cross-check + supply total)
#   make tools      report which required tools are missing
#   make clean      remove regenerable intermediates (NOT the committed artifacts)
#
# The pipeline is bit-reproducible: on a clean checkout, `make` must leave
# `git status` clean apart from mtimes. If it does not, that is a bug.

GO      ?= go

GENBALANCE := allocate/genbalance.txt.gz
BALANCES   := mkgenesis/balances.txt.gz

.PHONY: all
all: $(BALANCES)

## ---------------------------------------------------------------- pipeline

# Stage 1 — allocate: snapshots + policy -> genbalance.txt.gz
.PHONY: allocate
allocate:
	cd allocate && $(GO) run .

$(GENBALANCE): allocate/process_consolidated.go allocate/atone.go \
               inputs/cosmoshub-10562840.json.gz inputs/atomone-6439117.json.gz \
               policy/excluded.txt policy/ibc-escrow-addresses.txt
	cd allocate && $(GO) run .

# Stage 2 — mkgenesis: genbalance + premine -> balances.txt.gz
$(BALANCES): $(GENBALANCE) mkgenesis/non-airdrop.txt
	$(MAKE) -C mkgenesis

## ---------------------------------------------------------------- verify

.PHONY: verify
verify: test crosscheck supply

.PHONY: test
test:
	cd allocate && $(GO) test ./...
	cd mkgenesis && $(GO) test ./...

.PHONY: crosscheck
crosscheck:
	cd verify/crosscheck && $(GO) run .

# Print the totals a human should eyeball. Deliberately not an assertion:
# the expected figure is a policy decision, not a fact about the code.
.PHONY: supply
supply:
	@$(GO) run ./mkgenesis supply -genbalance $(GENBALANCE) -balances $(BALANCES)

## ---------------------------------------------------------------- misc

# The pipeline needs Go and gzip, nothing else. (archive/manfred-recheck/ is a
# separate, archived derivation and still wants gawk, jq and sqlite; it is not
# reachable from any target here.)
.PHONY: tools
tools:
	@missing=0; \
	for t in $(GO) gzip; do \
		command -v $$t >/dev/null 2>&1 || { echo "MISSING: $$t"; missing=1; }; \
	done; \
	if [ $$missing -eq 1 ]; then \
		echo; \
		echo "macOS: gzip ships with the system; install Go from https://go.dev/dl/"; \
		echo "Debian/Ubuntu: apt install golang gzip"; \
		exit 1; \
	fi; \
	echo "all required tools present"

.PHONY: clean
clean:
	$(MAKE) -C mkgenesis clean
