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
| block max gas | 3,000,000,000 | same |
| storage deposit (`params/vm:p:storage_price`) | 100 ugnot | same |
| deposit cap (`params/vm:p:default_deposit`) | 100 GNOT | same |
| code submission (`params/vm:p:code_submission_policy`) | `permissionless` | **`inert`** — see below |
| `MsgRun` (`params/vm:p:run_submitters`) | unrestricted | **allowlisted** — query the param for the current set |
| CLA deploy gate (`r/sys/cla`) | **OFF** | **OFF** |
| namespace gate (`r/sys/names.IsEnabled`) | `true`; personal-address path free | `true` |
| name registration | `r/sys/namereg/v1`, not paused | `r/sys/namereg/v0` — same realm renumbered, not paused, identical exported surface |
| validator set | `r/sys/validators/v3` holds it (`/v2` also deployed, empty) | `r/sys/validators/v0` holds it (`/v2` also deployed, empty) |
| faucet | 10 GNOT/grant, 1/addr/24h | none — mainnet ships without one |
| tx indexer | `indexer.pearl…/graphql/query` | `indexer.gno.land/graphql/query` — same query root, `getSupply` on both |
| toolchain tag | `chain/pearl` at `c4c72fdd2`, commit-only | `chain/mainnet` at `9c8eb132`, **semver twin `v1.2.0`** |
| sub-package path scheme | version at the **root** (`p/nt/avl/v0/rotree`) | version at the **leaf** (`p/nt/avl/rotree/v0`) |

The minimum fee for a 10M-gas write on pearl is 10,000 ugnot (0.01 GNOT); gnomcp offers ×2 over
the floor (`gnokey.md`). Mainnet runs the same gas price, so a write there would cost the same —
gnomcp simply never signs one.

Toolchain tags use the short chain **name**, never the chain-id (`chain/pearl`, not `chain/pearl-1`;
`chain/mainnet`, not `chain/gnoland-1`). Globbing a chain-id finds nothing. The two live chains fall
in different install cases: `chain/pearl` has no semver twin, so it installs by commit sha, while
`chain/mainnet` shares its sha with the annotated tag `v1.2.0`, which is the ref to install — see
`toolchain.md`. A tag can also lag its branch: `heads/chain/pearl` equals its tag, while
`heads/chain/mainnet` carries commits pushed after launch. No node reports a build sha (`/status`
carries a release `version` and an empty `software`), and both chains report the same one, so the
tag is the only anchor the repo offers.

## Mainnet — `gnoland-1`

Mainnet is a fresh chain whose balances come from an audited allocation rather than faucets, and
gnomcp treats it as read-only: no agent key, no session, no faucet, and both `master-address` and
the faucet fields refused at config time. A forced write stops at the keystore, which has no key to
give for a read-only chain-id. Reads and audits are the whole surface, which is what auditing
deployed code needs. It ships as the built-in `mainnet` profile, so reading it needs no config.

**Code submission is `inert`.** `params/vm:p:code_submission_policy` reads `"inert"` on mainnet
against `"permissionless"` on pearl. Under that policy the chain accepts a `MsgAddPackage` from any
address but **stores the package without typechecking or executing it**; it becomes callable only
once approved. So a deploy that reached mainnet would not run — the chain itself is a second barrier
behind gnomcp's read-only gate. The submission charge is empty today, so parking a package is free;
query `params/vm:p:inert_submission_charge` rather than assuming that holds.

**`MsgRun` is allowlisted.** `params/vm:p:run_submitters` carries a non-empty address list on
mainnet and is unset on pearl. `MsgRun` executes arbitrary source immediately under *every* policy,
including `inert`, which is why it gets its own gate. Query the param for the current set.

**No transfer restriction is active.** `params/bank:p:restricted_denoms` reads empty on both chains.
Read it rather than inferring a lock from launch tooling.

**`gnoland-1` and `gnoland1` are one hyphen apart and are different chains.** `gnoland-1` is
mainnet; `gnoland1` is betanet. Neither is writable, so confusing them cannot produce a write, but
it does decide which chain an audit reads.

**`gno.land` names mainnet.** It served betanet before mainnet launched, so a profile pinned to
that domain changed chains under a name that said otherwise. Read a chain-id rather than inferring
one from a hostname, and pin a chain's own hostnames when one exists. The same trap reaches the
toolchain: the dependency fetcher derives its remote from the **import path's domain**, so a
`gno.land/...` import resolves to `rpc.gno.land` — mainnet — whichever chain you are building for
(`toolchain.md`).

## Retired and halted chains

**betanet (`gnoland1`)** has stopped producing blocks. Its RPC, gnoweb and indexer still answer and
still serve its final state, which makes it the one retired chain a host check alone will not catch:
the height simply no longer moves. gnomcp ships no builtin for it, because a zero-config profile
would offer frozen state to read as if it were current. Reading its archive is still possible by
adding it deliberately with `gno_profile_add`. Its CLA gate was ENABLED, unlike pearl's and
mainnet's, and its `p/demo/tokens/grc20` carries the older `CallerTeller()` form described below.

**sapphire (`sapphire-1`)** is gone: its RPC, gnoweb, indexer and faucet hostnames no longer
resolve, so gnomcp ships no builtin profile for it and `sapphire-1` is no longer a writable
chain-id. **topaz (`topaz-1`)** went the same way before it; its tag `chain/topaz` (`fc40526`) sits
behind `heads/chain/topaz` (`63c2673`), which is why a tag is worth comparing against its branch
head before pinning.

Earlier numbered testnets (`test1`–`test13`) are likewise dead; the `test` name stays writable
because it also covers the e2e simnet's `test-9999`.

## Cross-chain drift — same import path, different source

A shared import path is not a shared implementation, and a shared package is not a shared path.
Three drifts matter between the live chains:

**1. The version segment sits in a different place on sub-packages.** Top-level packages share a
spelling: `p/nt/avl/v0` and `p/nt/mux/v0` resolve on both. Below the root they diverge, because
pearl hangs sub-packages under the version and mainnet gives each leaf its own.

```go
gno.land/p/nt/avl/v0/rotree                  // pearl
gno.land/p/nt/avl/rotree/v0                  // mainnet
gno.land/p/nt/ownable/v0/exts/authorizable   // pearl
gno.land/p/nt/ownable/exts/authorizable/v0   // mainnet
```

An import block that stays on root packages carries across; one that reaches a sub-package does
not. Resolve every import against the target chain with `gno_packages`.

**2. The GRC20 standard lives at a different path.** pearl carries it at
`p/demo/tokens/grc20`; mainnet has no `p/demo` tree at all and carries the standard at
`p/nt/grc20/v0`. The deployed sources are otherwise the same package.

**3. `grc20.TransferFrom` guards self-transfer on mainnet only.** mainnet's `token.gno` rejects
`owner == to` with `ErrCannotTransferToSelf`; pearl's does not, and the guard exists there only on
`Transfer`. A self-directed `TransferFrom` succeeds on pearl and fails on mainnet. The rest of the
package, `tellers.gno` included, is identical between them.

**`CallerTeller()` hangs off a different type on the halted betanet.**

```go
teller := ledger.CallerTeller()  // pearl and mainnet: func (ledger *PrivateLedger) CallerTeller() Teller
teller := tok.CallerTeller()     // betanet: func (tok *Token) CallerTeller() Teller
```

Both live chains also carry a `guardHome` check that confines a frame-relative teller to the token's
own realm, so a teller a realm builds and then exports is inert elsewhere. betanet has neither the
receiver change nor the guard, so a realm archived there may pass tellers between realms in a way
both live chains would reject — worth knowing when reading that code, not when writing new code.

The package sets differ too, and not only by path: the NFT standard is in mainnet's genesis set
(under the `p/nt` tree with a `/v0` leaf) and resolves on neither pearl nor betanet, while
`p/nt/commondao/v0` resolves on betanet and on neither live chain. `p/demo/tokens/grc721` resolves
nowhere. Read the **target chain's** deployed source (`gno_read` / `vm/qfile`) before relying on any
package this file does not cover.

## Deploying — pearl is the only public target

1. **Confirm the target** — `gno_status` (chain-id) or `gno_profile_list` (name ↔ chain-id map).
   A read-only chain has no deploy path at all, so a deploy that "should" go to gno.land is a
   pearl deploy or nothing. Mainnet would park the package inert even if one reached it.
2. **Gates** — the personal-address path is free (namespace gate on, address paths always allowed).
   CLA enforcement is off on pearl today, so no `Sign` step is needed, but it is a chain setting:
   confirm with `gno_cla_info` rather than trusting this line.
3. **Fund** — the faucet grants 10 GNOT, once per address per 24h.
4. **Fees** — `1ugnot/1000gas`; still query `auth/gasprice`, since this is the value most likely to
   drift next.
5. **Imports** — resolve every import against the target chain with `gno_packages`, never against
   master or this file. Sub-package paths and the GRC20 path are spelled differently on the two
   live chains, so a working import block does not transfer unchecked.
6. **Transaction history** — `gno_activity`/`gno_history` work on pearl and mainnet. To enumerate
   what is deployed, `gno_packages` reads the chain directly and needs no indexer.
7. **Local tests** — use the chain-matched toolchain and vendor on-chain deps from the matching
   source tree; a develop-HEAD toolchain can refuse to compile deps auto-fetched from a chain, and
   the fetcher's default remote is mainnet's whatever you are targeting (`toolchain.md`).
