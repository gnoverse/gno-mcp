# Networks — per-chain facts and cross-chain drift

**pearl** (chain-id `pearl-1`) is the one public chain gnomcp can write to. **mainnet**
(`gnoland-1`) is read-only: reads and audits work, no code path signs anything there. Every value
below was verified live on **2026-09-28**; re-query anything load-bearing (`gno_status`,
`auth/gasprice`, a realm's render, whether a package resolves) before relying on it.

**Run `gno_status` before quoting anything here.** This file covers exactly two chain-ids,
`pearl-1` and `gnoland-1`. A local gnodev, an e2e simnet, a staging chain and any newer testnet are
none of them, and nothing below describes them — their gas price, faucet policy, package set and
gate states are their own. Confirm the chain-id you are actually connected to first; if it is
neither of the two, read the value off that chain instead of this page, and say which chain your
answer is about.

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
| release | `chain/pearl` build | read `misc/deployments/mainnet.gno.land/upgrades.json` for the current one — `chain/mainnet`'s tag names only its genesis build |
| `/status` | `node_info.version` is `v1.0.0-rc.0` and `software` empty on every chain — neither identifies a release. `build_version` does: read it | same |
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
| faucet | present — read the grant size and per-address cap from `gno_status`'s `faucet` block, never from this row | none — mainnet ships without one |
| tx indexer | `indexer.pearl…/graphql/query` | `indexer.gno.land/graphql/query` — same query root, `getSupply` on both |
| toolchain tag | `chain/pearl` at `c4c72fdd2`, commit-only | `chain/mainnet` at `9c8eb132`, semver twin `v1.2.0` — genesis only, superseded by the upgrade ledger |
| sub-package path scheme | version at the **root** (`p/nt/avl/v0/rotree`) | version at the **leaf** (`p/nt/avl/rotree/v0`) |

The minimum fee for a 10M-gas write on pearl is 10,000 ugnot (0.01 GNOT); gnomcp offers ×2 over
the floor (`gnokey.md`). Mainnet runs the same gas price, so a write there would cost the same —
gnomcp simply never signs one.

Toolchain tags use the short chain **name**, never the chain-id (`chain/pearl`, not `chain/pearl-1`;
`chain/mainnet`, not `chain/gnoland-1`). Globbing a chain-id finds nothing. A tag can also lag its
branch: `heads/chain/pearl` equals its tag, while `heads/chain/mainnet` carries commits pushed after
launch. Neither tag is automatically the ref to install — read the chain's `upgrades.json` first and
fall back to the tag only when it has no entry (`toolchain.md`).

**A chain tag names its genesis build, not what it runs today.** `chain/mainnet` is mainnet's
`v1.2.0` genesis; the chain has since been halted and restarted on newer binaries by governance.

**Two places say what a chain runs, and they agree.** `/status` carries a `build_version` naming
the branch, commit depth and short sha of the running binary — mainnet's resolves to the same
commit as the newest entry in `misc/deployments/mainnet.gno.land/upgrades.json`, the ledger of
those restarts. Ask the node for what it is running and the ledger for what was agreed; a
disagreement is worth chasing rather than averaging. Do not read `node_info.version` for this —
it is `v1.0.0-rc.0` with an empty `software` on every chain and identifies nothing.

## Mainnet — `gnoland-1`

Mainnet is a fresh chain whose balances come from an audited allocation rather than faucets, and
gnomcp treats it as read-only: no agent key, no session, no faucet, and both `master-address` and
the faucet fields refused at config time. A forced write stops at the keystore, which has no key to
give for a read-only chain-id. Reads and audits are the whole surface, which is what auditing
deployed code needs. It ships as the built-in `mainnet` profile, so reading it needs no config.

**Code submission is `inert`.** `params/vm:p:code_submission_policy` reads `"inert"` on mainnet
against `"permissionless"` on pearl. Anyone may submit — `params/vm:p:code_submitters` is unset —
but the chain **parks** the package instead of running it: no typecheck, no `init()`, stored in a
key space of its own. An address in `params/vm:p:pkg_approvers` then sends `MsgEnablePackage`, which
typechecks the source and runs `init()` on *its* transaction and gas, with the submitter as
`OriginCaller`. Only then does the package exist. Either side can abandon a parked submission with
`MsgRejectPackage`; nothing expires one, and the submission charge is not refunded.

Mainnet is therefore not a museum: packages deployed after genesis do run there, once approved.
Treat "submitted" and "live" as different states with an unpredictable gap between them.

**A parked package is invisible to every ordinary read.** `vm/qpaths` skips it; `vm/qfuncs`,
`vm/qeval` and `vm/qrender` answer `package not found` and `vm/qfile` answers
`package … is not available` — in each case the *same* answer a path
that was never submitted gets. Its source cannot be read back at all; only the submitting
transaction carries it. Two queries exist for this and nothing else:

```bash
gnokey query vm/qpkgmeta_json -data "gno.land/r/x/y"    # status: "live" | "inert" | "absent"
gnokey query "vm/qinertpaths?limit=100" -data "gno.land/r/"   # everything awaiting approval
```

`qpkgmeta_json` is the only way to tell a parked package from one that does not exist, and it also
carries the reason a parked one is not live yet. Reach for it before reporting that a path is
missing on a chain running `inert`. Never probe a path's existence with a call: a call into a
parked path returns an internal error rather than a clean not-found (look for `unexpected node
with location` in the log), so a read answers the question more cheaply and more legibly.

**`MsgRun` is allowlisted.** `params/vm:p:run_submitters` carries a non-empty address list on
mainnet and is unset on pearl. `MsgRun` executes arbitrary source immediately under *every* policy,
including `inert`, which is why it gets its own gate. Query the param for the current set.

**No transfer restriction is active.** `params/bank:p:restricted_denoms` reads empty on both chains.
Read it rather than inferring a lock from launch tooling.

**`gnoland-1` and `gnoland1` are one hyphen apart and are different chains.** `gnoland-1` is
mainnet; `gnoland1` was betanet, whose hosts are gone. Neither is writable, so confusing them
cannot produce a write — a `gnoland1` profile fails to connect at all.

**`gno.land` names mainnet.** It served betanet before mainnet launched, so a profile pinned to
that domain changed chains under a name that said otherwise. Read a chain-id rather than inferring
one from a hostname, and pin a chain's own hostnames when one exists. The same trap reaches the
toolchain: the dependency fetcher derives its remote from the **import path's domain**, so a
`gno.land/...` import resolves to `rpc.gno.land` — mainnet — whichever chain you are building for
(`toolchain.md`).

## Retired chains

**betanet (`gnoland1`), sapphire (`sapphire-1`) and topaz (`topaz-1`) are gone.** Their RPC, gnoweb
and indexer hostnames do not resolve, so gnomcp ships no builtin for any of them, nothing archived
on them is readable from the chain, and `sapphire-1` is not a writable chain-id. topaz's tag
`chain/topaz` (`fc40526`) sits behind `heads/chain/topaz` (`63c2673`), which is why a tag is worth
comparing against its branch head before pinning.

**A host that answers does not prove a chain is live.** betanet spent its last stretch resolving,
replying to every read, and serving a height that never moved — a retirement no host check catches.
Sample the height from `gno_status` twice before treating a chain as current.

Earlier numbered testnets (`test1`–`test13`) are likewise dead; the `test` name stays writable
because it also covers the e2e simnet's `test-9999`.

## Cross-chain drift — same import path, different source

A shared import path is not a shared implementation, and a shared package is not a shared path.
The two live chains also run different VM releases, so the *language* differs between them too.
Four drifts matter:

**0. The language itself. mainnet rejects code pearl accepts.** mainnet has been upgraded past its
genesis build while pearl still runs the release it launched on. Two rules are measured to differ:

**A crossing `cur` is a fixed binding on mainnet.** Reassigning it, taking `&cur`, or
range-assigning to it all compile on pearl and fail at preprocess on mainnet, each with its own
message — `cannot reassign the crossing 'cur' parameter`, `cannot take the address of a
realm-typed 'cur'`, `cannot assign to a realm-typed 'cur' in a range clause`.

**`AssertOriginCall()` reached through a re-export panics on mainnet.** Where one realm exposes
another's crossing function as `var Deposit = other.Deposit`, calling the alias satisfies the check
on pearl and panics on mainnet with `invalid non-origin call`. mainnet anchors the origin call to
the entry package; pearl counts frames. Calling the original directly succeeds on both.

A realm that compiles and tests green against pearl can therefore fail on mainnet — at submit for
the first rule, and at call time for the second, which no local `gno test` against the wrong release
will show. Build against the target chain's own release (`toolchain.md`) rather than assuming one
binary serves both.

**Measure a suspected difference; never infer one from a changelog.** Whether an upstream PR is an
ancestor of a chain's build says nothing about whether the behaviour is present: a release that
rewrites a rule's message or mechanism looks like the rule arriving. `iota` as an ordinary
identifier reads as exactly that trap — both live chains reject it, and only the wording differs
(`cannot use iota outside constant declaration` on pearl, `builtin identifiers cannot be shadowed`
on mainnet). Install both releases (`toolchain.md`) and lint the same file against each.

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

**`CallerTeller()` hangs off `*PrivateLedger` on both live chains.**

```go
teller := ledger.CallerTeller()  // func (ledger *PrivateLedger) CallerTeller() Teller
```

Both also carry a `guardHome` check that confines a frame-relative teller to the token's own realm,
so a teller a realm builds and then exports is inert elsewhere. An older GRC20 hung the same method
off `*Token` with no guard; `security.md` reads what that receiver decides, and a fork can still
carry it.

The package sets differ too, and not only by path: the NFT standard is in mainnet's genesis set
(under the `p/nt` tree with a `/v0` leaf) and does not resolve on pearl, while `p/nt/commondao/v0`
resolves on neither live chain. `p/demo/tokens/grc721` resolves nowhere. Read the **target chain's**
deployed source (`gno_read` / `vm/qfile`) before relying on any package this file does not cover.

## Deploying — pearl is the only public target

1. **Confirm the target** — `gno_status` (chain-id) or `gno_profile_list` (name ↔ chain-id map).
   A read-only chain has no deploy path at all, so a deploy that "should" go to gno.land is a
   pearl deploy or nothing. Mainnet would park the package inert even if one reached it.
2. **Gates** — the personal-address path is free (namespace gate on, address paths always allowed).
   CLA enforcement is off on pearl today, so no `Sign` step is needed, but it is a chain setting:
   confirm with `gno_cla_info` rather than trusting this line.
3. **Fund** — `gno_faucet_fund`. The grant size and the per-address cooldown are operator settings:
   `gno_status` carries them in its `faucet` block, and the refusal message names the cap it hit. A
   capped address is not a broken faucet — a fresh key has its own allowance.
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
