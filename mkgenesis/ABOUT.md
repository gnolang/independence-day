# mkgenesis/ — the final merge

> `README.md` in this directory is **generated** by `go run . readme`. Do not hand-edit it. This file is
> the hand-written explanation; `README.md` is the machine-written report.

This is the last stage of the pipeline. It merges the computed airdrop with the two hand-written sheets
and produces the file that a chain's genesis builder actually downloads.

```
../allocate/genbalance.txt.gz   3,262,351 rows   (computed)
+ non-airdrop.txt                      75 rows / 66 addresses   (hand-written, mostly 2022)
+ publicsale.txt                       79 rows / 79 addresses   (hand-written, the Sonar sale)
= balances.txt.gz               3,262,464 rows
```

The rows do not add up to the output because **this stage sums duplicates**, and three different kinds
of duplicate occur:

| Overlap | Addresses | What it is |
|---|---:|---|
| within `non-airdrop.txt` | 9 | a multisig signer with both a gas float and a contributor-airdrop row |
| `non-airdrop.txt` ∩ airdrop | 6 | 2022 contributors and founders who also hold a snapshot entitlement |
| `publicsale.txt` ∩ airdrop | 26 | sale participants who also hold a snapshot entitlement |
| `non-airdrop.txt` ∩ `publicsale.txt` | 0 | |

3,262,351 + (66 − 6) + (79 − 26) = **3,262,464**.

The 26 are expected rather than surprising: the sale audience overlaps the Cosmos Hub / AtomOne one,
and a gno address is the same 20-byte key as the cosmos address it derives from. They are pinned by
`TestPublicSaleOverlapIsSummed` because nothing in the output shows that a row is a sum.

Note that **this stage sums duplicates**, whereas the consuming side (`gnogenesis balances add` →
`LeftMerge`) is last-write-wins and would silently drop one of the two. Any address that ends up both in
this sheet and in a genesis transaction on the consuming side is a live hazard.

## The public sale sheet

`publicsale.txt` is the only sheet whose rows may carry their own `;vesting=…` suffix, and the only one
whose rows are marked liquid at genesis. Both facts are consumed by `vesting.go`; read the header of
the sheet itself before editing it.

## The non-airdrop premine

`non-airdrop.txt` contributes **489,000 GNOT**:

| Group | GNOT |
|---|---|
| 3 named contributors | 300,000 |
| 45 GitHub requesters | 45,000 |
| 14 multisig signer gas floats | 14,000 |
| 13 `examples/` package authors | 130,000 |

The `test1` and `test2` rows (110,000 GNOT) were removed on 2026-09-03: both were funded from mnemonics
published in `gnolang/gno`'s own test fixtures, so anyone could spend them.

The premine is charged to the Ecosystem Treasury (`PREMINE_CHARGED_TO_ECOSYSTEM`, which tracks
`PREMINE_ABSORBED_FROM_CONTRIBS`), and its total is asserted against this file by
`TestPremineMatchesFile`.

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

## Vesting (Constitution §132-138)

**Off by default.** With `VESTING_START` unset the output is byte-identical to a build from before the
vesting pass existed — `go test ./...` asserts exactly that.

```sh
make VESTING_START=<unix> VESTING_END=<unix>
```

Every row then gains the suffix `gnogenesis balances add` parses:

```
g1…=632000000000000ugnot;vesting=606720000000000ugnot,1780000000,1843072000
```

§132 asks for *"4% unlocked on the day $GNOT becomes transferrable … and a 4% unlock every subsequent
month for 24 months"*. A continuous linear schedule over 24 months expresses that **exactly**, because
`4 + 96m/24 = 4(m+1)` — at month *m* the holder has `4(m+1)%` either way. So the vesting portion is 96%
of the balance (`VESTING_UNLOCK_PCT` defaults to 4), `start` is the transferability date and `end` is
24 months later.

| variable | flag | default | meaning |
|---|---|---|---|
| `VESTING_START` | `-vesting-start` | *(empty — off)* | unix seconds; the day GNOT becomes transferrable |
| `VESTING_END` | `-vesting-end` | *(empty)* | unix seconds; `START` + 24 months |
| `VESTING_UNLOCK_PCT` | `-vesting-unlock-pct` | `4` | percent unlocked at `START` |
| `VESTING_EXEMPT` | `-vesting-exempt` | *(empty)* | addresses that get no schedule |

`VESTING_EXEMPT` exists for §136-138 — *"150,000,000 $GNOT from the Investors allocation will be
unlocked at the mainnet launch"* — which is only expressible once that tranche has an address of its
own. That address is now `INVESTORS_UNLOCKED_ADDRESS` in `allocate/process_consolidated.go`; put it in
`VESTING_EXEMPT` and §136 is expressed end to end with no further code.

Two properties worth knowing:

- The schedule is applied **after** the sort, so it cannot influence row order. A vested sheet and a
  bare one list the same addresses in the same sequence.
- The suffix is **not** added to the balance — it is drawn from it. `README.md`'s total and
  `make supply` both read the balance and ignore the schedule, so a vested sheet still totals
  1,332,999,998.378908 GNOT.

The arithmetic is `int64`. The awk version this replaced accumulated in `float64` and had to refuse to
run once `amount * pct` reached 2^53; that limit is gone.

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
