---
name: gno
description: Use when reading, writing, auditing, or reviewing Gno realm code (`/r/` realms, `/p/` packages, `gno.land/...` import paths), discussing interrealm semantics (`cross`, `cur realm`, `PreviousRealm`, crossing functions), working with on-chain payments (`OriginSend`, `IsUserCall`, `SentCoins`), authoring or evaluating realm `Render(path string) string` output for gnoweb (markdown alerts, `<gno-form exec>`, image validation), or assessing the safety of a Gno smart contract before deployment or interaction.
---

# Gno

> **Status: SCAFFOLD.** This skill is in draft. Body is a router pointing to references; references are stubs that will be filled from `.mynote/gno-agentic/reference/{15,16,17}-*.md` corpus. Do not deploy until RED-GREEN-REFACTOR cycle has been run against pressure scenarios.

## Overview

Gno is an interpreted, stack-based Go-derived VM. Realms (`/r/`) are stateful smart contracts; packages (`/p/`) are stateless reusable libraries. Source lives on-chain — every realm is both executable code and human-readable content. The chain is in **betanet** as of 2026: the interrealm specification is the *youngest* and most actively-changing part of the stack, and many deployed realms predate the current security model.

**Core principle:** Pattern-matching from Solidity, Cosmos, or vanilla Go will get you wrong answers on interrealm semantics. Gno's model is its own — implicit storage-realm borrow, readonly taint on cross-realm references, explicit `cur realm` parameter for crossing functions, and "receiver attachment is a privilege grant" as a deliberate design choice rather than a fixable bug.

## When this skill applies

- Reading or writing `.gno` files
- Reviewing a realm before sending coins to it, interacting with it, or deploying it
- Questions about `cross`, `crossing function`, `cur realm`, `PreviousRealm`, `CurrentRealm`, `MsgCall` vs `MsgRun`, `OriginSend`, `IsUserCall`
- Debugging "readonly" taint panics, cross-realm pointer issues, or unexpected `PreviousRealm()` values
- Designing a `/p/` package that accepts caller-supplied interfaces, callbacks, or closures
- Designing a `/r/` realm that accepts payment

## How to use this skill

This SKILL.md is a router. References are categorized by topic for maintenance, not by task — load whichever fits the question. Cross-reference between them; they share vocabulary.

| Topic | Reference | Load when… |
|---|---|---|
| Spec / model | `references/interrealm.md` | Reasoning about `cross`, `cur realm`, `PreviousRealm`, readonly taint, crossing functions, `MsgCall` vs `MsgRun`. |
| Security / audit | `references/security.md` | Reviewing a realm; looking up a bug class; checking payment guards. |
| Audit procedure | `references/audit.md` | Conducting a security audit (3-phase procedure, evidence-gating rule, two-pass FP filter, severity calibration, structured output format). Loaded when the user explicitly asks to audit. |
| Patterns / idioms | `references/patterns.md` | Writing new realms; choosing `/r/` vs `/p/`, state shape, AVL, testing. |
| Stdlib API surface | `references/stdlib.md` | Calling `std.*`, `banker.*`, `avl.*`, `crypto.*`. |
| Rendering | `references/render.md` | Writing or auditing `Render(path string) string`; markdown extensions (alerts, columns, forms, imgvalidator, link, mentions); gnoweb output; `vm/qrender`. |
| In-flight / unmerged | `references/future.md` | Anything not in master yet (PR #5669 `cross2(rlm)`, `SentCoins` adoption, daokit upgrade, `<gno-form exec>` wire format, etc.). Always consult before emitting code that depends on a pattern's status. |

**Task hints** (multi-reference loads):
- *Auditing a realm interactively* → `audit.md` is the procedure; it pulls in `security.md` + `interrealm.md` + `patterns.md` + `render.md` (if relevant) + `future.md` as it runs.
- *Auditing under gates (read-only, structured report, FP-filtered)* → dispatch the `gno-auditor` sub-agent at `agents/auditor.md`. It wraps `audit.md` with a tool allowlist and a two-pass Task-tool dispatch for false-positive filtering.
- *Writing a realm* → `patterns.md` + `stdlib.md` + `render.md`, then `security.md` to avoid known footguns, then `future.md` to avoid generating code that won't compile.
- *Answering "what does X actually do?"* → `interrealm.md` for spec questions; `stdlib.md` for API questions; `render.md` for gnoweb markdown behavior.

## Quick reference

| Symbol / phrase | Meaning |
|---|---|
| `func F(cur realm, ...)` | Crossing function — caller must invoke with `F(cross, ...)`. Inside the function, `cur` is the current realm context. |
| `cross` (keyword) | Bare cross-call marker. Mints a fresh `cur` whose `prev` points to the caller's frame. Implicit target = current realm. |
| `cross2(rlm)` | **Not in master at 2026-05-15 — introduced by PR #5669 (WIP).** Explicit form: `cross2(cur)` ≡ bare `cross`; `cross2(otherRealm)` names an explicit target realm. |
| `runtime.PreviousRealm()` | Returns the previous realm in the call stack. Only shifts on **explicit cross-calls into crossing functions**. |
| `IsUser()` | True if caller is an EOA OR an ephemeral `MsgRun` realm. **Accepts `maketx run` — can bypass payment guards.** |
| `IsUserCall()` | True only if caller is an EOA via `MsgCall`. Use this, not `IsUser()`, when guarding `OriginSend`. |
| `banker.OriginSend()` | Coins included with the originating transaction. Today's canonical inbound-payment primitive. |
| `realm.SentCoins()` | PR #5039 (merged Apr 2026). Frame-local — coins sent into the current frame only. Re-entrancy-safe successor to `OriginSend()`. Not yet adopted across `examples/`. |
| Readonly taint | When code takes a reference (pointer / slice element) from a real object persisted in an **external** realm, the reference is readonly. Mutating it panics at runtime. |

## Red flags to grep for (auditor mode)

These show up across the bug-class catalog (see `references/security.md` for the why-and-how):

- `runtime.PreviousRealm()` inside a **non-crossing** function used as a caller-identity check
- `banker.OriginSend()` paired with `IsUser()` (should be `IsUserCall()`)
- Caller-supplied `interface{}` stored in realm state and later invoked
- `ExecFunc`, `WithExecutor`, `WithValidation`, `WithTally` — caller-provided callback patterns
- Methods on receivers persisted into another realm's state without explicit authority gating
- `MsgRun` consuming `OriginSend` envelope before calling the target realm
- Functions accepting `crossing()` body marker instead of `(cur realm, ...)` parameter — pre-Gno-0.9, stale spec

## Anti-pattern reflex

If you find yourself thinking "this is just like Solidity's `msg.sender`…" — **stop**. Gno's `PreviousRealm()` only shifts on explicit `cross`. A `PreviousRealm().PkgPath() == "..."` check inside a *non-crossing* function does NOT identify the immediate caller. It returns the realm two frames up the implicit-borrow chain. Read `references/interrealm.md` before continuing.

## Known limits of this skill

- **The interrealm spec is the youngest part of the chain.** Active migrations live in `references/future.md`; always consult it before emitting code that depends on a pattern's status.
- **No compiler protection for attached-method privilege escalation.** Receiver attachment is a privilege grant — the audit is the only line of defense. See `references/security.md`.
- **Many deployed examples predate the current spec.** Pre-Gno-0.9 code using `crossing()` body markers compiles via the transpiler but is not a safe pattern to copy.

## Source corpus

This skill was distilled from internal research in `.mynote/gno-agentic/reference/` (15 / 16 / 17). Those documents are working notes for the initial scaffold; ongoing maintenance happens against the categorical references directly, not the source docs.
