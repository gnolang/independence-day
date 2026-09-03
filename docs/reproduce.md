# Reproducing the genesis balances

Three levels, from cheapest to most paranoid.

---

## Level 1 — regenerate the outputs from the committed snapshots (~2 minutes)

```sh
make tools      # tells you what is missing
make            # allocate -> genbalance.txt.gz -> mkgenesis -> balances.txt.gz
make verify     # unit tests + independent cross-check + supply totals
git status      # should be clean
```

Requirements: Go 1.21+, GNU `awk` (`gawk`), GNU `zcat`, `jq`.

- **macOS:** `brew install gawk coreutils jq`. Apple's `zcat` looks for `.Z` files and will fail; either
  use `gzcat` from coreutils, or run `make ZCAT='gzip -dc'`. BSD `awk` also works
  (`make AWK=awk`) and produces byte-identical output, but `gawk` is what the original run used.
- **Debian/Ubuntu:** `apt install gawk gzip jq`.

### What "reproducible" means here

| Artifact | Bit-reproducible? |
|---|---|
| `allocate/genbalance.txt.gz` | **Yes**, including the gzip container — Go's `gzip.Writer` emits `mtime=0, OS=255`. |
| `mkgenesis/balances.txt` (uncompressed) | **Yes.** |
| `mkgenesis/balances.txt.gz` | **Yes**, because the Makefile passes `gzip -n`. Without `-n`, gzip stores the mtime and filename and every rebuild differs. |

Expected checksums for the current `main` are printed by `make supply`. If yours differ, that is worth
an issue.

---

## Level 2 — rebuild the consolidated snapshots

The two files in `inputs/` are themselves derived. To rebuild them:

**Cosmos Hub** — see [`../inputs/how-to-rebuild-cosmoshub.md`](../inputs/how-to-rebuild-cosmoshub.md) to
sync gaiad and export state at block 10562840, then
[`../inputs/README-snapshot-provenance.md`](../inputs/README-snapshot-provenance.md) for the
[`piux2/gnobounty7`](https://github.com/piux2/gnobounty7) merge:

```sh
jq '.app_state.bank.balances'      cosmos_10562840_export.json > balances.json
jq '.app_state.staking.delegations' cosmos_10562840_export.json > delegations.json
build/exactor merge --b balances.json --d delegations.json \
    --val validators.json --vote last_vote_pro69.json > snapshot_consolidated.json
```

`validators.json` and `last_vote_pro69.json` are committed in `inputs/` for exactly this purpose.

**AtomOne** — [`atomone-hub/govbox`](https://github.com/atomone-hub/govbox) against an AtomOne genesis
export at block [6439117](https://atomscan.com/atomone/blocks/6439117); commands are in
[`../inputs/README-snapshot-provenance.md`](../inputs/README-snapshot-provenance.md).

---

## Level 3 — re-derive without using this repository's code at all

The point of an airdrop record is that you do not have to trust the code that produced it.
[`../archive/manfred-recheck/`](../archive/manfred-recheck/) is exactly this: an independent derivation
of the Cosmos Hub figures using jq and sqlite instead of Go, with `shasums.txt` covering every
intermediate. [`../archive/prop69-votes/`](../archive/prop69-votes/) is an independent extraction of the
prop-69 votes straight from a gaiad node, so the vote weightings can be checked without trusting the
merged snapshot either.

Between them, every input to the allocation has a second source.

---

## Common gotchas

| Symptom | Cause |
|---|---|
| `zcat: can't stat: …gz (….gz.Z)` | macOS `zcat`. Use `make ZCAT='gzip -dc'`. |
| `gawk: command not found` | `brew install gawk`, or `make AWK=awk`. |
| `go run .` fails downloading modules | The root module pins a 2022 `gnolang/gno` revision on purpose. Do not `go get -u`. |
| `balances.txt.gz` differs but `balances.txt` matches | You are on a build without `gzip -n`. Compare the uncompressed files. |
| `crosscheck` reports a diff | One of the two artifacts was regenerated and the other was not. Run `make` from the repository root. |
