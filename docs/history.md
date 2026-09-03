# History of the allocation

Only commits that changed a number are listed. Everything else is docs and tooling.

| Date | Commit | PR | What changed |
|---|---|---|---|
| 2022-07-09 | `d79ce98` | #9 | `non-airdrop.txt` created — the 52-row premine. **Never edited since.** |
| 2022-07-15 | `c1bbada` | #11 | account merging |
| 2022-11-06 | `8dfb5ea` | #18 | IBC escrow skip added |
| 2025-09-03 | `8126bcf` | #23 | airdrop script and distribution update |
| 2026-02-01 | `f6fa69c` | #25 | `excluded.txt` introduced; exclusions matched as literal bech32 strings |
| 2026-02-24 | `c650ee2` | #32 | `excluded.txt` update |
| 2026-03-11 | `0229d78` | #28 | **AtomOne added.** A second qualification path, with `atone1…` addresses |
| 2026-03-12 10:48 | `6fa5437` | #36 | NT and GovDAO voter allocations added |
| 2026-03-12 15:21 | `9dec38a` | #38 | Non-20-byte (ICA) addresses rejected. 3,262,507 rows / 1,002,461,998.383260 GNOT |
| 2026-03-12 17:45 | `4b12044` | #40 | ICF added to `excluded.txt`. 3,262,505 rows / 1,002,461,998.378908 GNOT |
| 2026-03-12 18:31 | `d883acb` | #41 | `special-accounts.csv` update (annotation only — no effect on balances) |
| 2026-09-02 | `3672d8f` | #45 | Constants reconciled to the finalized 1.333B: Contributions 119,000,000 → 119,993,000, `TOTAL_AIRDROP_NT_LLC` = 332,000,000 added |

## Which artifact launched which chain

`gnolang/gno` pins the balance file **by commit sha**, so a deployment does not track `main`.

| Chain | Pinned commit | Rows | Total |
|---|---|---:|---:|
| `gnoland1` | `9dec38a` | 3,262,507 | 1,002,461,998.383260 GNOT |
| `test13` | `9dec38a` | 3,262,507 | 1,002,461,998.383260 GNOT |
| `test2`, `test3` (2022) | *unpinned*, `raw/main/…` | — | whatever `main` held at the time |

The `9dec38a` pin **predates** the ICF exclusion in `4b12044`. Because the distribution is proportional,
excluding an address does not only zero that address — it rescales every remaining holder in the same
bucket. Removing 15,399,456.604978 GNOT of ICF weight from the 350,000,000 ATOM bucket multiplies every
remaining ATOM-side entitlement by

```
350,000,000 / (350,000,000 − 15,399,456.604978) = 1.0460234058
```

so the diff between the two blobs is roughly 650,000 rows, not two. AtomOne-only holders are unaffected.

## Why the totals never look round

`whole()` truncates rather than rounds, so the file sums to ~1.62 GNOT below the buckets. Deterministic,
reproducible, and not drift.
