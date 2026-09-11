# inputs/ — the raw material

**Immutable.** Nothing in here is edited after capture. If a snapshot is wrong it is replaced wholesale
with a new one at a new height, and the change is called out in [`../docs/history.md`](../docs/history.md).

That applies to the public-sale settlement too. It is a snapshot of a list that is still growing —
participants keep binding gno.land addresses after it was taken — so if genesis slips, replace the
file at a new instant rather than adding rows to this one. `publicsale-sonar-2026-09-11.csv` is the
second such extract; the 09-09 one is kept because it has already been published and reviewed, and
the two differ only in which wallets had bound. Each is identified by the `address_bindings` id it
stops at, not by a wall-clock instant.

| File | What it is | Consumed by |
|---|---|---|
| `cosmoshub-10562840.json.gz` | Cosmos Hub consolidated snapshot at block **10562840** (2022-05-20 08:00 PDT), 27 MB. Per address: liquid `uatom`, delegated `duatom` (shares already converted back to ATOM using each validator's token/share ratio, so slashing is accounted for), and that address's last vote on prop 69. | `allocate/process_consolidated.go` |
| `atomone-6439117.json.gz` | AtomOne consolidated snapshot at block **6439117**. Per address: `uatone`, `duatone`, `uphoton`. | `allocate/atone.go` |
| `cosmoshub-validators.json` | Cosmos Hub validator token/share ratios at snapshot height. | **not read by this repo** — see below |
| `cosmoshub-prop69-last-votes.json.gz` | Every vote submitted while prop 69 was active, from a quicksync.io `cosmos-hub-4` archive node. | **not read by this repo** — see below |
| `publicsale-sonar-2026-09-11.csv` | **Current.** The Sonar public token sale, settled on Ethereum mainnet. All **122** wallets, marked `GENESIS` (had named a gno.land address by `address_bindings` id **79**, **78** of them) or `UNCLAIMED` (had not, **44**). **Reference only — this is not the file that is loaded**; the 78 + 1 rows that are live in `mkgenesis/publicsale.txt`. | humans |
| `publicsale-sonar-2026-09-09.csv` | Superseded. The same 122 wallets at the earlier extract, `address_bindings` id **68**, when 67 had bound. Kept as the published record, not read by anything. | humans |
| `how-to-rebuild-cosmoshub.md` | How to sync a gaiad full node and export state at 10562840. | humans |
| `README-snapshot-provenance.md` | The exact `gnobounty7` / `govbox` command lines that produced the two consolidated snapshots, plus notes on how delegations and zero-balance accounts were handled. | humans |
| `README-publicsale-provenance.md` | How each sale amount was derived, which wallets are in genesis and why the rest are not, the one 12-month-lockup row and its evidence, and what is and is not verified. | humans |

## Why two of these are committed but unread

`cosmoshub-validators.json` and `cosmoshub-prop69-last-votes.json.gz` are **upstream** of this
repository's Go code: they were fed to [`piux2/gnobounty7`](https://github.com/piux2/gnobounty7) to build
`cosmoshub-10562840.json.gz`. They are committed so that the *consolidation* step can be re-derived
rather than trusted. Do not delete them because "nothing imports them" — that is precisely the point.

## Re-deriving the snapshots independently

- **Cosmos Hub** — `how-to-rebuild-cosmoshub.md`, then `README-snapshot-provenance.md`.
- **AtomOne** — `README-snapshot-provenance.md`, using
  [`atomone-hub/govbox`](https://github.com/atomone-hub/govbox) against an AtomOne genesis export at
  block [6439117](https://atomscan.com/atomone/blocks/6439117).

A second, independently-tooled derivation of the Cosmos Hub numbers (jq + sqlite, with checksums) is
preserved in [`../archive/manfred-recheck/`](../archive/manfred-recheck/).
