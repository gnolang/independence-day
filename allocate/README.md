# allocate/ — the allocation engine

One Go program. It reads the snapshots from `../inputs/` and the decisions from `../policy/`, and writes
`genbalance.txt.gz`.

```sh
cd allocate
go run .        # ~90 s, writes genbalance.txt.gz
go test ./...
```

## Files

| File | What |
|---|---|
| `process_consolidated.go` | Constants, prop-69 weighting, exclusions, bucket allocations, output writer. **The single source of truth for every allocation number.** |
| `atone.go` | AtomOne qualification: `uatone + duatone + uphoton/7`. |
| `process_test.go` | Address conversion, weighting, distribution, and a total-supply check against `genbalance.txt.gz`. |
| `genbalance.txt.gz` | **Generated.** 3,262,457 rows, sorted by source address. |

## Output format

```
cosmos10008uvk6fj3ja05u092ya5sx6fn355wavael4j:g10008uvk6fj3ja05u092ya5sx6fn355walp9u5k=3204884ugnot
atone14lultfckehtszvzw4ehu0apvsr77afvyegusc0:g14lultfckehtszvzw4ehu0apvsr77afvyy5u50n=18268683530357ugnot
g1pxj9x5jkklzam9v76q7sn7grm0xnuj69qu7lmf:g1pxj9x5jkklzam9v76q7sn7grm0xnuj69qu7lmf=632000000000000ugnot
```

The first column is the **source** the entitlement came from — `cosmos1…` for the Cosmos Hub snapshot,
`atone1…` for AtomOne, and `g1…` for the ten synthetic rows (nt1, nt2, GovDAO T1, and the 7 founders).
The second column is always the gno address: the same 20 bytes re-encoded with the `g` HRP. Keeping the
source column is what makes the file auditable — you can see *why* a row exists.

Of the 3,262,457 rows, **2,602,257 come from AtomOne** and 660,190 from Cosmos Hub.

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
- **Changing a constant means regenerating BOTH `genbalance.txt.gz` and `../mkgenesis/`.** They are
  separate steps, and CI does not currently catch the second one being skipped. Run `make` from the
  repository root rather than `go run .` on its own.
