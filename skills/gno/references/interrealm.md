# Interrealm semantics

> **Category: spec / model.** Update when interrealm spec changes land in master. In-flight changes belong in `future.md`.
> **Authoritative spec**: `docs/resources/gno-interrealm.md` (38K). This reference is a load-bearing summary, not a replacement. When an audit hinges on exact semantics, load the spec.

## Why this reference exists

Gno extends Go to a multi-user runtime. Realms are independent agents that own state and authority; calling between realms is the central design problem. The interrealm spec is the youngest part of the chain — most LLM training data predates it, and pattern-matching from Solidity or Cosmos produces wrong answers. Read this before generating *any* caller-authentication, access-control, or cross-realm code.

## The two contexts

All Gno logic runs under **two simultaneous contexts**:

| Context | Determines | Changes on… | Accessible via |
|---|---|---|---|
| **Realm-context** | identity / agency: who is the actor, who called them. Has an associated Gno address that can send / receive coins. | explicit cross-calls only (`fn(cross, ...)`) | `runtime.CurrentRealm()` / `runtime.PreviousRealm()` |
| **Realm-storage-context** | where new and modified objects persist during transaction finalization. No associated address. | explicit cross-calls AND implicit borrow-crosses (calling a non-crossing method on a real receiver in a different realm) | not directly accessible at runtime |

After an explicit cross-call, both contexts point at the same realm. They diverge only when a non-crossing method is called on a receiver that resides in a different realm — the storage context shifts to the receiver's realm while the realm-context stays put.

**This divergence is the root of bug class `security.md` §9 (attached-method privilege escalation).** It is by design.

## Package types

Three flavors. Memorize the distinctions; they determine what code can do where.

| Path | Name | Stateful? | Can declare crossing functions? | Can import `/r/`? |
|---|---|---|---|---|
| `/r/...` | Realm | yes (persistent state) | yes | yes |
| `/p/...` | Pure package | no | **no** | **no** (only `/p/` imports) |
| `/e/...` | Ephemeral | yes (per-tx, discarded) | n/a (created by `MsgRun`) | yes |

**Important constraints**:
- A `/p/` package's behavior must be identical regardless of which realm calls it. P-code copied into a realm should behave the same. If a /p/ package behaves differently across callers, it's smuggling state and that's a bug.
- A `/p/` package **cannot** import a `/r/` package. The constraint is structural; the linker enforces it.
- An `/e/` package is what `MsgRun` runs in. It's created on-the-fly with path `gno.land/e/g1<user>/run` and discarded after the transaction.

## Crossing functions

**Syntax**:

```go
// Receiver-side: declare a crossing function in a realm.
func MakeBread(cur realm, ingredients ...string) Bread { ... }

// Caller-side: cross into it.
import "gno.land/r/alice/bakery"
loaf := bakery.MakeBread(cross, "flour", "water")
```

Rules:
- `cur realm` **must** be the first parameter. Anywhere else is illegal.
- `cur realm` is illegal in `/p/` packages.
- Callers pass `cross` as the first argument to cross-call.
- Callers can pass `nil` instead of `cross` for a non-crossing call (same-realm call only — non-crossing call into an external realm is a type-check error or runtime error).

**Calling convention table** (from the spec):

| Call type | Realm-context changes? | Storage-context changes? | Boundary? | Finalizes? |
|---|---|---|---|---|
| `fn(cross, ...)` to same realm | yes* | no | yes | yes |
| `fn(cross, ...)` to different realm | yes | yes | yes | yes |
| `fn(nil, ...)` non-crossing call | no | no | no | no |
| Non-crossing method, receiver in same realm | no | no | no | no |
| **Non-crossing method, receiver in different realm** | **no** | **yes** | **yes** | **yes** |
| Non-crossing method, unreal receiver | no | no | no | no |
| Non-crossing function | no | no | no | no |

\* `CurrentRealm()` returns the same realm, but `PreviousRealm()` shifts — what was current becomes previous.

The bolded row is the implicit borrow-cross. It's the only place storage context shifts without an explicit cross.

## `PreviousRealm()` semantics

**The most pattern-matched-wrong primitive in Gno.** Anti-Solidity reflexes give wrong answers.

`runtime.PreviousRealm()`:
- Returns the realm immediately prior to the *most recent realm boundary*.
- Boundaries are created **only** by explicit cross-calls into crossing functions.
- A non-crossing function call does **not** create a boundary, so `PreviousRealm()` returns whatever the *caller's* `PreviousRealm()` was — not the immediate caller.

**Consequence**: a check like

```go
func F(args ...) {                          // non-crossing — no cur realm param
    if runtime.PreviousRealm().PkgPath() != "gno.land/r/trusted/admin" {
        panic("admin only")
    }
    // ...
}
```

does NOT verify the immediate caller. If a non-crossing function in some other realm calls `F(...)`, `PreviousRealm()` is still whatever was previous *before that other call* — possibly the admin realm two frames back. This is a security bug. Always perform caller-identity checks inside **crossing** functions (`func F(cur realm, ...)`).

See `security.md` §5 (cur disclosure) and §7 (MsgRun semantics) for related classes.

## `CurrentRealm()` and stage

`runtime.CurrentRealm()` returns the active realm-context. Its value depends on which **stage** the VM is in:

| Stage | Triggered by | `CurrentRealm()` is… | `PreviousRealm()` is… |
|---|---|---|---|
| `StageAdd` | `MsgAddPackage` (deploy) | the package being deployed (incl. `/p/` packages — "mutating for a moment") | the deploying user |
| `StageRun` via `MsgCall` | a user calling a crossing function | the called realm | the user with `PkgPath: ""` |
| `StageRun` via `MsgRun` | a user running an ephemeral realm | `gno.land/e/g1<user>/run` (ephemeral) | the user with `PkgPath: ""` |

**Therefore**: code in an `init()` block sees deploy-time context, not call-time context. Code that runs under both `MsgCall` and `MsgRun` must distinguish via `IsUserCall()` vs `IsUserRun()` (see `security.md` §7).

## Caller-identity predicates

`Realm` exposes three predicates that look interchangeable but aren't:

| Predicate | EOA via `MsgCall` | EOA via `MsgRun` (ephemeral realm) | Realm via `MsgCall` |
|---|---|---|---|
| `IsUserCall()` | ✓ | ✗ | ✗ |
| `IsUserRun()` | ✗ | ✓ | ✗ |
| `IsUser()` | ✓ | ✓ | ✗ |

The `MsgRun` ephemeral realm can consume `banker.OriginSend()` and forward control — so `IsUser()` (broad) is the wrong guard for payment-gated paths. Use `IsUserCall()`. See `security.md` §1, §7.

## Implicit borrow-cross on methods

When a non-crossing method is called on a real receiver that lives in another realm, the **storage-context** shifts to the receiver's realm. The **realm-context** does NOT shift. The method's body executes with:
- the caller's identity (`CurrentRealm()` unchanged), but
- the receiver realm's persistence authority (new and modified objects persist into the receiver's realm).

**This is the trust grant**: whoever stores an object in their realm grants execution privileges to the methods declared on that object's type. If the type comes from an `r/` import authored by a third party, methods on that type can mutate the storing realm's state.

The spec is explicit about this:

> "The interrealm specification does not secure applications against arbitrary code execution. It is important for realm logic (and even p package logic) to ensure that arbitrary (variable) functions (and similarly arbitrary interface methods) are not provided by malicious callers; such arbitrary functions and methods whether crossing (or non-crossing) will inherit the previous realm (or both current and previous realms) and could abuse these realm-contexts."

The compiler does not flag this. Audit is the only line of defense. See `security.md` §3 (callback substitution), §9 (attached-method privilege).

## Readonly taint

Values accessed across a realm boundary are tainted read-only:

```go
// realm A imports realm B
b := externalrealm.B          // read: ok
b.FieldA = 42                  // PANIC: external realm's object is readonly
externalrealm.B = newB         // PANIC: same
```

Rules:
- Dot-selector access (`r.X`) on an external real value taints the result readonly.
- Index expression (`r.X[0]`) does the same.
- The taint **persists** through subsequent direct access — `externalrealm.X.Y.Z[0]` is readonly even if `Y` happens to live in the caller's realm.
- Function/method arguments and return values pass the taint through.
- To modify an external real object, call a function declared in the external realm that mutates it directly (closed-over scope).

**Round-tripping a slice through an external realm can produce a transient taint that doesn't fully clear on re-entry** — open issue #4765. See `security.md` §8.

## Closure capture (heap items)

Closures and package-level variables are represented internally as `*HeapItemValue`. Closures capture **heap items**, not the containing block. A closure created in realm A and stored in realm B's state **still references A's heap items**, so it carries A's identity / authority when invoked.

PR #4890 hardened the readonly stickiness on heap items inherited through closure captures, but the trust requirement remains: caller-supplied closures run with the *declarer's* realm authority via implicit borrow-cross on the captured state. See `security.md` §3.

## Guidelines (the spec author's mental model)

From `docs/resources/gno-interrealm.md` § Guidelines:

- **Public realm functions called by users must be crossing functions.** `MsgCall` only invokes crossing functions; users can't `MsgCall` non-crossing functions or `/p/` functions.
- **Methods should generally be non-crossing.** They're pre-bound to an object — a quasi-realm. A method that crosses into its declaring realm is "intrusive, but sometimes desired."
- **Utility functions** (common sequences of non-crossing logic) live as non-crossing functions in realm packages. They can import and call other realm utility functions; `/p/` packages cannot.
- **You can always cross-call a method from a non-crossing method if you need it.** The decision goes one way: non-crossing is the default, cross is the deliberate escalation.

## Cross-references

- `security.md` — bug classes built on each of the above primitives (§5 cur disclosure, §7 MsgRun trichotomy, §8 slice taint, §9 attached-method, §3 callback substitution)
- `patterns.md` — idioms that work *with* the model (crossing-function discipline, state shape, p/ vs r/)
- `stdlib.md` — `std.CurrentRealm`, `std.PreviousRealm`, `Realm.IsUserCall`, `Realm.IsUserRun`, `Realm.IsUser` API surface
- `render.md` — `Render()` is **not** a crossing function (no `cur realm` parameter)
- `future.md` — `cross2(rlm)` explicit caller form (PR #5669, in flight)

## Source

- `docs/resources/gno-interrealm.md` — canonical spec (38K, eight numbered sections).
- `.mynote/gno-agentic/reference/15-security-evolution-interrealm.md` §2 — timeline of merged spec PRs (#4060, #4192, #4264, #4429, #4750, #4890, #4899, #5039).
