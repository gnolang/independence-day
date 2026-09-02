# Verifying your own line

You do not need to trust this repository. You can check your own number by hand.

Everything below uses one worked example: a Cosmos Hub address that voted NO on prop 69.

---

## Step 0 — find your gno address

Your gno address is your Cosmos Hub (or AtomOne) address re-encoded with the `g` bech32 prefix. Same 20
bytes, same key, different human-readable part.

```sh
cd allocate
cat > /tmp/conv.go <<'EOF'
// go run /tmp/conv.go cosmos1...
EOF
# or, simplest: grep the source column directly (see step 2)
```

## Step 1 — find yourself in the snapshot

```sh
gzip -dc inputs/cosmoshub-10562840.json.gz \
  | jq -r '.[] | select(.address=="cosmos1YOURADDRESS")'
```

You will get something like:

```json
{
  "address": "cosmos1...",
  "coins": [
    { "denom": "uatom",  "amount": "455794000000" },
    { "denom": "duatom", "amount": "5083895000000" }
  ],
  "vote": "{\"option\":3,\"weight\":\"1.000000000000000000\"}"
}
```

- `uatom` — liquid ATOM at block 10562840.
- `duatom` — your delegations, already converted from shares back to ATOM using each validator's
  token/share ratio at that height, so past slashing is reflected.
- `vote` — your **last** vote on prop 69. `1` = YES, `2` = ABSTAIN, `3` = NO, `4` = NO WITH VETO.
  Absent or empty means you did not vote.

If you are not in this file, check the AtomOne snapshot: `inputs/atomone-6439117.json.gz`.

## Step 2 — compute your weight

**Cosmos Hub**, from `allocate/process_consolidated.go` → `weight()`:

| `vote` option | Weight |
|---|---|
| `1` YES | `uatom` (delegations count for **zero**) |
| `3` NO | `uatom + duatom + (duatom >> 1)` — integer 1.5×, rounding down |
| `4` NO WITH VETO | `uatom + duatom × 2` |
| anything else | `uatom + duatom` |

For the example (NO): `455794000000 + 5083895000000 + 2541947500000 = 8081636500000`.

**AtomOne**, from `allocate/atone.go`: `uatone + duatone + uphoton / 7` (integer division, no vote
weighting).

## Step 3 — convert weight to ugnot

```
your_ugnot = trunc( your_weight / total_weight × BUCKET × 1_000_000 )
```

`BUCKET` is 350,000,000 for Cosmos Hub and 231,000,000 for AtomOne. Note **truncation**, not rounding —
the fractional part is discarded.

`total_weight` is the sum over every qualifying address, which you can compute yourself:

```sh
cd allocate && go run .    # prints progress; the totals are what distribute() uses
```

If you hold both ATOM and ATONE, you get **both** allocations and they are summed.

## Step 4 — check it against the shipped file

```sh
gzip -dc allocate/genbalance.txt.gz | grep ':g1YOURADDRESS='
gzip -dc mkgenesis/balances.txt.gz  | grep '^g1YOURADDRESS='
```

The `genbalance` line shows which snapshot your entitlement came from (`cosmos1…` or `atone1…` prefix).
The `balances` line is the final number, and will be **larger** than the `genbalance` one if you also
appear in `mkgenesis/non-airdrop.txt`, or if you appear in *both* snapshots.

---

## If your number is zero, or you are not there at all

Work down this list:

1. **You voted YES on prop 69 and held only delegated ATOM.** Weight is then `uatom` alone, which is
   zero. This is the single most common reason.
2. **Your allocation truncated to 0 ugnot.** Rows that round to zero are dropped entirely
   (`process_consolidated.go`, the `ugnot != "0"` guard). The smallest surviving row is 1 ugnot.
3. **Your address is 32 bytes**, i.e. an interchain account (ICA). Those are rejected — they have no gno
   equivalent. You would see it logged as `has 32 bytes, expected 20 bytes` during a run.
4. **Your address is excluded** — check `policy/excluded.txt`.
5. **Your address is an IBC escrow account** — check `policy/ibc-escrow-addresses.txt`. These are keyless
   module accounts; nobody holds their key.
6. **You held nothing at the snapshot height.** The snapshots are dated: Cosmos Hub block 10562840
   (2022-05-20) and AtomOne block 6439117. Anything acquired afterwards does not count.

If none of these explain it, open an issue with the address and what you expected. That is what this
repository is for.
