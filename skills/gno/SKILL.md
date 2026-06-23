---
name: gno
description: Use when reasoning about, reading, or reviewing existing Gno realm code (`/r/` realms, `/p/` pure packages, `gno.land/...` import paths), discussing interrealm semantics (`cross(cur)`, `cur realm`, `PreviousRealm`, crossing functions), on-chain payments (`OriginSend`, `IsUserCall`, `cur.Previous().IsUserCall()`), evaluating realm `Render(path string) string` output for gnoweb, or the Gno memory and data model. Do NOT use this skill directly for tasks owned by its siblings — authoring/building/testing/deploying realms or scaffolding a Gno project goes to gno-build, explicit security audits to gno-audit, failed transactions/calls to gno-debug, first-contact "what is gno" teaching to gno-onboard; those skills load this one's references themselves.
---

# Gno

This skill helps you write, test, audit, and reason about Gno code — the language and on-chain runtime that powers gno.land.

## Overview

Gno is an interpreted Go-derived VM where source code lives on-chain. Every code unit is a **package**; the prefix letter is the kind:

| Prefix | Kind | Stateful? | Use for |
|---|---|---|---|
| `/r/` | Realm | yes (persistent) | Smart contracts. Public crossing functions, on-chain state. |
| `/p/` | Pure | post-init frozen | Reusable libraries. Stateless logic, types, interfaces. |
| `/e/` | Ephemeral | per-tx, discarded | `MsgRun` invocations. Created on-the-fly by the chain. |

Two callers reach realms: an **EOA** (externally-owned account) via `MsgCall` or `MsgRun`, or another **realm** via a cross-call. Identity, authority, and storage attribution all hinge on which.

**Core principle.** Pattern-matching from Solidity, Cosmos, or vanilla Go produces wrong answers here. Gno has its own model: **`cur realm` is a capability token**, the storage realm of an object is its allocator (PkgID at allocation = authority), cross-realm references carry a sticky **readonly taint**, and "receiver attachment is a privilege grant" is a deliberate design choice rather than a fixable bug. Always check `cur.IsCurrent()` before using `cur.Previous()`.

The chain is still **testnet**; the interrealm spec is the youngest and most actively-changing part of the stack. Treat security-critical patterns as version-bound to master HEAD and verify against upstream when emitting consequential code.

## When this skill applies

- Reading or writing `.gno` files
- Designing a realm: state shape, public API, payment handling, access control
- Designing a `/p/` package that accepts caller-supplied interfaces, callbacks, or closures
- Setting up a project: `gnomod.toml`, filetests, `gno` binary subcommands
- Questions about interrealm primitives: `cross(cur)`, `cur realm`, `PreviousRealm`, `CurrentRealm`, `MsgCall` vs `MsgRun`, `OriginSend`, `IsUserCall`
- Debugging readonly-taint panics, cross-realm pointer issues, unexpected `PreviousRealm()` values, persistence surprises
- Reviewing a realm before sending coins to it, interacting with it, or deploying it
- Conducting a formal security audit

## How to use this skill

This SKILL.md is a router. References are categorized by topic; load whichever fits the question. They share vocabulary and cross-reference each other freely.

| Topic | Reference | Load when… |
|---|---|---|
| Spec / model | `references/interrealm.md` | Reasoning about `cross(cur)`, the `cur` capability token, `PreviousRealm`, `IsCurrent`, the three borrow rules, readonly taint, conversion guards, `MsgCall` vs `MsgRun`, realm boundaries, finalization. The foundation for every other reference. |
| Security / audit catalog | `references/security.md` | Reviewing a realm; looking up a bug class (Class 1a/1b/2/3/4); checking payment guards; consulting the (A)/(B)/(C) safety hypothesis; using the verification checklist or the encapsulation pattern. |
| Audit procedure | `references/audit.md` | Conducting a formal security audit (3-phase procedure, evidence-gating, two-pass false-positive filter, severity calibration, structured output format). Loaded when the user explicitly asks to audit. |
| Patterns / idioms | `references/patterns.md` | Writing new realms — counter-intuitive idioms (globals, panic, init), package organization, state shape, crossing-function discipline, payment handling, events, access control, cost-aware design. The "do this" companion to security.md's "don't do this". |
| Stdlib API surface | `references/stdlib.md` | Calling `chain.*`, `chain/runtime.*`, `chain/runtime/unsafe.*`, `chain/banker.*`, `chain/markdown.*`, `testing.*`; the uverse builtins (`address`, `realm`, `cross(rlm)`, `revive`); the predicate trichotomy (`IsUserCall` vs `IsUserRun` vs `IsUser`); kept community packages. |
| Rendering | `references/render.md` | Writing or auditing `Render(path string) string`; markdown extensions (alerts, columns, forms, imgvalidator, link, mentions); gnoweb output; `vm/qrender` query surface; XSS / untrusted-content posture. |
| VM model | `references/memory.md` | Understanding what persists across transactions; typed values; heap items; closure captures; loop-variable semantics; pointer base/index model; data-structure choice (array vs slice vs map vs avl). Auditor: load when reasoning about object identity, aliasing, or "does this get persisted". |
| Build, test, configure | `references/build.md` | Scaffolding a project; `gnomod.toml` and `gnowork.toml` fields; the `gno` binary; writing realm tests (the `cur realm` test signature, faking caller/coins, the `std`→`chain`/`testing` migration, resolving `/p/` test dependencies); the three test flavors (unit, filetest, integration); filetest directives; Go-Gno compatibility surface; deploy mechanics. |
| Chain-matched toolchain | `references/toolchain.md` | Testing or linting local code against a live chain target; `gno` missing from `PATH` or its version doesn't match the target; fetching on-chain deps for local tests; the per-release binary store. |
| MCP acceleration | `references/mcp.md` | A Gno MCP server (e.g. gnomcp) is connected and you want to fetch on-chain package source or discover packages. Optional — the skill works without it; load to see when to reach for it. |
| Platform / sys realms | `references/sysrealms.md` | Reasoning about gno.land system realms (`r/sys/*`: names/users/namereg, validators, params, cla), namespace/name registration, the validator set, genesis-predeployed realms, GovDAO param governance, or why a sys realm differs across networks (test13 vs local vs betanet). Teaches the design; sends you to the live chain for concrete values. |
| CLI / tx machinery | `references/gnokey.md` | Understanding what a write costs or why it failed — the `gnokey` CLI and the tx model behind every write: `--gas-wanted`/`--gas-fee` (the full fee is deducted, not gas-used), the price floor, out-of-gas vs insufficient-fee, storage deposit (`--max-deposit`), simulate-to-size-gas, account/sequence/chainid sig errors. Maps each `gnokey maketx` command to its gnomcp tool, and shows the equivalent command for a write. The "why gnomcp signs for you, don't shell out to gnokey" discipline. |

### Task hints (multi-reference loads)

- *Writing a realm from scratch* → start with `patterns.md` (idioms) + `interrealm.md` (spec), then `stdlib.md` for API, `security.md` to avoid known footguns, `render.md` if it exposes a `Render()`, `memory.md` for state-shape decisions, `build.md` for testing and shipping.
- *Designing the public API of a realm* → `interrealm.md` (crossing functions, `cur.IsCurrent()`) + `security.md` (the verification checklist + (A)/(B)/(C) safety hypothesis) + `patterns.md` (realm-as-public-API mindset).
- *Debugging a panic* → `interrealm.md` (readonly taint, conversion guards, cross-realm panic semantics) + `memory.md` (heap-item and pointer model) + `security.md` (the operational signals — non-determinism, platform divergence).
- *Reviewing someone else's realm casually* → `security.md` + `interrealm.md` are always relevant; `patterns.md` for idiom checks; `render.md` if it has a `Render()`. Skip the formal procedure unless asked.
- *Auditing a realm formally* → `audit.md` is the procedure; it pulls in `security.md` + `interrealm.md` + `patterns.md` + `render.md` (if relevant) + `memory.md` (if persistence semantics are in play) as it runs.
- *Auditing under gates (read-only, structured report, FP-filtered)* → dispatch the `gno-auditor` agent **by name** via the Task tool (it's a registered agent, defined at the plugin's top-level `agents/auditor.md`). It wraps `audit.md` with a read-only tool allowlist and a two-pass Task-tool dispatch for false-positive filtering.
- *Setting up a new project* → `build.md` (scaffold, gnomod.toml, testing flavors) + `patterns.md` (package organization, `/r/` vs `/p/` vs `/e/`).
- *Answering "what does X actually do?"* → `interrealm.md` for spec questions; `stdlib.md` for API questions; `render.md` for gnoweb markdown behavior; `memory.md` for persistence behavior; `build.md` for tooling behavior.
- *Answering "how do I register a name / who are the validators / is enforcement on?" for a specific chain* → `sysrealms.md` (the sys-realm model + the trust-the-chain query discipline) + `mcp.md` (the query tools). The answer is a live query, not a recital.
- *Answering "what does this write cost / why did my tx fail with gas/fee/sequence?" or "what's the gnokey equivalent?"* → `gnokey.md` (the `Fee = {GasWanted, GasFee}` model, the price floor, the failure-mode table, the gnomcp↔gnokey mapping) + `debug.md` for triaging a specific failed call.

## Quick reference

| Symbol / phrase | Meaning |
|---|---|
| `func F(cur realm, ...)` | **Crossing function** — caller invokes with `F(cross(cur), ...)` for a cross-call, `F(cur, ...)` for a same-realm non-crossing call. Only `/r/` packages may declare crossing functions. `MsgCall` only dispatches to crossing functions. |
| `cross(rlm)` | Uverse function marking a cross-call. `cross(cur)` is the canonical form; `cross(otherRealm)` names an explicit target. The bare `cross` keyword that existed during the 0.9 migration is gone. |
| `cur.IsCurrent()` | **The authentication primitive.** Returns true only for the live crossing frame's `cur`. Check this before using `cur.Previous()`, `cur.Address()`, or `cur.PkgPath()` — otherwise a stale stored realm value still resolves numerically and forges identity. (security.md Class 2 designation-forgery.) |
| `cur.Previous()` | Captured realm that was current before this crossing. Use `cur.Previous().Address()` to identify the caller after the `IsCurrent` check. |
| `runtime.PreviousRealm()` | Imported from `chain/runtime/unsafe`. **Legacy stack-walker** — returns the realm prior to the most recent boundary regardless of which function you're in. Use `cur` in new code; reach for this only when you need stack-walking semantics. |
| `IsUserCall()` | True only if caller is an EOA via `MsgCall`. Use this when guarding `OriginSend`. |
| `IsUserRun()` | True only if caller is an EOA via `MsgRun` (in the ephemeral `/e/g1<user>/run` realm). |
| `IsUser()` | True if `IsUserCall()` OR `IsUserRun()`. **Insufficient for payment guards** — the `MsgRun` ephemeral can consume the `OriginSend` envelope before forwarding control. |
| `banker.OriginSend()` | Coins included with the originating transaction. Pair with `cur.Previous().IsUserCall()` and an amount check. |
| Readonly taint | When code reads a value across a realm-storage boundary, the value is tainted read-only. The taint is sticky and propagates through field access, indexing, slicing, copies, and conversions. Mutation panics. |
| Borrow rules | Three implicit rules that set `m.Realm` per call: declaring-realm (#1, `/r/`-callables), storage-realm (#2, `/p/`/stdlib methods on real foreign-stamped receivers), closure-capability (#3, `/p/`-declared closures carry creator-realm authority). |
| PkgID | Stamped on every object at allocation time. **Storage = Authority** — the realm that allocated an object owns it. |
| `panic` | Use for unrecoverable contract state — aborts the transaction cleanly, reverts all state. `error` does NOT revert state. |

## Security patterns quick check

Before writing or reviewing crossing functions, check these common mistakes:

| Trigger | What to look for | Fix |
|---|---|---|
| **Authorization check** | `cur.Previous()` without `cur.IsCurrent()` | Check `cur.IsCurrent()` first |
| **Payment handling** | `OriginSend` without `cur.Previous().IsUserCall()` | Guard with `.IsUserCall()`, not `.IsUser()` |
| **Caller identity** | Using `OriginCaller()` in crossing function | Use `cur.Previous().Address()` after `IsCurrent()` |
| **Render output** | Raw `path` concatenated into markdown | Sanitize with `sanitize.InlineText` (`gno.land/p/nt/markdown/sanitize/v0`); see `render.md` |
| **Callback params** | Function accepts `func()` from caller | Replace with explicit state-mutation methods |
| **Interface methods** | Interface signature contains `cur realm` | Take `address` instead; caller derives from `cur.Previous()` |
| **Pointer getters** | Exported function returns `*privateState` | Return values, not pointers |
| **Render storage** | `Render` iterates over `map` | Use `avl.Tree` with `.Iterate()` for deterministic order |

Load `security.md` for detailed threat model and exploitation mechanics.

## Anti-pattern reflex

If you find yourself thinking *"this is just like Solidity's `msg.sender`…"* — **stop**. Gno's caller-identity model is not stack-walking by default. Specifically:

- `cur.Previous()` is only meaningful inside a **crossing function** AND only after `cur.IsCurrent()` returns true.
- `runtime.PreviousRealm()` (from `chain/runtime/unsafe`) is a stack-walker — inside a non-crossing function it returns whatever was previous at the last realm boundary, possibly an unrelated frame upstream. It does NOT identify the immediate caller.
- A `PreviousRealm().PkgPath() == "..."` check inside a non-crossing function is a **security bug** (security.md Class 2).

Load `interrealm.md` § The `cur` capability token before emitting any caller-identity check.

## Known limits of this skill

- **The interrealm spec is the youngest part of the chain.** Treat patterns as version-bound to the master HEAD this skill was distilled against; verify against upstream when emitting security-critical code.
- **No compiler protection for attached-method privilege escalation** (the (A)-class violation when a `/p/`-type with concrete-callback higher-order methods gets embedded in `/r/`-data). Receiver attachment is a privilege grant — the audit is the only line of defense. See `references/security.md` Safety hypothesis (B).
- **Test-13 quarantine snapshot.** The kept-in-`examples/` set was frozen at scaffold time; new community packages added or removed post-scaffold are not reflected. Verify against the current master tree before recommending an import.
- **gno.land docs are evolving.** When a recommendation here disagrees with `docs/resources/*.md` in the gnolang/gno repo, the upstream docs win — file an update.

## Source

Distilled from `docs/resources/` in the `gnolang/gno` repo: `gno-interrealm-v2.md`, `gno-security.md`, `gno-security-guide.md`, `effective-gno.md`, `gno-stdlibs.md`, `gno-memory-model.md`, `gno-data-structures.md`, `gno-testing.md`, `configuring-gno-projects.md`, `go-gno-compatibility.md`, plus observed conventions in `examples/gno.land/` (post-test-13 quarantine).
