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
allocate/genbalance.txt.gz        3,262,351 rows
        │
        ├── mkgenesis/non-airdrop.txt      75 rows,   489,000 GNOT
        ├── mkgenesis/publicsale.txt       79 rows, 21,604,687.430103 GNOT
        ├── mkgenesis/investors.txt         9 rows, 16,076,470 GNOT
        ▼  cd mkgenesis && make            (concatenate, sum duplicates, sort desc)
mkgenesis/balances.txt.gz         3,262,473 rows
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
| Core / Ecosystem / Validator treasuries (§120-122) | `TOTAL_TREASURY_CORE`, `TOTAL_TREASURY_ECOSYSTEM`, `TOTAL_TREASURY_VALIDATOR` |
| Who pays for the founders and the premine | `FOUNDERS_CHARGED_TO_CORE`, `PREMINE_CHARGED_TO_ECOSYSTEM` |
| What actually reaches each treasury address | `TOTAL_TREASURY_*_NET` |
| Investors, split by §136 | `TOTAL_INVESTORS_UNLOCKED`, `TOTAL_INVESTORS_VESTING`, `TOTAL_AIRDROP_NT` |
| NT LLC | `TOTAL_AIRDROP_NT_LLC` |
| Founders (total, split evenly) | `TOTAL_AIRDROP_GOVDAO_FOUNDERS` |
| The three treasury addresses | `TREASURY_CORE_ADDRESS`, `TREASURY_ECOSYSTEM_ADDRESS`, `TREASURY_VALIDATOR_ADDRESS` |
| The three ex-nt1 addresses | `INVESTORS_UNLOCKED_ADDRESS`, `INVESTORS_VESTING_ADDRESS`, `NT_LLC_ADDRESS` |
| nt2 address | `MULTISIG_NT2_ADDRESS` |
| Who the 7 founders are | `govdaoFounders` |
| Which AiB addresses get swept into nt2 | `aibCosmosAddrs`, `aibAtoneAddrs` |
| Prop-69 vote weighting | `weight()` |
| Proportional split | `distribute()` |
| Exclusion matching | `skip()`, `loadExcludedAddresses()`, `loadEscrowAddress()` |
| 20-byte address requirement | `convertAddress()` |
| Truncation | `whole()` |
| PHOTON→ATONE ratio | `PHOTON_TO_ATONE_RATIO`, `allocate/atone.go` |
| Input/output file paths | the `const` block at the top of `process_consolidated.go` |
| Investors, less the public sale | `TOTAL_PUBLIC_SALE_UGNOT`, `TOTAL_INVESTORS_UNLOCKED_UGNOT` |
| The premine | `mkgenesis/non-airdrop.txt` |
| The public sale rows | `mkgenesis/publicsale.txt` |
| Duplicate summing | `accumulate()` in `mkgenesis/build.go` |
| What the §132 schedule is computed over | `entry.unlocked`, `vesting.apply()` in `mkgenesis/` |

There is deliberately no config file. Every number that affects an allocation is a Go constant in one
file, so `git log -p allocate/process_consolidated.go` is a complete history of the economics.

---

## The non-airdrop premine

`mkgenesis/non-airdrop.txt` is the older of the two hand-written balance sources. Most of it dates from
**July 2022**; it adds **489,000 GNOT** on top of whatever the buckets sum to:

| Group | Rows | GNOT |
|---|---:|---:|
| named contributors | 3 | 300,000 |
| GitHub requesters | 45 | 45,000 |
| multisig signer gas floats | 14 | 14,000 |
| `examples/` package authors | 13 | 130,000 |
| **Total** | **75** | **489,000** |

Two things that matter:

- The `test1` and `test2` rows (110,000 GNOT) were removed on 2026-09-03 — both were funded from
  mnemonics published in `gnolang/gno`'s own test fixtures. `faucet0` and `faucet1` (2,000,000 GNOT)
  were removed on 2026-09-09: there is no mainnet faucet, and a faucet is not an Ecosystem Treasury
  purpose under §322-328.
- The premine is paid for out of the Ecosystem Treasury, so the shipped file is the buckets plus
  489,000 minus the truncation residual.

---

## The public token sale

`mkgenesis/publicsale.txt` is the second hand-written balance source, and unlike the premine it is
**supply-neutral by construction**: `TOTAL_PUBLIC_SALE_UGNOT` is subtracted from the §136 unlocked
tranche before it is written, so every ugnot in the sheet is one that
`INVESTORS_UNLOCKED_ADDRESS` does not receive. Adding or removing a row moves GNOT between the two,
never into or out of existence — `TestPublicSaleMatchesFile` is what keeps the constant and the file
from drifting apart.

| Group | Rows | GNOT |
|---|---:|---:|
| participants who named a gno.land address | 78 | 16,521,530.801184 |
| `[sale-unclaimed]` 2-of-4 multisig, for the 44 who had not | 1 | 5,083,156.628919 |
| **Total** | **79** | **21,604,687.430103** |

Three things that matter:

- **26 of the 78 also hold an airdrop entitlement** at the same address, 675,669.367465 GNOT between
  them, and `accumulate()` sums the two. That is the intended treatment — the airdrop is for holding
  ATOM/ATONE at the 2022/2024 snapshots, the sale entitlement is for paying USD in 2026 — but it is
  invisible in the output, so `TestPublicSaleOverlapIsSummed` pins the set.
- **Sale allocations do not vest**, and those 26 rows are why that cannot be expressed as an exempt
  list: exempting the address would let its airdrop ride free too. `entry.unlocked` records how much
  of a row is liquid at genesis and the §132 schedule is computed over the remainder, which is right
  on both halves.
- **One row declares its own schedule** (a US accredited investor under a 12-month cliff) and one
  address carries at most one schedule — so for that row the declared cliff replaces §132 rather than
  stacking with it, and its airdrop portion ends up unscheduled. It is 1,995.215082 GNOT and the
  alternative is to express nothing at all. See
  [`../inputs/README-publicsale-provenance.md`](../inputs/README-publicsale-provenance.md).

---

## The investor and partner distributions

`mkgenesis/investors.txt` is the third hand-written source and the second carve-out of the same §136
tranche, so the sale paragraph above applies to it verbatim: `TOTAL_INVESTOR_DISTRIBUTIONS_UGNOT` is
subtracted from the tranche before it is written, and `TestInvestorDistributionsMatchFile` keeps the
constant and the file in step. **9 rows, 16,076,470 GNOT.** None of the nine overlaps any other sheet
or the airdrop, so all nine are new rows in the output.

The three §136 lines close exactly, which is the property that makes both carve-outs a re-shaping
rather than a reallocation — `TestTheThreeSection136LinesSumToTheTranche`:

```
112,318,842.569897   INVESTORS_UNLOCKED_ADDRESS
 21,604,687.430103   public sale     (79 rows)
 16,076,470.000000   distributions   ( 9 rows)
--------------------
150,000,000.000000   = §136 exactly
```

The counterparties are **not named** — in the sheet, in this file, or anywhere in the repository. The
rows carry opaque labels and are ordered by address so that neither the label nor the ordering
carries information, and `TestInvestorDistributionsMatchFile` asserts that row *shape* rather than a
blocklist of names, since a blocklist would publish the very list it protects. The amounts are public
whatever we do; what is withheld is which counterparty each row belongs to.

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
   genesis-transaction fee payer, one of the two values is silently discarded. This stage sums
   duplicates; the consumer does not.
3. **Nothing on the consuming side asserts total supply.** `gnogenesis verify` validates bank *params*,
   not the balance list. If this file is wrong, nothing downstream will say so.
