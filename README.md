# independence-day

This repository contains the scripts and data used to compute the initial GNOT token allocation for the Gno.land genesis block. The goal is to distribute GNOT in a fair and transparent way, proportional to ATOM, ATONE and PHOTON holdings and weighted by participation in Cosmos Hub governance.

All source data, intermediate outputs, and processing scripts are included so anyone can independently verify the resulting balances.

## Allocation overview

The total supply is approximately **1,002,461,998 GNOT**, split across the following buckets:

| Bucket | Amount (GNOT) | Description |
|---|---|---|
| ATOM airdrop | 350,000,000 | Cosmos Hub ATOM holders (snapshot block 10562840, 2022-05-20) |
| AtomOne airdrop | 231,000,000 | AtomOne ATONE holders (snapshot block 6439117) |
| Contributions | 119,000,000 | GovDAO multisig, for community contributions |
| NT allocation | 300,000,000 | Newtendermint LLC + investors multisig |
| GovDAO founders | 7,000 | 1,000 GNOT per founder (7 founders) |

## Voting weight rules (Cosmos Hub prop 69)

Each ATOM-holding address is assigned a weight based on its liquid ATOM (`uatom`) balance and its delegated ATOM (`duatom`) balance at the snapshot, modified by its vote on Cosmos Hub [Proposal 69](https://www.mintscan.io/cosmos/proposals/69):

| Vote | Weight formula |
|---|---|
| YES | `uatom` only (staked ATOM excluded) |
| NO | `uatom + duatom × 1.5` |
| NO WITH VETO | `uatom + duatom × 2` |
| ABSTAIN / did not vote | `uatom + duatom` |

Each address's GNOT is then allocated proportionally to its weight relative to the total weight of all qualifying addresses.

## Data sources

### Cosmos Hub snapshot (`consolidate/`)

- **`snapshot_consolidated_10562840.json.gz`** — consolidated snapshot at block 10562840 (2022-05-20 08:00 PDT). Contains each address's liquid and delegated ATOM balances plus its last vote on prop 69.
- **`last_vote_pro69.json.gz`** — all votes submitted while prop 69 was active, sourced from a Cosmos Hub archive node (quicksync.io cosmos-hub-4).
- **`validators.json`** — validator token-to-share ratios at snapshot height, used to convert delegation shares back to ATOM (accounting for slashing).

The consolidated snapshot was produced using [gnobounty7](https://github.com/piux2/gnobounty7). See [`consolidate/README.md`](consolidate/README.md) for the exact commands.

### AtomOne snapshot (`consolidate/`)

- **`snapshot_consolidated_atone_6439117.json.gz`** — consolidated snapshot of AtomOne at block 6439117. Produced using [`govbox`](https://github.com/atomone-hub/govbox). See [`consolidate/README.md`](consolidate/README.md) for the exact commands.

### Excluded addresses (`consolidate/excluded.txt`, `consolidate/ibc_escrow_address.txt`)

- CEX, custodial, mining pool, and other non-individual addresses listed in [`special-accounts.csv`](special-accounts.csv), are included, unless specifically stated in excluded.txt.

### Final balances (`mkgenesis/`)

- **`balances.txt.gz`** — the final genesis balance file, containing 3,262,505 lines totalling 1,002,461,998,378,908 ugnot. This is the direct input to the genesis block.

## How to verify

### Prerequisites

- Go 1.21+
- `jq`

### 1. Re-run the balance computation

```bash
cd consolidate
go run .
# Produces genbalance.txt.gz
```

Compare the output against the committed `genbalance.txt.gz` and the `mkgenesis/balances.txt.gz`.

### 2. Run the tests

```bash
cd consolidate
go test ./...
```

### 3. Re-create the Cosmos Hub consolidated snapshot from scratch

Follow the instructions in [`snapshot/cosmoshub_snapshot.md`](snapshot/cosmoshub_snapshot.md) to sync a full Cosmos Hub node and export state at block 10562840. Then follow [`consolidate/README.md`](consolidate/README.md) to rebuild `snapshot_consolidated_10562840.json.gz` using [gnobounty7](https://github.com/piux2/gnobounty7).

### 4. Re-create the AtomOne consolidated snapshot from scratch

Follow the instructions in [`consolidate/README.md`](consolidate/README.md) under the `snapshot_consolidated_atone.json` section, using the [`govbox`](https://github.com/atomone-hub/govbox) tool against an AtomOne genesis export at block [6439117](https://atomscan.com/atomone/blocks/6439117).

## Repository layout

```
consolidate/        Scripts and data for computing per-address GNOT weights
mkgenesis/          Final genesis balance file
prop69/             Raw prop 69 vote data (votes.csv, votes-unique.csv)
snapshot/           Instructions for taking a Cosmos Hub full-node snapshot
special-accounts.csv  Addresses excluded from the airdrop (CEX, custodial, etc.)
```

## Contributions

* https://github.com/gnolang/gno/pull/3, https://github.com/gnolang/gno/pull/6, https://github.com/gnolang/gno/pull/7, https://github.com/gnolang/gno/pull/11 - @piux2, big part of the initial version of the airdrop generator.
* https://github.com/gnolang/gno/pull/17 - @KorNatten, update special address list.
* 0xAN|Nodes.Guru `g1jj32fhrz6awxupdw5na244nxutjk99xk847wm2` for the 5/20 export.
