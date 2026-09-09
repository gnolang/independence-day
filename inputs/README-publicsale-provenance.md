# The public token sale — where the numbers come from

The GNOT public sale ran on **Sonar**, Ethereum mainnet, contract
[`0x959f2ceE7B6C2095d228692eCb2E4744f2D3fDb4`](https://etherscan.io/address/0x959f2ceE7B6C2095d228692eCb2E4744f2D3fDb4).
It settled **122 wallets** for **21,604,687.430103 GNOT**.

This file explains how each wallet's amount was derived, which of the 122 are in genesis and which
are not, and what the one flagged row is. The settlement itself is
[`publicsale-sonar-2026-09-09.csv`](./publicsale-sonar-2026-09-09.csv); the rows actually loaded are
[`../mkgenesis/publicsale.txt`](../mkgenesis/publicsale.txt).

## Which of the 122 are in genesis

Only the participants who **named a gno.land address**. A participant does that by signing a message
with the Ethereum wallet that paid, at <https://sale.gno.land/distribution>; the site checks the
address decodes and records the binding.

At the snapshot instant — **2026-09-09T06:46Z**, `address_bindings` max id **68** — 67 of the 122 had
bound. The CSV marks them `GENESIS`; the other 55 are `UNCLAIMED`.

|  | wallets | ugnot |
|---|---:|---:|
| In genesis | 67 | 12,266,287,839,945 |
| of which base | | 10,421,557,472,871 |
| of which bonus | | 1,844,730,367,074 |
| **Not in genesis** | **55** | **9,338,399,590,158** |
| Total settled | **122** | **21,604,687,430,103** |

The 55 settled and are owed their allocation; they simply had not named an address in time. Their
9,338,399,590,158 ugnot goes to the `[sale-unclaimed]` 2-of-4 multisig
(`g1rphzpk58kn0nqpgu8k8apaq2ftzgpsgql8wjr0`, `gnolang/multisigs`) and is distributed by hand as they
come forward.

**The binding page has no deadline, so the list keeps growing.** Anyone binding after the snapshot
falls in the multisig share. If genesis slips, re-extract and rebuild rather than patching rows in.

### On the counts

The extract has **68 rows** for **67 wallets** — one wallet submitted twice, 28 seconds apart, to the
same address — and at most **66 people**, because one Sonar entity owns two of the 67 wallets. Say
"wallets", not "people".

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
1,844,730,367,074 ugnot that nobody is owed.

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

1. **It is only emitted when vesting is on** (`make VESTING_START=… VESTING_END=…`). The committed
   `balances.txt.gz` is built with vesting off, so the schedule is absent from it and the restriction
   has to be honoured by hand. `mkgenesis build` prints a warning whenever it drops the schedule this
   way — do not ignore that line.
2. **The genesis binary must contain gno commit `331e17fc3`** (gnolang/gno#6095, 2026-08-28), which is
   on `master` only. `chain/pearl` and `chain/sapphire` carry the grammar but an older design where
   the account is not a `*GnoAccount`; there, a vesting address that also appears in
   `unrestricted_addrs` makes `InitChain` **panic**. If the build does not contain it, do not ship the
   vesting line at all — the fallback is to drop that row from the sheet and distribute it by hand,
   which puts the genesis total at 12,264,445,979,480 and the multisig at 9,340,241,450,623.

## What has been verified

- Amounts reconcile to the ugnot against the bonus-distribution export and the binding recipients,
  all 122.
- Tiered bands and 24h eligibility replayed from the Ethereum event log reproduce the settlement file,
  all 122.
- All 68 binding signatures recover to the paying address, so every destination was authorised by its
  payer. Mutation-tested: one altered character anywhere breaks recovery.
- All 67 destinations are valid bech32, distinct, none equal to an Ethereum address, minimum pairwise
  difference 31 of 40 characters.
- Value is conserved from chain to file: no ugnot created or lost.
- Re-read on-chain 2026-09-09 at block 25,935,858: all 244 (wallet, token) accepted amounts unchanged
  since the 2026-08-03 export, `stage() = 4` (Done), `totalAcceptedAmount() = 1,183,664,402,471`. The
  contract is an immutable minimal proxy, so allocations cannot have been rewritten.

## What has NOT been verified

**Nothing proves a participant controls the address they named.** The binding flow only checks that
the address decodes. A valid address pasted in error loses those tokens permanently. This is a known
property of the design, not a defect in this derivation.

`signer` records how the destination was obtained. 63 came from Adena, which reads the address out of
the extension, so those are key-backed by construction. 4 were pasted via gnokey and carry only a
checksum — 24,480 GNOT, 0.20% of the sheet. Nothing can prove key control for those.

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
