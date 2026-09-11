# independence-day

**The auditable record of the GNOT genesis allocation.**

This repository computes, from public chain snapshots and a published set of rules, how much GNOT every
address receives at genesis. It produced the balance sheet that launched **gnoland1** — **3,262,507**
accounts, at commit [`9dec38a4`](https://github.com/gnolang/independence-day/tree/9dec38a4a72c9e84db7e78ae010370de250f2d64),
totalling 1,002,461,998.383260 GNOT — and every input, intermediate and script used to get there is
committed here so that anyone can re-derive the numbers independently.

Note that `main` has moved on since: the row count and the totals on this branch are **not** the ones
that launched `gnoland1`. [`docs/history.md`](docs/history.md) records which commit produced which
chain.

> **Want to run a gnoland1 validator?** That lives elsewhere:
> <https://github.com/gnolang/gno/tree/chain/gnoland1/misc/deployments/gnoland1>

---

## Contents

- [The allocation](#the-allocation)
- [How your balance was computed](#how-your-balance-was-computed)
- [Repository layout](#repository-layout)
- [Reproducing the result](#reproducing-the-result)
- [Verifying your own line](#verifying-your-own-line)
- [Stable paths](#stable-paths--do-not-move-these)
- [Credits](#credits)

---

## The allocation

Constants live in one place: [`allocate/process_consolidated.go`](allocate/process_consolidated.go),
in the `const` block at the top. They are the *only* place a bucket size is defined.

| Bucket | GNOT | Where it goes |
|---|---:|---|
| ATOM airdrop | 350,000,000 | Cosmos Hub ATOM holders, snapshot block 10562840 (2022-05-20 08:00 PDT) |
| AtomOne airdrop | 231,000,000 | AtomOne ATONE/PHOTON holders, snapshot block 6439117 |
| Investors — unlocked at launch (§136) | 150,000,000 | 128,395,312.569897 to `g1j3et7juxr3npgdll3lml3mpv0y6m49rztjnf76` — no vesting schedule — and 21,604,687.430103 to the public sale, below |
| Investors — vesting (§132) | 150,000,000 | `g1x7tm26g9wj84cmg3cs74uwf3g9lqj4mjp6gax3` |
| NT,LLC (§124) | 332,000,000 | `g1pku9u3jwr8k8vjpfypqzd0uhmrwr35zk0f8u7p` |
| Core Treasury (§120) | 39,994,000 | `g1shmvjxkvx9kgnrta5rzwcpdqszy4pkfvv9qjz9`; 40,000,000 less the founders |
| Ecosystem Treasury (§121) | 59,511,000 | `g1ugke9x9ylrlex0lxcgw7eu0mdftvcrgglru0l0`; 60,000,000 less the premine |
| Validator Services Treasury (§122) | 20,000,000 | `g1kj5ag4xdjws00rfzg49x6lljv34pty5uchcq2p` |
| GovDAO founders | 6,000 | 1,000 each to 6 of the 7 — Jae is skipped, he holds a snapshot entitlement. Charged to the Core Treasury (§333) |
| Non-airdrop premine | 489,000 | `mkgenesis/non-airdrop.txt` — charged to the Ecosystem Treasury |
| **Total** | **1,333,000,000** | |

> **The six addresses are real multisigs**, from `gnolang/multisigs`. Each is the same signer set as the
> account it succeeds — the three treasuries are the `[govdao]` members, the three ex-nt1 buckets are the
> `[nt1]` members — plus one provably-unspendable salt key, so that six purposes get six distinct
> addresses rather than collapsing onto two. Effective thresholds are unchanged (4-of-7 and 4-of-6): a
> salt key is a real curve point with no private key, so it can never contribute a signature.

### Unrestricted addresses (§126)

At genesis `$GNOT` is globally non-transferable, and §126 requires an exemption list.
[`mkgenesis/unrestricted.txt`](mkgenesis/unrestricted.txt) is it — **82 addresses**, generated: the
three funds §127 names (Ecosystem, and both Investors tranches) plus every row of `publicsale.txt`.
It is derived rather than hand-written so a participant who binds an address cannot end up in the
genesis but off the exemption list.

**Whitelisting is not unlocking.** §129 — *"Whitelisted funds remain subject to the vesting schedule
below"* — so the investors-vesting tranche is on the list *and* keeps its §132 schedule.

NT,LLC, the Core and Validator Services treasuries and nt2 are deliberately **absent**; each needs a
ruling rather than a default. See the header of the file.

Three things are *not* separate buckets and surprise people:

- **The public token sale** is a **carve-out of the §136 unlocked tranche**, not an eighth bucket. The
  Sonar sale settled 122 wallets for 21,604,687.430103 GNOT; buyers are investors and their tokens are
  contractually lockup-free, so the sale is simply the part of that 150,000,000 which is already owed
  to named people. It comes *out of* the tranche, which is why the row above reads 128,395,312.569897
  rather than 150,000,000. The 78 wallets that had named a gno.land address by the snapshot are paid
  directly; the other 44 are paid to a 2-of-4 multisig
  (`g1rphzpk58kn0nqpgu8k8apaq2ftzgpsgql8wjr0`) and distributed by hand as they come forward. See
  [`mkgenesis/publicsale.txt`](mkgenesis/publicsale.txt) and
  [`inputs/README-publicsale-provenance.md`](inputs/README-publicsale-provenance.md).


- **nt2 (`g1sp27hn785v3kud6cg9dnhrng7wzp9cnljffhcg`)** is a **sweep, not an allocation**. The AiB
  addresses listed in `aibCosmosAddrs` / `aibAtoneAddrs` are removed from the airdrop and their combined
  entitlement is re-added under nt2. That GNOT comes *out of* the 350M + 231M, not on top.
- **[`mkgenesis/non-airdrop.txt`](mkgenesis/non-airdrop.txt)** adds 489,000 GNOT of pre-airdrop premine
  (early contributors, GitHub requesters, a 1,000 GNOT gas float for each multisig signer key, and the
  `examples/` contributor airdrop) at the very last step. Some of it is dated 2022, and it is
  paid for by deducting the same amount from the Ecosystem Treasury — which is why that row reads
  59,511,000 rather than 60,000,000. There are no faucet rows: there is no mainnet faucet. Controlled by
  `PREMINE_ABSORBED_FROM_CONTRIBS` in [`allocate/process_consolidated.go`](allocate/process_consolidated.go).
  See [`docs/pipeline.md`](docs/pipeline.md#the-non-airdrop-premine).

## How your balance was computed

### Cosmos Hub (ATOM)

Each address gets a **weight** from its liquid ATOM (`uatom`) and delegated ATOM (`duatom`) at the
snapshot, modified by how it voted on Cosmos Hub
[Proposal 69](https://www.mintscan.io/cosmos/proposals/69):

| Vote on prop 69 | Weight |
|---|---|
| YES | `uatom` only — **staked ATOM is excluded entirely** |
| NO | `uatom + duatom × 1.5` |
| NO WITH VETO | `uatom + duatom × 2` |
| ABSTAIN, or did not vote | `uatom + duatom` |

Then `ugnot = weight / total_weight × 350,000,000 × 1,000,000`, truncated (not rounded) to a whole ugnot.

### AtomOne (ATONE / PHOTON)

No governance weighting. `weight = uatone + duatone + uphoton / 7` (integer division), then the same
proportional split against 231,000,000.

### Exclusions

- [`policy/excluded.txt`](policy/excluded.txt) — addresses removed from the airdrop entirely.
- [`policy/ibc-escrow-addresses.txt`](policy/ibc-escrow-addresses.txt) — the 426 derived IBC transfer
  escrow accounts.
- [`policy/special-accounts.csv`](policy/special-accounts.csv) — annotation of CEX / custodial /
  validator / module addresses.

Read [`policy/README.md`](policy/README.md) before assuming any of these is enforced — one of them is
**annotation only**, and the exact enforcement status of each is documented there.

## Repository layout

The tree reads in pipeline order.

```
inputs/      immutable chain snapshots + how they were captured   (never edited after capture)
policy/      human decisions: exclusions, annotations, escrow list
allocate/    the allocation engine (Go) -> genbalance.txt.gz
mkgenesis/   final merge with the premine -> balances.txt.gz      (the genesis file)
verify/      independent cross-checks of the outputs
archive/     historical material, kept for provenance; not part of the pipeline
docs/        allocation rules, pipeline detail, reproduction, history
```

Each directory has its own `README.md` explaining what is in it and whether it is live or historical.

## Reproducing the result

Full detail — including rebuilding the snapshots from a gaiad export — is in
[`docs/reproduce.md`](docs/reproduce.md). The short version:

```sh
make            # allocate -> genbalance.txt.gz -> mkgenesis -> balances.txt.gz
make verify     # unit tests + independent cross-check + supply total
```

Requirements: Go 1.21+ and `gzip`. Both stages are Go, so there is nothing else to install.

**The pipeline is bit-reproducible.** Running `make` on a clean checkout regenerates
`allocate/genbalance.txt.gz` byte-for-byte, and `mkgenesis/balances.txt` byte-for-byte. Verified on
linux/amd64 and darwin/arm64. If your run differs, that is a bug and we want to hear about it.

## Verifying your own line

See [`docs/verify-your-balance.md`](docs/verify-your-balance.md). It walks through, for one address:
finding your row in the snapshot, reading your prop-69 vote, computing your weight by hand, and
confirming it against the shipped `balances.txt.gz`.

## Stable paths — do not move these

External tooling fetches raw URLs into this repository. **These paths are a public contract:**

| Path | Fetched by |
|---|---|
| `mkgenesis/balances.txt.gz` | `gnolang/gno` → `misc/deployments/gnoland1/gen-genesis.sh`, `misc/deployments/test13.gno.land/gen-genesis.sh`, `misc/deployments/mainnet.gno.land/gen-genesis.sh` (pinned by commit sha), and `misc/deployments/test{2,3}.gno.land/Makefile` (unpinned, `raw/main/...`) |
| `mkgenesis/unrestricted.txt` | `gnolang/gno` → `misc/deployments/mainnet.gno.land/gen-genesis.sh` (pinned by commit sha + sha256) — the §126 exemption list |
| `mkgenesis/non-airdrop.txt` | same pipeline, indirectly |

`mkgenesis/` therefore keeps its historical name even though the rest of the tree was renamed for
clarity. Anything that moves `mkgenesis/balances.txt.gz` breaks every unpinned consumer silently — the
download just 404s. If it ever has to move, land the redirect first.

`mkgenesis/README.md` and `mkgenesis/unrestricted.txt` are **generated** (`go run . readme` /
`go run . unrestricted`). Do not hand-edit them.

## Credits

See [`CONTRIBUTIONS.md`](CONTRIBUTIONS.md).
