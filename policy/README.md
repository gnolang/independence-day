# policy/ — the human decisions

Everything here is a **judgement call**, not data. Each file is hand-edited and each one changes who gets
paid. Read the enforcement column carefully — they are not all wired up the same way.

| File | What it is | Enforcement |
|---|---|---|
| `excluded.txt` | Addresses removed from the airdrop: 3 DokiaCapital, 3 Interchain Foundation, 4 Cosmos Hub module accounts, 1 AtomOne DAO placeholder. | **Partially enforced** — read by `allocate/process_consolidated.go` → `skip()`. See the caveat below. |
| `ibc-escrow-addresses.txt` | The 426 IBC transfer escrow accounts, one per channel, as `cosmos1…:g1…:channel-N`. Derived, not curated. | **Loaded but not enforced** — the `return true` inside `skip()` is commented out. |
| `special-accounts.csv` | 78 rows annotating CEX, custodial, mining-pool, wallet, IBC and module addresses on Cosmos Hub. | **Not enforced at all.** No code reads this file. Tracked in issue #12. |
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

## `special-accounts.csv` — known quality issues

- Line 2 is a literal `cosmosxxxx,example` placeholder row.
- 78 rows resolve to 76 unique addresses: Kraken and Coinbase Custody each appear twice under different
  `type` labels.
- The `type` column is free text with 44 distinct values, including `''`, `'?'` and `'CEX or DEX'`.
- Several rows are **validator operator** addresses, not exchange hot wallets. Excluding a validator
  operator is a different decision from excluding an exchange, and hits different people.

Treat it as a research note, not a machine-readable list, until someone has done a human pass over it.
