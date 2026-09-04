# mkgenesis/ — the final merge

> `README.md` in this directory is **generated** by `go run . readme`. Do not hand-edit it. This file is
> the hand-written explanation; `README.md` is the machine-written report.

This is the last stage of the pipeline. It merges the computed airdrop with the hand-written premine and
produces the file that a chain's genesis builder actually downloads.

```
../allocate/genbalance.txt.gz   3,262,457 rows   (computed)
+ non-airdrop.txt                      52 rows   (hand-written, 2022)
= balances.txt.gz               3,262,505 rows
```

52 + 3,262,457 − 3,262,505 = **4** addresses appear in both files and have their amounts **summed**:

| Address | premine | airdrop | note |
|---|---|---|---|
| `g1us8428u2a5satrlxzagqqa5m6vmuze025anjlj` | 100,000 (test2) | 1,000 | the founder row and the test2 row land on the same key |
| `g14da4n9hcynyzz83q607uu8keuh9hwlv42ra6fa` | 100,000 (@piux2) | airdrop | |
| `g13278z0a5ufeg80ffqxpda9dlp599t7ekregcy6` | 1,000 (@alstn3726) | airdrop | |
| `g1j80fpcsumfkxypvydvtwtz3j4sdwr8c2u0lr64` | 1,000 (@danny-pham) | airdrop | |

Note that **this stage sums duplicates**, whereas the consuming side (`gnogenesis balances add` →
`LeftMerge`) is last-write-wins and would silently drop one of the two. Any address that ends up both in
this sheet and in a genesis transaction on the consuming side is a live hazard.

## The non-airdrop premine

`non-airdrop.txt` contributes **2,345,000 GNOT**:

| Group | GNOT |
|---|---|
| faucet0 + faucet1 | 2,000,000 |
| 3 named contributors | 300,000 |
| 45 GitHub requesters | 45,000 |

The `test1` and `test2` rows (110,000 GNOT) were removed on 2026-09-03: both were funded from mnemonics
published in `gnolang/gno`'s own test fixtures, so anyone could spend them.

The premine is charged to the Contributions bucket (`PREMINE_ABSORBED_FROM_CONTRIBS`), and its total is
asserted against this file by `TestPremineMatchesFile`.

## Running it

```sh
make        # rebuilds balances.txt, balances.txt.gz and README.md
make re     # clean + rebuild
```

The merge itself is Go (`main.go`, `build.go`, `readme.go`); the `Makefile` only wires the targets
together. Requires Go and `gzip` — nothing else.

```sh
go run . build     # genbalance.txt.gz + non-airdrop.txt -> balances.txt
go run . readme    # balances.txt -> README.md
go run . supply    # totals and sha256 for both committed artifacts
```

## Reproducibility notes

- `balances.txt` **is** bit-reproducible, and `go test ./...` proves it: the golden test rebuilds it
  from the committed inputs and requires a byte-for-byte match against the committed `balances.txt.gz`
  and `README.md`.
- The row order is amount descending, ties broken by **whole-line descending byte order**. That
  reproduces the `sort -t = -k 2 -n -r` this stage used to shell out to: it had no `-s`, so equal
  amounts fell back to the last-resort whole-line comparison, which `-r` reversed too. The tie-break is
  load-bearing — do not change it. Comparing bytes in Go also removes the old dependence on `LC_ALL`.
- `balances.txt.gz` is bit-reproducible **only because** the Makefile passes `gzip -n`, which suppresses
  the stored mtime and filename. Without `-n` every rebuild produces a different `.gz` for identical
  content, which makes publishing a checksum meaningless.
- The `.gz` is still written by `gzip`, not by Go. Go's `compress/gzip` produces a different (smaller)
  container for identical content, and this path is a public contract — see below. `allocate/` does use
  Go's writer, which is why the two committed `.gz` files carry different OS bytes.

## ⚠️ This path is a public contract

`mkgenesis/balances.txt.gz` is fetched by raw URL from outside this repository — see
[the root README](../README.md#stable-paths--do-not-move-these). This directory keeps its historical name
for that reason alone. Moving the file breaks every unpinned consumer with a silent 404.
