# inputs/ — the raw material

**Immutable.** Nothing in here is edited after capture. If a snapshot is wrong it is replaced wholesale
with a new one at a new height, and the change is called out in [`../docs/history.md`](../docs/history.md).

| File | What it is | Consumed by |
|---|---|---|
| `cosmoshub-10562840.json.gz` | Cosmos Hub consolidated snapshot at block **10562840** (2022-05-20 08:00 PDT), 27 MB. Per address: liquid `uatom`, delegated `duatom` (shares already converted back to ATOM using each validator's token/share ratio, so slashing is accounted for), and that address's last vote on prop 69. | `allocate/process_consolidated.go` |
| `atomone-6439117.json.gz` | AtomOne consolidated snapshot at block **6439117**. Per address: `uatone`, `duatone`, `uphoton`. | `allocate/atone.go` |
| `cosmoshub-validators.json` | Cosmos Hub validator token/share ratios at snapshot height. | **not read by this repo** — see below |
| `cosmoshub-prop69-last-votes.json.gz` | Every vote submitted while prop 69 was active, from a quicksync.io `cosmos-hub-4` archive node. | **not read by this repo** — see below |
| `how-to-rebuild-cosmoshub.md` | How to sync a gaiad full node and export state at 10562840. | humans |
| `README-snapshot-provenance.md` | The exact `gnobounty7` / `govbox` command lines that produced the two consolidated snapshots, plus notes on how delegations and zero-balance accounts were handled. | humans |

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
