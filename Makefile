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
ALLOC_README := allocate/README.md
BALANCES   := mkgenesis/balances.txt.gz

.PHONY: all
all: $(BALANCES) $(ALLOC_README)

## ---------------------------------------------------------------- pipeline

# Stage 1 — allocate: snapshots + policy -> genbalance.txt.gz
.PHONY: allocate
allocate:
	cd allocate && $(GO) run .

$(GENBALANCE): allocate/process_consolidated.go allocate/atone.go \
               inputs/cosmoshub-10562840.json.gz inputs/atomone-6439117.json.gz \
               policy/excluded.txt policy/ibc-escrow-addresses.txt
	cd allocate && $(GO) run .

# allocate/README.md is a report on the artifact above, not part of producing it
# — mkgenesis/README.md has the same relationship to balances.txt. Both are
# pinned by golden tests, so a stale one is a red build.
$(ALLOC_README): $(GENBALANCE) allocate/readme.go
	cd allocate && $(GO) run . readme

# Stage 2 — mkgenesis: genbalance + premine + public sale -> balances.txt.gz
$(BALANCES): $(GENBALANCE) mkgenesis/non-airdrop.txt mkgenesis/publicsale.txt
	$(MAKE) -C mkgenesis

## ---------------------------------------------------------------- verify

.PHONY: verify
verify: test crosscheck checksums supply

# Assert the committed artifacts are byte-for-byte what SHA256SUMS records.
#
# This is the check that makes `balances.txt.gz` a publishable contract rather
# than a file that happens to be in git. `supply` below prints the same hashes,
# but printing is not verifying: nothing failed when they changed.
#
# It is also what catches a rebuild with the wrong gzip. The golden test
# decompresses before comparing, so it sees the plaintext and never the
# container — and the container is what consumers fetch.
.PHONY: checksums
checksums:
	@shasum -a 256 -c SHA256SUMS

# Rewrite SHA256SUMS from the artifacts currently on disk. Run this only when
# the artifacts are MEANT to change, and quote the new hashes in the PR.
.PHONY: checksums-update
checksums-update:
	@shasum -a 256 $(GENBALANCE) $(BALANCES) > SHA256SUMS
	@cat SHA256SUMS

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

# The pipeline needs Go and GNU gzip, nothing else. (archive/manfred-recheck/ is
# a separate, archived derivation and still wants gawk, jq and sqlite; it is not
# reachable from any target here.)
#
# It has to be GNU gzip specifically. macOS ships "Apple gzip", which honours
# -n and produces correct output — but a DIFFERENT deflate stream, ~243 KB
# smaller for this input. balances.txt.gz is fetched by raw URL as a public
# contract, so the container bytes are part of the artifact, and nothing else in
# the repository can see the difference: the golden test decompresses first, and
# CI runs on ubuntu, where gzip is always GNU.
.PHONY: tools
tools:
	@missing=0; \
	for t in $(GO) gzip shasum; do \
		command -v $$t >/dev/null 2>&1 || { echo "MISSING: $$t"; missing=1; }; \
	done; \
	if [ $$missing -eq 0 ] && ! gzip --version 2>&1 | head -1 | grep -q 'GNU\|^gzip [0-9]'; then \
		echo "WRONG GZIP: $$(gzip --version 2>&1 | head -1)"; \
		echo "  balances.txt.gz only reproduces with GNU gzip; this one writes a different container."; \
		echo "  macOS: brew install gzip, then put it first on PATH:"; \
		echo '    export PATH="$$(brew --prefix)/opt/gzip/bin:$$PATH"'; \
		echo "  nix: nix shell nixpkgs#gzip"; \
		missing=1; \
	fi; \
	if [ $$missing -eq 1 ]; then \
		echo; \
		echo "Install Go from https://go.dev/dl/"; \
		echo "Debian/Ubuntu: apt install golang gzip  (gzip is GNU there already)"; \
		exit 1; \
	fi; \
	echo "all required tools present ($$(gzip --version 2>&1 | head -1))"

.PHONY: clean
clean:
	$(MAKE) -C mkgenesis clean
