# The pipeline, end to end

```
inputs/how-to-rebuild-cosmoshub.md          how to produce the raw ~60 GB gaiad export
        │
        ▼  external: piux2/gnobounty7  "exactor merge --b --d --val --vote"
inputs/cosmoshub-10562840.json.gz   ─┐
inputs/atomone-6439117.json.gz      ─┤  external: atomone-hub/govbox
policy/excluded.txt                 ─┤
policy/ibc-escrow-addresses.txt     ─┤  (loaded; its skip is currently disabled)
        │                            │
        ▼  cd allocate && go run .   │
allocate/genbalance.txt.gz        3,262,457 rows
        │
        ├── mkgenesis/non-airdrop.txt      50 rows, 2,345,000 GNOT
        ▼  cd mkgenesis && make            (concatenate, sum duplicates, sort desc)
mkgenesis/balances.txt.gz         3,262,505 rows
        │
        ▼  fetched by raw URL
gnolang/gno  misc/deployments/gnoland1/gen-genesis.sh
gnolang/gno  misc/deployments/test13.gno.land/gen-genesis.sh
        │
        ▼  gnogenesis balances add
genesis.json
```

---

## Where every decision lives

All in `allocate/process_consolidated.go` unless noted.

| Decision | Symbol / location |
|---|---|
| ATOM bucket | `TOTAL_AIRDROP_ATOM` |
| AtomOne bucket | `TOTAL_AIRDROP_ATONE` |
| Contributions bucket | `TOTAL_AIRDROP_CONTRIBS` |
| Investors | `TOTAL_AIRDROP_NT` |
| NT LLC | `TOTAL_AIRDROP_NT_LLC` |
| Founders (total, split evenly) | `TOTAL_AIRDROP_GOVDAO_FOUNDERS` |
| nt1 / nt2 / GovDAO T1 addresses | `MULTISIG_NT1_ADDRESS`, `MULTISIG_NT2_ADDRESS`, `MULTISIG_GOVDAO_ADDRESS` |
| Who the 7 founders are | `govdaoFounders` |
| Which AiB addresses get swept into nt2 | `aibCosmosAddrs`, `aibAtoneAddrs` |
| Prop-69 vote weighting | `weight()` |
| Proportional split | `distribute()` |
| Exclusion matching | `skip()`, `loadExcludedAddresses()`, `loadEscrowAddress()` |
| 20-byte address requirement | `convertAddress()` |
| Truncation | `whole()` |
| PHOTON→ATONE ratio | `PHOTON_TO_ATONE_RATIO`, `allocate/atone.go` |
| Input/output file paths | the `const` block at the top of `process_consolidated.go` |
| The premine | `mkgenesis/non-airdrop.txt` |
| Duplicate summing | the `gawk` line in `mkgenesis/Makefile` |

There is deliberately no config file. Every number that affects an allocation is a Go constant in one
file, so `git log -p allocate/process_consolidated.go` is a complete history of the economics.

---

## The non-airdrop premine

`mkgenesis/non-airdrop.txt` is the only hand-written balance source. It was last edited in **July 2022**
and adds **2,345,000 GNOT** on top of whatever the buckets sum to:

| Group | Rows | GNOT |
|---|---:|---:|
| `faucet0`, `faucet1` | 2 | 2,000,000 |
| named contributors | 3 | 300,000 |
| GitHub requesters | 45 | 45,000 |
| **Total** | **50** | **2,345,000** |

Two things that matter:

- The `test1` and `test2` rows (110,000 GNOT) were removed on 2026-09-03 — both were funded from
  mnemonics published in `gnolang/gno`'s own test fixtures. `faucet0` and `faucet1` carry the same
  2022 "(temporary)" marking and have **not** been resolved.
- The premine is paid for out of the Contributions bucket, so the shipped file is the buckets plus
  2,345,000 minus the truncation residual.

---

## Truncation residual

`whole()` truncates each account's `Dec` to a whole ugnot rather than rounding. Across ~3.26M rows the
discarded fractions total roughly **1.62 GNOT**, so the sum of the buckets and the sum of the file never
match exactly. This is expected and is not drift — it is deterministic and reproducible.

---

## Interface to the consuming side

`gnolang/gno` downloads `mkgenesis/balances.txt.gz` by raw URL and feeds it to `gnogenesis balances add`.
Three properties of that consumer are worth knowing when editing this repository:

1. **The download is pinned by commit sha and has no checksum.** Changing `main` does not change what a
   pinned deployment fetches; conversely, a pinned deployment can be arbitrarily stale relative to `main`.
2. **The merge is last-write-wins, not additive.** If an address appears both in this sheet and as a
   genesis-transaction fee payer, one of the two values is silently discarded. This stage's `gawk` sums
   duplicates; the consumer does not.
3. **Nothing on the consuming side asserts total supply.** `gnogenesis verify` validates bank *params*,
   not the balance list. If this file is wrong, nothing downstream will say so.
