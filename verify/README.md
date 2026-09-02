# verify/ — independent checks

Nothing here generates anything. These exist to catch the failure mode where a constant changes and one
of the two generated artifacts is not regenerated.

## `crosscheck/`

An independent Go re-implementation of the `mkgenesis` merge, using the *modern* `gnolang/gno`
balance-parsing types rather than this repository's 2022-era ones. It asserts:

```
mkgenesis/balances.txt.gz  ==  allocate/genbalance.txt.gz  +  mkgenesis/non-airdrop.txt
```

address by address and coin by coin, and independently re-derives each `g1…` address from its `cosmos1…`
counterpart to confirm the two columns of `genbalance.txt.gz` agree with each other.

```sh
cd verify/crosscheck && go run .
# expected final line: "Balance files match."
```

It has its own `go.mod` because it deliberately pins a recent `gnolang/gno`, while the root module stays
pinned to a 2022 revision for bit-compatibility with the original run. **Do not merge the two modules** —
using two independent implementations of bech32 and coin parsing is the whole value of this check.

Originally contributed as `aeddi-recheck/` (PR #22, @aeddi).

## What is still not covered

- Nothing asserts an **exact** total supply for `mkgenesis/balances.txt.gz`. `allocate/process_test.go`
  checks `genbalance.txt.gz` to within 0.01%; the file that actually ships is unchecked.
- Nothing asserts that no address in the sheet collides with a genesis-transaction fee payer on the
  consuming side. `gnogenesis`' balance merge is last-write-wins, so a collision silently destroys one of
  the two values rather than erroring.
- Nothing checks the sheet against a dust floor, an address-format regex, or duplicate detection.
  (All three currently pass, but nothing keeps them passing.)
