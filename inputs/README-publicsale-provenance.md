# The public token sale — where the numbers come from

The GNOT public sale ran on **Sonar**, Ethereum mainnet, contract
[`0x959f2ceE7B6C2095d228692eCb2E4744f2D3fDb4`](https://etherscan.io/address/0x959f2ceE7B6C2095d228692eCb2E4744f2D3fDb4).
It settled **122 wallets** for **21,604,687.430103 GNOT**.

This file explains how each wallet's amount was derived, which of the 122 are in genesis and which
are not, and what the one flagged row is. The settlement itself is
[`publicsale-sonar-2026-09-11.csv`](./publicsale-sonar-2026-09-11.csv); the rows actually loaded are
[`../mkgenesis/publicsale.txt`](../mkgenesis/publicsale.txt).

## Which of the 122 are in genesis

Only the participants who **named a gno.land address**. A participant does that by signing a message
with the Ethereum wallet that paid, at <https://sale.gno.land/distribution>; the site checks the
address decodes and records the binding.

The extract behind this file ends at `address_bindings` id **79**, whose newest binding is
**2026-09-11T04:45:17.459Z**; it was taken 2026-09-11T06:32Z. 78 of the 122 had bound by then. The
CSV marks them `GENESIS`; the other 44 are `UNCLAIMED`.

**Quote the id, not a wall-clock instant.** The previous file was described here as the
"2026-09-09T06:46Z" snapshot, which was a local (UTC+9) file timestamp written as if it were UTC.
The true instant was 2026-09-08T21:47Z, and the extract ends at id 68 whose newest binding is
2026-09-08T07:40:32.565Z. No binding predates either extract and is missing from it, so both files
are sound; only the label was wrong.

|  | wallets | ugnot |
|---|---:|---:|
| In genesis | 78 | 16,521,530,801,184 |
| of which base | | 13,994,946,775,197 |
| of which bonus | | 2,526,584,025,987 |
| **Not in genesis** | **44** | **5,083,156,628,919** |
| Total settled | **122** | **21,604,687,430,103** |

The 44 settled and are owed their allocation; they simply had not named an address in time. Their
5,083,156,628,919 ugnot goes to the `[sale-unclaimed]` 2-of-4 multisig
(`g1rphzpk58kn0nqpgu8k8apaq2ftzgpsgql8wjr0`, `gnolang/multisigs`) and is distributed by hand as they
come forward.

**The binding page has no deadline, so the list keeps growing.** Anyone binding after the snapshot
falls in the multisig share. If genesis slips, re-extract and rebuild rather than patching rows in.

### On the counts

The extract has **79 rows** for **78 wallets** (one wallet submitted twice, 28 seconds apart, to the
same address) and at most **77 people**, because one Sonar entity owns two of the 78 wallets. That
last count is not checkable from this repository: entity ids live in the settlement bundle's
bonus-distribution export, which carries 77 distinct `sale_specific_entity_id` across the 78
wallets, one of them twice. Say "wallets", not "people".

## How the amounts were derived

Clearing price **$0.0645** (129/2000), the sale floor: 47.35% subscribed, so every commitment was
accepted in full and no refunds were issued.

- `base` = accepted USD / 0.0645
- `tiered bonus` = 15% of the portion of the running sale total below $1,000,000, 10% above. The
  published schedule also had 5% and 3% bands from $1.5M and $2M; the sale raised $1.18M and never
  reached them. Rebuilt from the on-chain order of the 173 `CommitmentIncreased` events, per delta.
- `first-24h bonus` = 5% of base for a first bid before 2026-07-21 12:00 UTC, from the `BidPlaced`
  log. The settlement CSV timestamp is the *last* bid and would have excluded 8 participants.

`ugnot_total` **is** the total. Do not add the bonus columns to it — that would mint
2,526,584,025,987 ugnot that nobody is owed.

## The one flagged row

`g18c0grhdx96lw2u5t9qchl390n5weu9znkwf5vm` (1,841,860,465 ugnot) carries `lockup_12m = yes`: a US
accredited investor under a 12-month transfer restriction after TGE.

It is not a judgement call. In tx
[`0x605c1c33b4a60f1851aa34fff1e8f277443c343fbdfca415f0e18facacaa1cf8`](https://etherscan.io/tx/0x605c1c33b4a60f1851aa34fff1e8f277443c343fbdfca415f0e18facacaa1cf8)
the bid is `{lockup: true, price: 3, amount: 108000000}` and the Sonar-signed permit's `payload` is
`0x…01`, which the contract decodes as `forcedLockup` and which makes the flag **mandatory** — a bid
without it reverts. The permit signature recovers to `0x9A1964E04d10ccFe1A9cE300d943F71c6E3aE24c`, the
sole holder of `PURCHASE_PERMIT_SIGNER_ROLE`. So the restriction was imposed at purchase time by the
sale platform; our own frontend never offers a lockup toggle.

The 12-month duration comes from the sale FAQ's statement of a 12-month transfer restriction after TGE
for US accredited investors. The legal disclaimer itself speaks of an indefinite period and names no
term — so if the duration ever matters legally, take it from the disclaimer and not from this file.

`mkgenesis/publicsale.txt` carries the restriction as a cliff, `1820534400` being
2027-09-10T00:00:00Z:

    g18c0grhdx96lw2u5t9qchl390n5weu9znkwf5vm=1841860465ugnot;vesting=1841860465ugnot,0,1820534400;type=delayed

`type=delayed` vests everything at the end and nothing before, and is enforced by the bank keeper at
runtime — a transfer while locked is refused, verified by running gno's own
`GetBalancesFromSheet` on this sheet and `go test ./tm2/pkg/sdk/bank/ -run Vesting`.

Two things to know before relying on it:

1. **It is always emitted, including when the §132 pass is off.** This changed on 2026-09-09: the
   schedule used to be dropped from every committed build, because `vesting.apply()` tested for
   "vesting off" before it tested for "this row declared its own schedule". The committed
   `balances.txt.gz` therefore showed this address fully liquid at genesis, with only a build-log
   warning to say so. A declared schedule is not part of the opt-in §132 mechanism — it exists
   precisely because §132 cannot express it — so it is now honoured unconditionally.

   **Consequence for `unrestricted_addrs`:** caveat 2 below is live rather than theoretical.
2. **The genesis binary must contain gno commit `331e17fc3`** (gnolang/gno#6095, 2026-08-28), which is
   on `master` only. `chain/pearl` and `chain/sapphire` carry the grammar but an older design where
   the account is not a `*GnoAccount`; there, a vesting address that also appears in
   `unrestricted_addrs` makes `InitChain` **panic**.

   **This is not one row.** `unrestricted.txt` currently holds **28** addresses that also carry a
   vesting line in the built `balances.txt`: the 26 sale participants who ALSO hold an airdrop
   entitlement and so keep a §132 schedule over the airdrop half, plus two of the three §127 funds.
   It was 27 before this snapshot. The nine §136 distribution rows add none — they are unlocked and
   none of them holds an airdrop entitlement — but that is a fact about today's sheet, not a
   property, so re-run the count rather than trusting this number:

       awk -F'#' '{print $1}' mkgenesis/unrestricted.txt | awk 'NF{print $1}' > /tmp/u
       gzip -dc mkgenesis/balances.txt.gz |
           awk -F= 'NR==FNR{u[$1];next} $1 in u && /;vesting=/ {print $1}' /tmp/u -

   So dropping the one declared-schedule row is **not** a way to make an older binary safe: it
   removes 1 of 28 triggers. Either build from a revision containing `331e17fc3`, or do not use
   `unrestricted_addrs` at all on that binary. (For completeness, dropping that row alone would put
   the genesis total at 16,519,688,940,719 and the multisig at 5,084,998,489,384, and it is still
   the right move if the goal is to avoid shipping a lockup the binary cannot honour, as opposed to
   avoiding the panic.)

## What has been verified

The committed CSV carries no Ethereum address and no signature, by the privacy decision at the foot
of this file, so the first three items below are author-attested and cannot be re-run from this
repository alone. The aggregate band totals, the per-row arithmetic and the bech32 properties can.

- Amounts reconcile to the ugnot against the bonus-distribution export and the binding recipients,
  all 122.
- Tiered bands and 24h eligibility replayed from the Ethereum event log reproduce the settlement file,
  all 122.
- All 79 binding signatures recover to the paying address, so every destination was authorised by its
  payer. Mutation-tested: one altered character anywhere breaks recovery.
- All 78 destinations are valid bech32, distinct, none equal to an Ethereum address, minimum pairwise
  difference 31 of 40 characters.
- Value is conserved from chain to file: no ugnot created or lost.
- Re-read on-chain 2026-09-09 at block 25,935,858: all 244 (wallet, token) accepted amounts unchanged
  since the 2026-08-03 export, `stage() = 4` (Done), `totalAcceptedAmount() = 1,183,664,402,471`. The
  contract is an immutable minimal proxy, so allocations cannot have been rewritten. Deliberately not
  repeated for this extract: the sale is closed and the proxy immutable, so no amount can move. What
  changes between extracts is which wallets have bound, which is database state, not chain state.

## What has NOT been verified

**Nothing proves a participant controls the address they named.** The binding flow only checks that
the address decodes. A valid address pasted in error loses those tokens permanently. This is a known
property of the design, not a defect in this derivation.

`signer` records how the destination was obtained. 73 came from Adena, which reads the address out of
the extension, so those are key-backed by construction. 5 were pasted via gnokey and carry only a
checksum: 26,544.186046 GNOT, 0.16% of what is paid directly at genesis (0.12% of the whole
settled sale). Nothing can prove key control for those.

## For anyone rebuilding this

- Use integer or `Decimal` arithmetic, **never float**. A float rebuild disagrees by 1 ugnot on 4 rows
  and breaks `ugnot_total == base + bonus` on those.
- The tiered bonus **cannot** be re-derived from the CSVs alone. It depends on the arrival order of
  the 173 `CommitmentIncreased` events, and 9 participants are split mid-payment at the $1,000,000
  mark. Keep the raw event log from the settlement bundle.
- Participants signed a message containing `Chain: gnoland1`. Neither genesis tool defaults to it —
  `gnogenesis generate` writes `dev` and `gnogenesis fork generate` writes `gnoland-1` with a hyphen,
  and nothing validates it. Pass `-chain-id gnoland1` explicitly and grep the final `genesis.json`
  for `"chain_id": "gnoland1"`.

## A column that is deliberately not here

The settlement carries an `eth_address` per row: the wallet that paid, and the wallet whose signature
authorised the destination. It is **withheld from the committed CSV**, because publishing it would
publish a mapping between each participant's Ethereum identity and their gno.land identity — a link
that the on-chain data does not by itself provide. That is a privacy decision, not a technical one,
and it has not been made.

The cost is real: without it the *authorisation* step above cannot be re-checked from this repository
alone, only the arithmetic. Restoring the column is a one-line change if the decision goes the other
way.
