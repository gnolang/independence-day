# allocate/ — the allocation engine

> `README.md` in this directory is **generated** by `go run . readme`. Do not hand-edit it. This file is
> the hand-written explanation; `README.md` is the machine-written report.

One Go program. It reads the snapshots from `../inputs/` and the decisions from `../policy/`, and writes
`genbalance.txt.gz`.

```sh
cd allocate
go run .            # ~90 s, writes genbalance.txt.gz
go run . readme     # regenerates README.md from that artifact
go test ./...
```

## Files

| File | What |
|---|---|
| `process_consolidated.go` | Constants, prop-69 weighting, exclusions, bucket allocations, output writer. **The single source of truth for every allocation number.** |
| `atone.go` | AtomOne qualification: `uatone + duatone + uphoton/7`. |
| `readme.go` | Reads `genbalance.txt.gz` back and renders `README.md`. States no figure it did not measure. |
| `process_test.go` | Address conversion, weighting, distribution, and a total-supply check against `genbalance.txt.gz`. |
| `golden_test.go` | Requires the committed `README.md` to match what `readme.go` regenerates. |
| `genbalance.txt.gz` | **Generated.** Sorted by source address. Row count and totals: [`README.md`](./README.md). |

## Output format

```
<source address>:<gno address>=<amount>ugnot
```

The first column is the **source** the entitlement came from — `cosmos1…` for the Cosmos Hub snapshot,
`atone1…` for AtomOne, and `g1…` for the synthetic rows, which are the ones assigned outright rather
than derived from a snapshot: the purpose-bound buckets (the three treasuries, the two Investors
tranches, NT,LLC, nt2), the GovDAO founders, the founding validator set's gas float (§122 — the
four `INITIAL_VALSET` signing addresses and their four operators, mirrored from `gnolang/gno`'s
`misc/deployments/mainnet.gno.land`) and the §120 chain-service floats. The second column is always
the gno address: the same 20 bytes re-encoded with the `g` HRP. Keeping the source column is what
makes the file auditable — you can see *why* a row exists.

`README.md` lists the current row count, the split by source, and every synthetic row in full. Those
figures are deliberately not repeated here: this file is hand-written and would go stale, which is
exactly what happened to the numbers that used to live in it.

## Things worth knowing before changing anything

- **Truncation, not rounding.** `whole()` drops the decimal part of each account's `Dec`. Across 3.26M
  rows that loses ~1.62 GNOT in total, which is why the sum never lands on a round number.
- **Zero rows are suppressed** at write time: an address that qualifies but rounds to 0 ugnot does not
  appear in the output at all. That is why there are no zero-balance rows downstream.
- **Non-20-byte addresses are skipped**, with a log line. That is how 32-byte interchain-account (ICA)
  addresses are kept out — they have no gno equivalent.
- **nt2 is a sweep, not a bucket.** `processNTMultisig` *deletes* the AiB addresses from the distribution
  and re-adds their combined total under nt2, so that GNOT comes out of the 350M + 231M rather than on
  top of it. Six of the seven `aibCosmosAddrs` are not present in the Cosmos snapshot and are logged as
  `not found in distribution`; that is expected, not an error.
- **Changing a constant means regenerating BOTH `genbalance.txt.gz` and `../mkgenesis/`,** and then
  `README.md` on top. Run `make` from the repository root rather than `go run .` on its own; it wires
  the three together in order. CI does catch a skipped regeneration — `verify/crosscheck` and the two
  golden tests all run on every pull request — but it catches it as a red build, not as a merge you can
  fix later.
