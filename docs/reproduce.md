# Reproducing the genesis balances

Three levels, from cheapest to most paranoid.

---

## Level 1 — regenerate the outputs from the committed snapshots (~2 minutes)

```sh
make tools      # tells you what is missing — run this FIRST
make            # allocate -> genbalance.txt.gz -> mkgenesis -> balances.txt.gz
make verify     # unit tests + cross-check + checksums + supply totals
git status      # should be clean
```

Requirements: Go 1.21+ and **GNU** `gzip`.

⚠️ **It has to be GNU gzip.** macOS ships "Apple gzip", which is a different implementation: it
honours `-n` and produces perfectly valid output, but a **different deflate stream** — about 243 KB
smaller for this input, with a different sha256. Since `balances.txt.gz` is published by raw URL, the
container bytes are part of the artifact, so a rebuild with Apple gzip leaves `git status` dirty and
changes the hash consumers check. `make tools` now refuses to proceed rather than letting you find
out from a confusing diff.

```sh
brew install gzip                                    # macOS
export PATH="$(brew --prefix)/opt/gzip/bin:$PATH"
nix shell nixpkgs#gzip                               # or, with nix
```

On Debian/Ubuntu and in CI, `gzip` is already GNU and there is nothing to do.

(`archive/manfred-recheck/` is a separate, archived derivation that still wants `gawk`, `jq` and
sqlite. It is not reachable from any target in the root `Makefile`.)

### What "reproducible" means here

| Artifact | Bit-reproducible? |
|---|---|
| `allocate/genbalance.txt.gz` | **Yes**, including the gzip container — Go's `gzip.Writer` emits `mtime=0, OS=255`. |
| `mkgenesis/balances.txt` (uncompressed) | **Yes**, and `cd mkgenesis && go test ./...` asserts it against the committed artifacts. |
| `mkgenesis/balances.txt.gz` | **Yes — with GNU gzip.** Two conditions, both required: the Makefile passes `gzip -n` (without it gzip stores the mtime and filename, so every rebuild differs), *and* the gzip is GNU (Apple's writes a different deflate stream). |

Expected checksums are committed in [`../SHA256SUMS`](../SHA256SUMS) and asserted by `make checksums`,
which `make verify` and CI both run. `make supply` prints the same hashes for eyeballing. If
`make checksums` fails on a clean checkout you have not rebuilt anything — that is worth an issue.

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
| `go run .` fails downloading modules | The root module pins a 2022 `gnolang/gno` revision on purpose. Do not `go get -u`. |
| `balances.txt.gz` differs but `balances.txt` matches | Your `gzip` is not GNU — almost always macOS's "Apple gzip". Run `make tools`; it names the problem. (The other cause is a build without `gzip -n`, but this Makefile always passes it.) |
| `make checksums` fails right after `make` | Same cause as the row above, nine times out of ten. Check `gzip --version` before concluding the content changed. |
| `crosscheck` reports a diff | One of the two artifacts was regenerated and the other was not. Run `make` from the repository root. |
