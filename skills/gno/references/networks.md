# Networks — per-chain facts and cross-chain drift

**pearl** (chain-id `pearl-1`) is the one public chain gnomcp can write to. **mainnet**
(`gnoland-1`) is read-only: reads and audits work, no code path signs anything there. Every value
below was verified live on **2026-09-15**; re-query anything load-bearing (`gno_status`,
`auth/gasprice`, a realm's render, whether a package resolves) before relying on it.

**This file describes genesis sets only.** A chain's genesis is fixed at launch and safe to write
down; everything deployed after it belongs to whoever deployed it and changes without notice, so no
package count, no namespace and no third-party package is recorded here. To find out what a chain
carries beyond genesis, ask it with `gno_packages`. Method and design live in the topic references;
this file is only the per-chain snapshot and the differences between chains.

## Per-chain matrix

| Fact | pearl — writable | mainnet — read-only |
|---|---|---|
| chain-id | `pearl-1` | `gnoland-1` |
| writes through gnomcp | deploys, calls, sessions, faucet | **none** — read tools only |
| RPC | `rpc.pearl.testnets.gno.land:443` | `rpc.gno.land:443` |
| gnoweb | `pearl.testnets.gno.land` | `gno.land` |
| node version | `v1.0.0-rc.0` | `v1.0.0-rc.0` |
| gas price (`auth/gasprice`) | `1ugnot/1000gas` | same |
| storage deposit (`params/vm:p:storage_price`) | 100 ugnot | same |
| CLA deploy gate (`r/sys/cla`) | **OFF** | **OFF** |
| namespace gate (`r/sys/names.IsEnabled`) | `true`; personal-address path free | `true` |
| name registration | `r/sys/namereg/v1`, not paused; format `nym-<stem><digits>` (stem 5–13 lowercase letters, exactly 3 digits, 12–20 chars) | — |
| faucet | 10 GNOT/grant, 1/addr/24h | none — mainnet ships without one |
| tx indexer | `indexer.pearl…/graphql/query`, serves `getSupply` | `indexer.gno.land/graphql/query` |
| toolchain tag | `chain/pearl` at `c4c72fdd2`, matching `heads/chain/pearl` | — |
| packages | a curated genesis set, plus whatever has been deployed since — query, never assume | same, and a different set: no `p/demo/tokens/grc20`, no `r/sys/namereg/v1` |

The minimum fee for a 10M-gas write on pearl is 10,000 ugnot (0.01 GNOT); gnomcp offers ×2 over
the floor (`gnokey.md`). Fees do not arise on mainnet.

Toolchain tags use the short chain **name**, never the chain-id (`chain/pearl`, not `chain/pearl-1`).
Both refs contain a `/`, so neither is a valid `go install @` ref and both install by commit SHA —
see `toolchain.md`. No node reports a build sha (`/status` carries a release `version` and an empty
`software`), so the tag is the only anchor the repo offers.

## Mainnet — `gnoland-1`

Mainnet is a fresh chain whose balances come from an audited allocation rather than faucets, and
gnomcp treats it as read-only: no agent key, no session, no faucet, and both `master-address` and
the faucet fields refused at config time. A forced write stops at the keystore, which has no key to
give for a read-only chain-id. Reads and audits are the whole surface, which is what auditing
deployed code needs. It ships as the built-in `mainnet` profile, so reading it needs no config.

**`gnoland-1` and `gnoland1` are one hyphen apart and are different chains.** `gnoland-1` is
mainnet; `gnoland1` is betanet. Neither is writable, so confusing them cannot produce a write, but
it does decide which chain an audit reads.

**`gno.land` names mainnet.** It served betanet before mainnet launched, so a profile pinned to
that domain changed chains under a name that said otherwise. Read a chain-id rather than inferring
one from a hostname, and pin a chain's own hostnames when one exists.

## Retired and halted chains

**betanet (`gnoland1`)** has stopped producing blocks. Its RPC, gnoweb and indexer still answer and
still serve its final state, which makes it the one retired chain a host check alone will not catch:
the height simply no longer moves. gnomcp ships no builtin for it, because a zero-config profile
would offer frozen state to read as if it were current. Reading its archive is still possible by
adding it deliberately with `gno_profile_add`. Its CLA gate was ENABLED, unlike pearl's and
mainnet's, so a realm deployed there was signed for; its `r/sys/namereg/v1` never existed, and its
`p/demo/tokens/grc20` carries the older `CallerTeller()` form described below.

**sapphire (`sapphire-1`)** is gone: its RPC, gnoweb, indexer and faucet hostnames no longer
resolve, so gnomcp ships no builtin profile for it and `sapphire-1` is no longer a writable
chain-id. **topaz (`topaz-1`)** went the same way before it; its tag `chain/topaz` (`fc40526`) sits
behind `heads/chain/topaz` (`63c2673`), which is why a tag is worth comparing against its branch
head before pinning.

Earlier numbered testnets (`test1`–`test13`) are likewise dead; the `test` name stays writable
because it also covers the e2e simnet's `test-9999`.

## Cross-chain drift — same import path, different source

A shared import path is not a shared implementation: chains cut from different masters carry
different sources under the same name. The one API split found in the packages these references
teach, between pearl and the halted betanet:

**`p/demo/tokens/grc20` — `CallerTeller()` hangs off a different type per chain.**

```go
teller := ledger.CallerTeller()  // pearl: func (ledger *PrivateLedger) CallerTeller() Teller
teller := tok.CallerTeller()     // betanet: func (tok *Token) CallerTeller() Teller
```

pearl also carries a `guardHome` check that confines a frame-relative teller to the token's own
realm, so a teller a realm builds and then exports is inert elsewhere. betanet has neither the
receiver change nor the guard, so a realm archived there may pass tellers between realms in a way
pearl would reject — worth knowing when reading that code, not when writing new code.

The package sets differ as well as the sources: `r/sys/namereg/v1` exists on pearl and not on
betanet, `p/nt/commondao/v0` the other way round. Read the **target chain's** deployed source
(`gno_read` / `vm/qfile`) before relying on any package this file does not cover.

## Deploying — pearl is the only public target

1. **Confirm the target** — `gno_status` (chain-id) or `gno_profile_list` (name ↔ chain-id map).
   A read-only chain has no deploy path at all, so a deploy that "should" go to gno.land is a
   pearl deploy or nothing.
2. **Gates** — the personal-address path is free (namespace gate on, address paths always allowed).
   CLA enforcement is off on pearl today, so no `Sign` step is needed, but it is a chain setting:
   confirm with `gno_cla_info` rather than trusting this line.
3. **Fund** — the faucet grants 10 GNOT, once per address per 24h.
4. **Fees** — `1ugnot/1000gas`; still query `auth/gasprice`, since this is the value most likely to
   drift next.
5. **Imports** — `p/demo/tokens/grc721` and `p/nt/commondao/v0` are in no genesis set, so neither
   is there to import unless someone has deployed it. Verify every import against the target chain
   with `gno_packages`, never against master or this file.
6. **Transaction history** — `gno_activity`/`gno_history` work on pearl and mainnet. To enumerate
   what is deployed, `gno_packages` reads the chain directly and needs no indexer.
7. **Local tests** — use the chain-matched toolchain and vendor on-chain deps from the matching
   source tree; a develop-HEAD toolchain can refuse to compile deps auto-fetched from a chain
   (`toolchain.md`).
