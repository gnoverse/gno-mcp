# Networks — per-chain facts and cross-chain drift

**pearl** (chain-id `pearl-1`) is the public chain gnomcp writes to. Every value below was verified
live on **2026-09-15**; re-query anything load-bearing (`gno_status`, `auth/gasprice`, a realm's
render, whether a package resolves) before relying on it.

**This file describes genesis sets only.** A chain's genesis is fixed at launch and safe to write
down; everything deployed after it belongs to whoever deployed it and changes without notice, so no
package count, no namespace and no third-party package is recorded here. To find out what a chain
carries beyond genesis, ask it with `gno_packages`. Method and design live in the topic references;
this file is only the per-chain snapshot and the differences between chains.

## pearl

| Fact | Value |
|---|---|
| chain-id | `pearl-1` |
| RPC | `rpc.pearl.testnets.gno.land:443` |
| gnoweb | `pearl.testnets.gno.land` |
| node version | `v1.0.0-rc.0` |
| gas price (`auth/gasprice`) | `1ugnot/1000gas` |
| minimum fee for a 10M-gas write | 10,000 ugnot (0.01 GNOT) — gnomcp offers ×2 over the floor (`gnokey.md`) |
| storage deposit (`params/vm:p:storage_price`) | 100 ugnot |
| CLA deploy gate (`r/sys/cla`) | **OFF** — "enforcement is currently DISABLED" |
| namespace gate (`r/sys/names.IsEnabled`) | `true`; personal-address path free |
| name registration | `r/sys/namereg/v1`, not paused; format `nym-<stem><digits>` (stem 5–13 lowercase letters, exactly 3 digits, 12–20 chars) |
| faucet | 10 GNOT/grant, 1/addr/24h |
| tx indexer | `indexer.pearl.testnets.gno.land/graphql/query` — current, serves `getSupply` |
| toolchain tag | `chain/pearl` at `c4c72fdd2`, matching `heads/chain/pearl` |
| packages | a curated genesis set, plus whatever has been deployed since — query, never assume |

Toolchain tags use the short chain **name**, never the chain-id (`chain/pearl`, not `chain/pearl-1`).
The ref contains a `/`, so it is not a valid `go install @` ref and installs by commit SHA — see
`toolchain.md`. The node reports no build sha (`/status` carries a release `version` and an empty
`software`), so the tag is the only anchor the repo offers.

**Supply figures come from the indexer.** `getSupply(denom)` returns `total`, `locked` and
`spendable` at a given height, where `locked` is the unvested portion held by vesting accounts.
pearl created vesting accounts at genesis, which is what makes the split meaningful. Read it from
the indexer, never from a realm.

## Retired chains

**sapphire (`sapphire-1`)** is gone: its RPC, gnoweb, indexer and faucet hostnames no longer
resolve, so gnomcp ships no builtin profile for it and `sapphire-1` is no longer a writable
chain-id. **topaz (`topaz-1`)** went the same way before it; its tag `chain/topaz` (`fc40526`) sits
behind `heads/chain/topaz` (`63c2673`), which is why a tag is worth comparing against its branch
head before pinning.

Earlier numbered testnets (`test1`–`test13`) are likewise dead; the `test` name stays writable
because it also covers the e2e simnet's `test-9999`.

## Cross-chain drift — same import path, different source

A shared import path is not a shared implementation: chains cut from different masters carry
different sources under the same name. The split worth knowing in the packages these references
teach is in `p/demo/tokens/grc20`.

**On pearl, `CallerTeller()` hangs off `*PrivateLedger`**, not off `*Token`:

```go
teller := ledger.CallerTeller()  // pearl: func (ledger *PrivateLedger) CallerTeller() Teller
```

pearl also carries a `guardHome` check that confines a frame-relative teller to the token's own
realm, so a teller a realm builds and then exports is inert elsewhere. Older chains had neither, and
a realm written against one of them may pass tellers between realms in a way pearl rejects. Read the
**target chain's** deployed source (`gno_read` / `vm/qfile`) before relying on any package this file
does not cover.

## Deploying to pearl — the checklist

1. **Confirm the target** — `gno_status` (chain-id) or `gno_profile_list` (name ↔ chain-id map).
2. **Gates** — the personal-address path is free (namespace gate on, address paths always allowed).
   CLA enforcement is off today, so no `Sign` step is needed, but it is a chain setting: confirm
   with `gno_cla_info` rather than trusting this line.
3. **Fund** — the faucet grants 10 GNOT, once per address per 24h.
4. **Fees** — `1ugnot/1000gas`; still query `auth/gasprice`, since this is the value most likely to
   drift next.
5. **Imports** — `p/demo/tokens/grc721` and `p/nt/commondao/v0` are in no genesis set, so neither is
   there to import unless someone has deployed it. Verify every import against the target chain with
   `gno_packages`, never against master or this file.
6. **Transaction history** — `gno_activity`/`gno_history` read the indexer. To enumerate what is
   deployed, `gno_packages` reads the chain directly and needs no indexer.
7. **Local tests** — use the chain-matched toolchain and vendor on-chain deps from the matching
   source tree; a develop-HEAD toolchain can refuse to compile deps auto-fetched from a chain
   (`toolchain.md`).
