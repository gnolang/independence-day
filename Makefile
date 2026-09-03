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
AWK     ?= gawk
ZCAT    ?= zcat
SHASUM  ?= shasum -a 256

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

.PHONY: crosscheck
crosscheck:
	cd verify/crosscheck && $(GO) run .

# Print the totals a human should eyeball. Deliberately not an assertion:
# the expected figure is a policy decision, not a fact about the code.
.PHONY: supply
supply:
	@echo "== allocate/genbalance.txt.gz =="
	@$(ZCAT) $(GENBALANCE) | wc -l | $(AWK) '{print $$1 " rows"}'
	@$(ZCAT) $(GENBALANCE) | cut -d: -f2 | cut -d= -f2 | sed 's/ugnot//' \
		| $(AWK) '{s+=$$1} END {printf "%d ugnot (%.6f GNOT)\n", s, s/1000000}'
	@echo "== mkgenesis/balances.txt.gz =="
	@$(ZCAT) $(BALANCES) | wc -l | $(AWK) '{print $$1 " rows"}'
	@$(ZCAT) $(BALANCES) | cut -d= -f2 | sed 's/ugnot//' \
		| $(AWK) '{s+=$$1} END {printf "%d ugnot (%.6f GNOT)\n", s, s/1000000}'
	@echo "== sha256 =="
	@$(SHASUM) $(GENBALANCE) $(BALANCES)

## ---------------------------------------------------------------- misc

.PHONY: tools
tools:
	@missing=0; \
	for t in $(GO) $(AWK) $(ZCAT) jq; do \
		command -v $$t >/dev/null 2>&1 || { echo "MISSING: $$t"; missing=1; }; \
	done; \
	if [ $$missing -eq 1 ]; then \
		echo; \
		echo "macOS: brew install gawk coreutils jq   (coreutils provides GNU zcat as gzcat;"; \
		echo "       or run: make ZCAT='gzip -dc')"; \
		echo "Debian/Ubuntu: apt install gawk gzip jq"; \
		exit 1; \
	fi; \
	echo "all required tools present"

.PHONY: clean
clean:
	$(MAKE) -C mkgenesis clean
