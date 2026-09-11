# inputs/ — the raw material

**Immutable.** Nothing in here is edited after capture. If a snapshot is wrong it is replaced wholesale
with a new one at a new height, and the change is called out in [`../docs/history.md`](../docs/history.md).

That applies to the public-sale settlement too. It is a snapshot of a list that is still growing —
participants keep binding gno.land addresses after it was taken — so if genesis slips, replace the
file at a new instant rather than adding rows to this one. **Replaced means replaced:** there is one
extract in this directory at a time, identified by the `address_bindings` id it stops at rather than
by a wall-clock instant. `publicsale-sonar-2026-09-11.csv` (id 79) is the second, and it superseded
`publicsale-sonar-2026-09-09.csv` (id 68), which was deleted rather than kept alongside it.

Nothing is lost by that. The two extracts hold the same 122 wallets with identical amounts — only
which of them had bound differs, and every `GENESIS` row of the earlier file is present verbatim in
the later one. The earlier file is also still in git, at
[`9ecf4d3`](https://github.com/gnolang/independence-day/commit/9ecf4d3) (#65) where it was captured:

```sh
git show 9ecf4d3:inputs/publicsale-sonar-2026-09-09.csv
```

Two extracts in the tree at once is the worse option, not the safer one: the one that is loaded is
not named in either file, so a reader has to work out which is live, and a stale one sitting next to
a current one is exactly the shape of the drift the rest of this directory's rules exist to prevent.

| File | What it is | Consumed by |
|---|---|---|
| `cosmoshub-10562840.json.gz` | Cosmos Hub consolidated snapshot at block **10562840** (2022-05-20 08:00 PDT), 27 MB. Per address: liquid `uatom`, delegated `duatom` (shares already converted back to ATOM using each validator's token/share ratio, so slashing is accounted for), and that address's last vote on prop 69. | `allocate/process_consolidated.go` |
| `atomone-6439117.json.gz` | AtomOne consolidated snapshot at block **6439117**. Per address: `uatone`, `duatone`, `uphoton`. | `allocate/atone.go` |
| `cosmoshub-validators.json` | Cosmos Hub validator token/share ratios at snapshot height. | **not read by this repo** — see below |
| `cosmoshub-prop69-last-votes.json.gz` | Every vote submitted while prop 69 was active, from a quicksync.io `cosmos-hub-4` archive node. | **not read by this repo** — see below |
| `publicsale-sonar-2026-09-11.csv` | The Sonar public token sale, settled on Ethereum mainnet, through `address_bindings` id **79**. All **122** wallets, marked `GENESIS` (had named a gno.land address by then, **78** of them) or `UNCLAIMED` (had not, **44**). **Reference only — this is not the file that is loaded**; the 78 + 1 rows that are live in `mkgenesis/publicsale.txt`. Supersedes the id-68 extract, which is in git at `9ecf4d3`. | humans |
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
