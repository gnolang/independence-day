# policy/ — the human decisions

Everything here is a **judgement call**, not data. Each file is hand-edited and each one changes who gets
paid. Read the enforcement column carefully — they are not all wired up the same way.

| File | What it is | Enforcement |
|---|---|---|
| `excluded.txt` | Addresses removed from the airdrop: 3 DokiaCapital, 3 Interchain Foundation, 4 Cosmos Hub module accounts, 1 AtomOne DAO placeholder. | **Partially enforced** — read by `allocate/process_consolidated.go` → `skip()`. See the caveat below. |
| `ibc-escrow-addresses.txt` | The 426 IBC transfer escrow accounts, one per channel, as `cosmos1…:g1…:channel-N`. Derived, not curated. | **Enforced.** 106 of the 426 held a balance and are now skipped. |
| `special-accounts.csv` | 78 rows annotating CEX, custodial, mining-pool, wallet, IBC and module addresses on Cosmos Hub. | **Readable but inert by default** — enforced only for the classes named in `excluded-types.txt`, which ships empty. Tracked in issue #12. |
| `excluded-types.txt` | Exclusion by *class*, matched against the `type` column of `special-accounts.csv`. **Every line is commented out**, so nothing is excluded today. | active only when a line is uncommented |
| `gen-ibc-escrow/` | One-shot generator for `ibc-escrow-addresses.txt`. | run by hand |

## `excluded.txt` — the matching caveat

Exclusions are matched as **literal bech32 strings**, and the entries are written in `cosmos1…` form.
The AtomOne qualification path passes `atone1…` addresses for the *same underlying 20-byte keys*, so a
`cosmos1…` entry does not exclude that key on the AtomOne side.

If you are auditing this repository, that is the first thing to check. Fixing it means normalising to the
20-byte payload before comparison — in `skip()`, `loadExcludedAddresses()` and `loadEscrowAddress()`
alike, since the escrow list has the same shape.

## `ibc-escrow-addresses.txt` — how it is derived

`gen-ibc-escrow/main.go` computes, for `channel-0` … `channel-425`:

```
addr20 = sha256("ics20-1" || 0x00 || "transfer/channel-N")[:20]
```

and emits the `cosmos1` and `g1` bech32 encodings of it. These are **keyless module accounts** — no
private key exists for them on any chain. Any GNOT credited to one is permanently unspendable, so
crediting them is a burn, not an allocation.

```sh
cd policy/gen-ibc-escrow && go run .    # rewrites ../ibc-escrow-addresses.txt
```

## `excluded-types.txt` — excluding by class

`special-accounts.csv` was annotation that no code read. `excluded-types.txt` makes it actionable
without turning it into an address list: the decision stays expressed as *"no exchanges"* rather than as
thirty hand-copied addresses that go stale the moment the CSV is updated.

Patterns match the `type` column exactly, except that a single trailing `*` matches any type with that
prefix — which is what makes `Upbit*` cover `Upbit #01 (Deposit)` through `Upbit #20 (Staking)`.
Matching is case-insensitive. **A pattern that matches no row is a hard error**, so a typo cannot
silently exclude nothing.

Addresses are matched on the 20-byte payload, so one entry covers both the `cosmos1…` and `atone1…`
encodings of a key.

**Excluding does not reduce total supply.** The airdrop buckets are fixed at 350M and 231M and are
distributed proportionally, so every excluded GNOT is redistributed to the remaining holders of that
bucket. It is a transfer from the named class to everyone else, not a burn. The file lists each class's
current total so the size of that transfer is visible before anyone uncomments a line.

## `special-accounts.csv` — known quality issues

- Line 2 is a literal `cosmosxxxx,example` placeholder row.
- 78 rows resolve to 76 unique addresses: Kraken and Coinbase Custody each appear twice under different
  `type` labels.
- The `type` column is free text with 44 distinct values, including `''`, `'?'` and `'CEX or DEX'`.
- Several rows are **validator operator** addresses, not exchange hot wallets. Excluding a validator
  operator is a different decision from excluding an exchange, and hits different people.

Treat it as a research note, not a machine-readable list, until someone has done a human pass over it.
