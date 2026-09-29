# Networks: per-chain facts and porting notes

**onyx** (chain-id `onyx-1`) is the testnet gnomcp writes to by default, through the built-in
`testnet` profile. **mainnet** (`gnoland-1`) is read-only: reads and audits work, no code path signs
anything there. Every value below is as of **2026-09-29**; re-query anything load-bearing
(`gno_status`, `auth/gasprice`, a realm's render, whether a package resolves) before relying on it.

**Run `gno_status` before quoting anything here.** This file covers exactly two chain-ids,
`onyx-1` and `gnoland-1`. A local gnodev, an e2e simnet, a staging chain and any newer testnet are
neither, and nothing below describes them: their gas price, faucet policy, package set and gate
states are their own. Confirm the chain-id you are actually connected to first; if it is neither,
read the value off that chain instead of this page, and say which chain your answer is about.

**This file describes genesis sets only.** A chain's genesis is fixed at launch and safe to write
down; everything deployed after it belongs to whoever deployed it and changes without notice, so no
package count, no namespace and no third-party package is recorded here. To find out what a chain
carries beyond genesis, ask it with `gno_packages`. Method and design live in the topic references;
this file is only the per-chain snapshot.

## Per-chain matrix

| Fact | onyx: default, writable | mainnet: read-only |
|---|---|---|
| chain-id | `onyx-1` | `gnoland-1` |
| gnomcp profile | `testnet` | `mainnet` |
| writes through gnomcp | deploys (parked until approved), calls, sessions, faucet; **no `MsgRun`** | **none**, read tools only |
| RPC | `rpc.onyx.testnets.gno.land:443` | `rpc.gno.land:443` |
| gnoweb | `onyx.testnets.gno.land` | `gno.land` |
| release (`/status` `build_version`) | `v1.5.0`, commit `e75fef82c`: the release mainnet runs | `heads/chain/mainnet.3444+e75fef82c` |
| upgrade ledger | `misc/deployments/onyx.gno.land/upgrades.json` | `misc/deployments/mainnet.gno.land/upgrades.json` |
| gas price (`auth/gasprice`) | `1ugnot/1000gas` | same |
| block max gas | 3,000,000,000 | same |
| storage deposit (`params/vm:p:storage_price`) | 100 ugnot | same |
| deposit cap (`params/vm:p:default_deposit`) | 100 GNOT | same |
| code submission (`params/vm:p:code_submission_policy`) | **`inert`**, approved automatically | **`inert`** |
| `MsgRun` (`params/vm:p:run_submitters`) | **allowlisted** | **allowlisted** |
| CLA deploy gate (`r/sys/cla`) | **OFF** | **OFF** |
| namespace gate (`r/sys/names.IsEnabled`) | `true`; personal-address path free | `true` |
| name registration | `r/sys/namereg/v0`, not paused | `r/sys/namereg/v0` |
| validator set | `r/sys/validators/v0` holds it (`/v2` also deployed, empty) | same |
| faucet | present: read the grant size and per-address cap from `gno_status`'s `faucet` block, never from this row | none: mainnet ships without one |
| tx indexer | `indexer.onyx.testnets.gno.land/graphql/query` | `indexer.gno.land/graphql/query`; same query root, `getSupply` on both |
| toolchain install ref | tag `v1.5.0` | the ledger's newest entry; `chain/mainnet` (`9c8eb132`, semver twin `v1.2.0`) names genesis only |
| sub-package path scheme | version at the **leaf** (`p/nt/avl/rotree/v0`) | same |

The minimum fee for a 10M-gas write is 10,000 ugnot (0.01 GNOT) on both chains; gnomcp offers ×2
over the floor (`gnokey.md`). Mainnet runs the same gas price, so a write there would cost the same;
gnomcp simply never signs one.

Toolchain tags use the short chain **name**, never the chain-id (`chain/onyx`, not `chain/onyx-1`;
`chain/mainnet`, not `chain/gnoland-1`). Globbing a chain-id finds nothing. A tag can also lag its
branch: `heads/chain/mainnet` carries commits pushed after launch, and onyx has no branch at all.
Neither tag is automatically the ref to install: read the chain's `upgrades.json` first and fall
back to the tag only when it has no entry (`toolchain.md`). `chain/onyx` tags the commit that added
onyx's deployment config, not the binary it runs: the ledger names `v1.5.0`, and that tag is the
install ref.

**A chain tag names a launch commit, not what the chain runs today.** `chain/mainnet` is mainnet's
`v1.2.0` genesis; the chain has since been halted and restarted on newer binaries by governance.

**Two places say what a chain runs, and they agree.** `/status` carries a `build_version` naming
the running binary: a bare release tag on onyx, a branch with commit depth and short sha on
mainnet. On both it resolves to the same commit as the newest entry in the chain's
`upgrades.json`, the ledger of those restarts. Ask the node for what it is running and the ledger
for what was agreed; a disagreement is worth chasing rather than averaging. Do not read
`node_info.version` for this. It is `v1.0.0-rc.0` with an empty `software` on both chains and
identifies nothing.

## onyx: `onyx-1`

onyx runs mainnet's release line and takes each mainnet upgrade first, as its rehearsal. Today both
run commit `e75fef82c`, so code that compiles and runs on onyx compiles and runs on mainnet. That
holds only while both `build_version`s resolve to the same commit. The strings never match: onyx
reports the tag `v1.5.0`, which peels to `e75fef82c`, and mainnet reports
`heads/chain/mainnet.<depth>+e75fef82c`. Compare the commits. During an upgrade onyx runs the new
release before mainnet does, and the two differ until mainnet follows.

Its genesis deploys mainnet's genesis package set. Of the packages these references build on, three
differ from mainnet's copies, only in doc comments: GRC20, GRC721 and `r/sys/params`. It is a fresh
chain: no mainnet balances carried over, and funds come from its faucet.

**onyx allowlists `MsgRun`.** `gno_run` refuses any caller the list does not name (the agent key,
or a session's master) with `run_not_allowed` before signing anything. Without gnomcp the chain
answers `… is not authorized to send MsgRun; see the vm run_submitters param`. Deploy the logic as a
realm and call it, or run the script on a local gnodev.

## Inert code submission: onyx and mainnet

`params/vm:p:code_submission_policy` reads `"inert"` on both chains. Anyone may submit
(`params/vm:p:code_submitters` is unset), but the chain **parks** the package instead of running it:
no typecheck, no `init()`, stored in a key space of its own. An address in
`params/vm:p:pkg_approvers` then sends `MsgEnablePackage`, which typechecks the source and runs
`init()` on *its* transaction and gas, with the submitter as `OriginCaller` and paying the storage
deposit. Only then does the package exist. The submitter and an approver can each abandon a parked
submission with `MsgRejectPackage`; nothing expires one. The monorepo specifies all of this in
`gno.land/adr/pr5888_phase2_inert_packages.md`, and the `MsgRun` gate and the deposit charged at
enable in `gno.land/adr/pr6088_msgrun_allowlist_and_inert_charging.md`.

**onyx's approver is automatic.** It is an oracle (`contribs/gpao` in the monorepo) that watches
each block, type-checks and preprocesses every submission (the same checks `MsgEnablePackage`
re-runs) and enables the ones that pass. A good package goes live a few seconds after its deploy
commits. A package that fails those checks is never enabled: it stays parked indefinitely, and the
chain reports it exactly like one still waiting for approval.

**Simulation does not type-check on an inert chain.** Parking skips the type check, so a dry run of
an ill-typed package reports success. Lint the package locally against the chain's release
(`toolchain.md`) before deploying; the chain will not tell you.

**A parked path accepts a resubmission from its submitter.** Fixing the code and deploying it to
the same path with the same key replaces what is parked, and the approver looks at it afresh.
Another address is refused (`package already awaiting approval at …, submitted by …`).

Packages deployed to mainnet after genesis run there once approved. "Submitted" and "live" are
different states with a gap between them.

**A parked package is invisible to every ordinary read.** `vm/qpaths` skips it; `vm/qfuncs`,
`vm/qeval`, `vm/qrender` and `vm/qfile` answer it exactly as they answer a path that was never
submitted. Its source cannot be read back at all; only the submitting transaction carries it. Two
queries exist for this and nothing else:

```bash
gnokey query vm/qpkgmeta_json -data "gno.land/r/x/y"    # status: "live" | "inert" | "absent"
gnokey query "vm/qinertpaths?limit=100" -data "gno.land/r/"   # everything awaiting approval
```

`qpkgmeta_json` is the only way to tell a parked package from one that does not exist. It also
carries a `reason`, which reads the same for a package still waiting and for one the approver
refused. Reach for it before reporting that a path is missing on a chain running `inert`. Never
probe a path's existence with a call: a call into a parked path and a call into a path never
submitted fail with the same internal error (`unexpected node with location` in the log), so only
`qpkgmeta_json` answers the question.

## Mainnet: `gnoland-1`

Mainnet is a fresh chain whose balances come from an audited allocation rather than faucets, and
gnomcp treats it as read-only: no agent key, no session, no faucet, and both `master-address` and
the faucet fields refused at config time. A forced write stops at the keystore, which has no key to
give for a read-only chain-id. Reads and audits are the whole surface, which is what auditing
deployed code needs. It ships as the built-in `mainnet` profile, so reading it needs no config.

**`MsgRun` is allowlisted.** `params/vm:p:run_submitters` carries a non-empty address list on both
chains. `MsgRun` executes arbitrary source immediately under *every* policy, including `inert`,
which is why it gets its own gate. Query the param for the current set.

**No transfer restriction is active.** `params/bank:p:restricted_denoms` reads empty on both
chains. Read it rather than inferring a lock from launch tooling.

**`gnoland-1` and `gnoland1` are one hyphen apart and are different chains.** `gnoland-1` is
mainnet; `gnoland1` was betanet, whose hosts are gone. Neither is writable, so confusing them
cannot produce a write: a `gnoland1` profile fails to connect at all.

**`gno.land` names mainnet.** It served betanet before mainnet launched, so a profile pinned to
that domain changed chains under a name that said otherwise. Read a chain-id rather than inferring
one from a hostname, and pin a chain's own hostnames when one exists. The same trap reaches the
toolchain: the dependency fetcher derives its remote from the **import path's domain**, so a
`gno.land/...` import resolves to `rpc.gno.land` (mainnet) whichever chain you are building for
(`toolchain.md`).

## Retired chains

**pearl (`pearl-1`), betanet (`gnoland1`), sapphire (`sapphire-1`) and topaz (`topaz-1`) are
gone.** Their RPC, gnoweb and indexer hostnames do not resolve, so gnomcp ships no builtin for any
of them, nothing archived on them is readable from the chain, and none of their chain-ids is
writable. topaz's tag `chain/topaz` (`fc40526`) sits behind `heads/chain/topaz` (`63c2673`), which
is why a tag is worth comparing against its branch head before pinning.

**A host that answers does not prove a chain is live.** betanet spent its last stretch resolving,
replying to every read, and serving a height that never moved: a retirement no host check catches.
Sample the height from `gno_status` twice before treating a chain as current.

Earlier numbered testnets (`test1`–`test13`) are likewise dead. gnomcp still treats any `test*`
chain-id as writable, so a local chain named that way takes writes.

## Porting code written for pearl

pearl, onyx's predecessor, ran an older release line with a different genesis set. Code written
and tested against it, including examples and tutorials from its time, can fail on onyx and
mainnet in these places:

- **A crossing `cur` is a fixed binding.** Reassigning it, taking `&cur`, or range-assigning to it
  compiled on pearl and fails at preprocess on onyx and mainnet, each with its own message:
  ``cannot reassign the crossing `cur` parameter``,
  `` cannot take the address of a realm-typed `cur` ``,
  ``cannot assign to a realm-typed `cur` in a range clause``.
  On onyx such a package parks and is never enabled.
- **`AssertOriginCall()` reached through a re-export panics.** Where one realm exposes another's
  crossing function as `var Deposit = other.Deposit`, calling the alias passed on pearl and panics
  on onyx and mainnet with `invalid non-origin call`, because the origin call is anchored to the
  entry package. Calling the original directly succeeds.
- **The version segment sits at the leaf of a sub-package.** Top-level packages keep their
  spelling (`p/nt/avl/v0`, `p/nt/mux/v0`); below the root, pearl hung sub-packages under the
  version:

  ```go
  gno.land/p/nt/avl/v0/rotree                  // pearl
  gno.land/p/nt/avl/rotree/v0                  // onyx, mainnet
  gno.land/p/nt/ownable/v0/exts/authorizable   // pearl
  gno.land/p/nt/ownable/exts/authorizable/v0   // onyx, mainnet
  ```

- **The GRC20 standard moved.** pearl carried it at `p/demo/tokens/grc20`; onyx and mainnet have no
  `p/demo` tree and carry it at `p/nt/grc20/v0`. Their `TransferFrom` also rejects `owner == to`
  with `ErrCannotTransferToSelf`, which pearl's did not.
- **Personal-namespace packages gained a `/v0` leaf** (`p/moul/realmpath`, `p/moul/authz`,
  `p/jeronimoalbi/pager`), and the system realms sit at `/v0`: `r/sys/namereg/v0` (pearl:
  `/v1`), `r/sys/validators/v0` (pearl: `/v3`), `p/sys/validators/v0` (pearl: no version),
  `r/gnoland/boards2/v0` (pearl: `/v1`).

Resolve every import against the target chain with `gno_packages`, and lint against the chain's own
release (`toolchain.md`) before deploying. No local `gno test` against another release shows these
differences.

**Measure a suspected difference; never infer one from a changelog.** Whether an upstream PR is an
ancestor of a chain's build says nothing about whether the behaviour is present: a release that
rewrites a rule's message or mechanism looks like the rule arriving. Install both releases
(`toolchain.md`) and lint the same file against each.

## GRC20 tellers and the NFT standard

**`CallerTeller()` hangs off `*PrivateLedger` on both chains.**

```go
teller := ledger.CallerTeller()  // func (ledger *PrivateLedger) CallerTeller() Teller
```

Both also carry a `guardHome` check that confines a frame-relative teller to the token's own realm,
so a teller a realm builds and then exports is inert elsewhere. An older GRC20 hung the same method
off `*Token` with no guard; `security.md` reads what that receiver decides, and a fork can still
carry it.

The NFT standard is in the genesis set (under the `p/nt` tree with a `/v0` leaf), while
`p/nt/commondao/v0` resolves on neither chain. `p/demo/tokens/grc721` resolves nowhere. Read the
**target chain's** deployed source (`gno_read` / `vm/qfile`) before relying on any package this
file does not cover.

## Deploying: onyx by default

1. **Confirm the target** with `gno_status` (chain-id) or `gno_profile_list` (name ↔ chain-id
   map). A read-only chain has no deploy path at all, so a deploy that "should" go to gno.land is
   an onyx deploy or nothing. Mainnet would park the package even if one reached it.
2. **Gates.** The personal-address path is free (namespace gate on, address paths always allowed).
   CLA enforcement is off on both chains today, so no `Sign` step is needed, but it is a chain
   setting: confirm with `gno_cla_info` rather than trusting this line.
3. **Name.** The package name must equal the path's last element, or the element before a `/vN`
   suffix. onyx rejects a mismatch at submit with `invalid package path`.
4. **Fund** with `gno_faucet_fund`. The grant size and the per-address cooldown are operator
   settings: `gno_status` carries them in its `faucet` block, and the refusal message names the cap
   it hit. A capped address is not a broken faucet; a fresh key has its own allowance.
5. **Fees.** `1ugnot/1000gas`; still query `auth/gasprice`, since this is the value most likely to
   drift next.
6. **Imports.** Resolve every import against the target chain with `gno_packages`, never against
   master or this file. Code written for pearl spells several imports differently (§ Porting code
   written for pearl).
7. **Parking.** Lint against onyx's release before deploying, since the chain does not type-check a
   submission. After the deploy, call the package only once it is live: `gno_addpkg` waits for the
   approver and reports `package_status`, and a read of a parked path answers `package_parked`.
   `live` is the only status that means callable. `inert` means parked: a package still parked a
   minute after its deploy is not going to be enabled on its own, so lint it, check the deploying
   key can pay the storage deposit, then redeploy to the same path with the same key.
   `redeploy_parked` means the previous version keeps serving reads and calls until the new one is
   enabled. `unknown` means the chain gave no usable answer: read the path before calling it.
8. **Scripts.** On onyx `gno_run` returns `run_not_allowed`: deploy the logic as a realm and
   `gno_call` it, or run the script on a local gnodev.
9. **Transaction history.** `gno_activity`/`gno_history` work on both chains. To enumerate what is
   deployed, `gno_packages` reads the chain directly and needs no indexer.
10. **Local tests.** Use the chain-matched toolchain and vendor on-chain deps from the matching
    source tree; a develop-HEAD toolchain can refuse to compile deps auto-fetched from a chain, and
    the fetcher's default remote is mainnet's whatever you are targeting (`toolchain.md`).
