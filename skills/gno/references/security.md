# Security

> **Category: security / audit.** Update when a new bug class is identified or a fix changes the canonical pattern.
> **Authoritative spec**: `docs/resources/gno-security.md` (threat-class taxonomy) and `docs/resources/gno-security-guide.md` (long-form patterns) in the `gnolang/gno` repo. This reference is a load-bearing summary plus auditor-oriented detection signals; the upstream docs are the source of truth.

## Purpose

Catalog of bug classes with concrete code shapes, grep signals, and fix patterns. Loaded by an agent reviewing a Gno realm for safety or guarding against known footguns while authoring one.

## Threat model

A **victim** realm `/r/V` holds state and exposes APIs. An **attacker** is any actor — end user, another realm `/r/A`, or a pure package `/p/A` — that can call into `/r/V`'s exposed surface. The attacker's goal is **write authority laundering**: causing a write to `/r/V`-stamped data while `m.Realm` is `/r/V` and the writing code path is attacker-controlled.

Assumptions:
- Attacker can deploy `/r/A` or `/p/A` and import `/r/V`.
- Attacker can call any exported function or method `/r/V` exposes and hold any pointer it returns.
- Attacker cannot use `reflect`, `unsafe`, goroutines, or any other escape hatch — these do not exist in Gno.
- Attacker cannot read `/r/V`'s unexported fields (Go's package-scoped identifier rule applies).

## How to use this reference

Each class follows the same template:

- **Shape** — minimal code example showing the bug.
- **Why wrong** — one or two lines naming the invariant violated.
- **Detection** — grep / search pattern an auditor agent can run.
- **Fix** — minimal code example showing the correction.
- **Status** — `ACTIVE` (still appears in master), `HISTORIC` (swept out of master but appears in deployed realms or imported code), `OPEN` (known bug not yet fixed).

Cross-link to `interrealm.md` for the spec model these violations build on.

## The five threat classes

Upstream taxonomy from `gno-security.md`. Code comments in the gno repo reference these class numbers (`// SECURITY (Class-4 captured callback)`).

| Class | Name | Mechanism |
|---|---|---|
| **1a** | cur-disclosure / impersonate-self | Hostile interface implementation captures `cur.Address()` / `cur.PkgPath()` from a `cur realm` parameter and later acts AS the realm that handed it the cur. |
| **1b** | cur-disclosure / impersonate-caller | Hostile implementation captures `cur.Previous()` and acts AS that realm's caller. |
| **2** | designation-forgery | Public method takes `(caller address, ...)` or `(pkgPath string, ...)`; any attacker calls it with the victim's identity. Also: non-crossing helpers that accept a secondary `realm` value and skip `rlm.IsCurrent()` before trusting it — a stale or previous realm value's `.Address()` still resolves but no longer refers to the live caller. |
| **3** | impl-substitution | Public function accepts an open interface; attacker supplies an implementation that lies on read (DoS or silent escalation). **Fires even when the interface has no realm-typed methods** — it's a read/behavior-integrity class, not a cur-leak class. |
| **4** | closed-over-authority | A canonical-typed value's constructor (or post-construction setter) takes attacker-controllable callback/data; the value passes an `IsCanonicalX` type check but carries hostile state. When Class 3 and Class 4 both apply, file as Class 4 — the allowlist passed; residual harm is captured-state. |

## Four structural defenses

The VM provides four independent defenses. A realm becomes exploitable when an API design defeats all of them at once.

| # | Defense | One-line summary |
|---|---|---|
| D1 | **Declaration-site rule** (borrow #1) | `/r/`-declared callables run with `m.Realm` borrowed to declaring `/r/`. Attacker code never gets victim authority just by being called by victim. |
| D2 | **Storage-site rule** (borrow #2) | `/p/` or stdlib methods on a real foreign-stamped receiver borrow `m.Realm` to the receiver's storage realm. Generic library helpers mutate caller-owned state only. |
| D3 | **Closure-capability rule** (borrow #3) | A closure carries the authority of its creator-realm forever. Cannot gain authority by changing hands. |
| D4 | **Readonly taint** | Foreign-realm reads are tainted; the taint propagates through copy, conversion, indexing. Mutations panic. No write path bypasses the check. |

## The safety hypothesis (A/B/C)

A victim realm `/r/V` is safe from external state mutation if **all three** of the following hold:

### (A) All logic-data types are `/r/`-declared.

Define `type User struct{...}`, `type Order struct{...}` in your own `/r/V` package, not in a shared `/p/`. Two reasons:

1. `/p/`-attackers cannot reference `/r/V` types in their signatures (`/p/ → /r/` imports are forbidden).
2. Any `/r/`-attacker impl of an interface taking `*v.User` runs with `m.Realm = /r/A` by D1 (borrow #1), so writes hit readonly.

The escape valve is the **encapsulation pattern** (GRC20 reference, below) — `/p/`-declared types with airtight encapsulation.

### (B) No `/p/`-type embedded in `/r/V`-data has higher-order methods with concretely-`/p/`-typed callbacks.

The subtle one. If `/r/V` has

```go
type Wrapper struct {
    Inner *somelib.Node
}
```

then attackers reach `Inner`, and `somelib.Node` may have `Iterate(cb func(*Node) bool)`. Inside `Apply`'s body, `m.Realm` is borrowed to `/r/V` by D2 (the `*Node`'s PkgID is `/r/V`). The body invokes `fn`. If `fn` is a top-level `/p/A.Evil` function with signature `func(*somelib.Node)`, **neither borrow rule fires** — top-level `/p/`-functions have no `/r/` declaring realm and no receiver. `m.Realm` stays at `/r/V` for the entire callback. Writes through the parameter commit under victim authority.

This is the **no-anchor case** — see `interrealm.md` § Three borrow rules.

Real-world `/p/`-types with this shape include `nt/avl/v0/node.Iterate`, `moul/cow/node.Iterate`, `onbloc/json/builder.WriteObject`.

### (C) Victim does not invoke caller-supplied function/interface values while holding its own authority.

The mirror of (B), viewed from `/r/V`'s API:

```go
func ApplyHook(fn func(any)) {
    fn(internalState)   // /r/V's m.Realm; attacker fn can launder
}
```

Closures handed in by an attacker are safe — D3 (borrow #3) borrows `m.Realm` back to the attacker for the body. The gap is narrower than it looks: it applies only to top-level `/p/` `FuncDecl` values, not arbitrary `func()` parameters.

**Defense in depth**: type the callback parameter with one of your own `/r/V`-declared types — `fn func(*v.User)`. `/p/` code can't name `v.User`, and any `/r/A` implementation runs under `/r/A`'s authority by D1.

Empirically verified across the `gnovm/tests/files/zrealm_launder_*.gno` probe corpus (~64 filetests, ~50 of them the `_rdata_` subset), each annotated with its attack mechanism and outcome.

## Bug class details (auditor mode)

### Class 1a/1b — cur-disclosure

**Shape**:

```go
// In /r/V, public crossing function:
func F(cur realm) {
    thirdParty.Hook(cur)   // forwarding cur to untrusted code
}

// thirdParty (attacker), declared as:
type Hooker interface { Hook(cur realm) }
```

**Why wrong**: `cur realm` is a capability token for the immediately-preceding crossing into `/r/V`. Forwarding it to attacker code lets the attacker capture `cur.Address()` (1a) or `cur.Previous()` (1b) and later impersonate `/r/V` or its caller.

**Detection**: interface methods declared with `cur realm` parameters; functions that pass `cur` (not `cross(cur)`) to subsequent cross-realm calls. The interface-with-`cur realm` shape is the primary tell.

**Fix**: never declare an interface method that takes `cur realm`. Take `caller address` instead, and let the crossing call site derive the address from its runtime-current `cur.Previous().Address()`.

**Status**: ACTIVE class (defensive lint). The compiler today rejects most expressions; the lint catches the cases it doesn't.

### Class 2 — designation-forgery

**Shape**:

```go
func DoThing(addr address) {
    log[addr] = ...   // anyone can call with any address
}
```

Or:

```go
func DoThing(_ int, rlm realm) {
    // missing IsCurrent() check
    addr := rlm.Previous().Address()   // cur.Previous() or a stale realm value still resolves
    log[addr] = ...
}
```

**Why wrong**: an `address` or `pkgPath` parameter is attacker-controlled. To identify the actual caller at an entrypoint, use the crossing function's first `cur realm` and derive the address inside. For non-crossing helpers that intentionally accept a secondary `rlm realm`, first prove `rlm.IsCurrent()`; otherwise the caller can pass `cur.Previous()` or another realm value. A stale or previous realm value can still resolve `.Address()` numerically — it just no longer refers to the live caller.

**Detection**: function signatures with `caller address` or `pkgPath string` as identity parameters; non-crossing helper signatures like `_ int, rlm realm` where `rlm` is used for authority without an `rlm.IsCurrent()` guard. Do not flag the first `cur realm` of a crossing function solely for lacking `cur.IsCurrent()`; the runtime guarantees it is current.

When reconciling upstream docs, treat `docs/resources/gno-interrealm-v2.md`'s public-API checklist as a broad prompt to inspect caller-identity use, not as an automatic finding against every crossing entrypoint. For the `_ int, rlm realm` migration shape, apply `gnovm/adr/interrealm_v2.md`: guard the helper's secondary `rlm`, not the crossing function's first `cur`.

**Fix**:

```go
func DoThing(_ int, rlm realm) {
    if !rlm.IsCurrent() { panic("spoofed realm") }
    addr := rlm.Previous().Address()
    log[addr] = ...
}
```

**Status**: ACTIVE. Most common LLM-generated bug shape — pattern-matching from Solidity's `msg.sender` produces wrong answers here.

### Class 3 — impl-substitution

**Shape**:

```go
type Voter interface { Cast(p ProposalID) bool }

func Vote(cur realm, v Voter, p ProposalID) {
    requirePermission()
    if v.Cast(p) {          // external call into UNKNOWN realm
        markVoted(p)         // state mutation AFTER external call
    }
}
```

**Why wrong**: `v` is caller-supplied. The implementation can lie on read (DoS or silent escalation), or invoke other `/r/V` methods to interleave state changes. Classic re-entrancy variant; Gno has no implicit `nonReentrant` guard.

**Detection**: public functions accepting an open interface (not gated by canonical-type check); functions that call an interface method AND mutate state on the same path without a generation counter or in-flight flag; embedded-marker "seal" patterns (which are bypassable — see Sealing below).

**Fix**: at every public entry point that accepts an interface from external callers, gate with a canonical-type assert:

```go
if _, ok := t.(*ConcreteImpl); !ok {
    panic("not a canonical impl")
}
```

Or use the package-provided predicate (`grc20.IsCanonicalTeller(t)`). Then apply checks-effects-interactions or an in-flight flag.

**Status**: ACTIVE. Common shape: a DAO/permission framework that lets governance code accept a caller-supplied `Permissions`/`Voter`/`Executor` interface and performs state writes after invoking one of its methods.

### Class 4 — closed-over-authority

**Shape**:

```go
type ExecFunc func(realm) error

func WithExecutor(cb ExecFunc) Option {
    return func(d *Definition) { d.executeCb = cb }
}

// Later, during proposal execution:
func (d *Definition) Execute(cur realm) error {
    return d.executeCb(cur)   // runs caller-supplied code with DAO authority
}
```

**Why wrong**: `cb` is captured from the caller's realm at config time. When invoked later under DAO authority, the function body executes with the storing realm's privileges. PR #4890 hardened the readonly-taint side; the caller-trust requirement remains by design.

**Detection**: function signatures containing `func(...)` or function-typed parameters; realm-stored function values (`var cb func(...)`); `SetXxx(cur realm, fn func(...))` style setters. **Also flag latent cases** — a function-valued state variable that's writable but currently never invoked. Today it's harmless; one future commit wiring it into a gated path turns it into an active Class 4 hazard. Mark YELLOW with a "wire-and-document-or-delete" recommendation.

**Fix**: prefer typed interfaces with known implementations over `func()` parameters. Where callbacks are unavoidable, document the trust assumption loudly. The constructor itself is the trust boundary — *the caller IS the authority for the lifetime of the constructed value*.

**Status**: ACTIVE. Typical shapes: `WithExecutor`-style option constructors.

## Payment-guard canonical pattern

The single most common security-relevant operation in `/r/` code:

```go
func Buy(cur realm) {
    if !cur.Previous().IsUserCall() {
        panic("only EOA via MsgCall can fund this")
    }
    coins := unsafe.OriginSend()
    if coins.AmountOf("ugnot") != price {
        panic("incorrect amount")
    }
    // mutate state
}
```

Three predicates that look interchangeable but aren't:

| Predicate | EOA via `MsgCall` | EOA via `MsgRun` (ephemeral) | Realm via `MsgCall` |
|---|---|---|---|
| `IsUserCall()` | ✓ | ✗ | ✗ |
| `IsUserRun()` | ✗ | ✓ | ✗ |
| `IsUser()` | ✓ | ✓ | ✗ |

`IsUser()` accepts both EOAs and the user's ephemeral `gno.land/e/g1<user>/run` realm created by `MsgRun`. The ephemeral can consume the `OriginSend` envelope **before** forwarding control, bypassing the receipt invariant. Use `IsUserCall()` for payment-gated paths.

**Detection**: file contains `unsafe.OriginSend(` AND `IsUser(` (no `Call` suffix). Both in the same function = high severity. `OriginSend` purely read-for-display = low severity.

**Status**: HISTORIC. No genuine example realm guards an `OriginSend` payment with `IsUser()` in the same function on current master; the only files referencing both predicates are deliberate test fixtures under `r/tests/vm/`, and there each predicate sits in a separate helper. The dangerous pattern still appears in older deployed realms and external code, so the auditor agent should expect to find it in user-supplied input.

## The encapsulation pattern (GRC20 reference)

`gno.land/p/demo/tokens/grc20` is the canonical example of *safe* `/p/`-declared data. It violates (A) — `Token`, `PrivateLedger`, and `fnTeller` are all `/p/`-declared — but compensates with airtight encapsulation:

| Defense | How |
|---|---|
| All sensitive fields are unexported | `Token.ledger`, `PrivateLedger.balances`, `PrivateLedger.allowances`, `fnTeller.accountFn` all lowercase. Foreign packages cannot access them. |
| No exported method leaks an interior pointer | No `Token` method returns `*PrivateLedger`, `*avl.Tree`, or `*avl.Node`. |
| Authority transitions gated at the right boundary | Crossing entrypoints use their runtime-current first `cur`; non-crossing helper methods that accept `_ int, rlm realm` check `rlm.IsCurrent()` before resolving realm identity. |
| Forgery defended by nominal type assertion | `IsCanonicalTeller(t)` checks `_, ok := t.(*fnTeller)`. Embedding wrappers fail this. |
| `*PrivateLedger`'s unauthenticated mutators isolated by package privacy | `Mint`/`Burn`/etc. have no `cur` check. They're safe only because no realm exports the `*PrivateLedger` pointer. |

Realm authors using GRC20 must:

1. Store `*PrivateLedger` in a **lowercase** package-level variable.
2. Expose only authenticated entry points (`Transfer(cur realm, to address, amount int64)` calling `userTeller.Transfer(...)`).
3. If accepting a `Teller` from external callers, gate with `IsCanonicalTeller(t)` before dispatching its methods.
4. **Never import `gno.land/r/tests/vm/test20`** — its `PrivateLedger` is deliberately exported for tests; using it in production = instant compromise.

## Anti-patterns (footguns)

### Exposing a pointer to mutable state

```go
var users []*User
func Users() []*User { return users }   // attacker gets aliased slice
```

The readonly taint protects you from direct field writes, but if `*User` has any method whose body writes the receiver, calling that method on the returned pointer succeeds — D2 borrows `m.Realm` back to `/r/V` and the write commits.

**Rule**: getters return values (copies), unexported method results, or read-only views. Never a pointer to internal mutable state.

### Embedding a `/p/`-type with concrete-callback higher-order methods

The (B)-class vector. When embedding/fielding a `/p/`-type, audit its method set. If it has any `func(...) func(*PType)`-shaped method, treat embedding as **publishing a mutator API** to the world.

**Rule**: either don't embed, or keep the field unexported AND don't return aliased pointers to it.

### Accepting an attacker callback under your own authority

The (C)-class vector. Even `func()` is dangerous — the callback body can call back into your own state-mutating methods.

**Rule**: never invoke a caller-supplied function/interface value while holding your own `m.Realm`. Either:
- Type the callback parameter with one of your own `/r/V`-declared types so attackers can't supply a matching `/p/`-callback, OR
- Don't invoke caller callbacks at all; design synchronous APIs.

### Sealing is not a security boundary

Unexported marker methods on an interface (`isCanonical()` etc.) are **bypassable via embedding** in Gno. See `examples/quarantined/gno.land/p/test/seal/filetests/z_seal_*_filetest.gno` for the four working bypass tests.

**Rule**: sealing is documentation, not defense. For real allowlists, use a concrete-type switch (`switch v.(type) { case *MyImpl: ... }`) at the boundary function.

### Stored `realm`-typed values

Storing a `realm` value (whether `cur` or `cur.Previous()`) panics at attachment time or transaction finalize: `cannot persist realm value: realm values are ephemeral and tied to a call frame`.

**Rule**: if you need to remember a caller across transactions, store `cur.Previous().Address()` or `cur.Previous().PkgPath()` (plain strings), not the realm value.

## Properties that strengthen the boundary

### Cross-realm panic aborts the transaction

A panic raised inside a realm-borrowed frame **cannot be caught by `recover()` in any other realm**. The transaction aborts entirely. A write that would have panicked at the readonly check takes the whole transaction with it — no half-mutated state, no recover-and-retry under a different guise. (The test-only `revive(fn)` builtin is the documented exception.)

### Readonly taint propagates through value copy

Reading a foreign struct value into a local variable preserves the readonly bit. Writing the local copy still panics. Go-semantics-divergent but closes a class of subtle attacks where attacker might "extract" victim data into their own context.

### Bound method values carry the receiver's PkgID

`mv := victim.Apply` is a function value that remembers its receiver. When invoked later — even stored in attacker state — D2 (borrow #2) fires based on the receiver's PkgID. Method *expressions* (unbound: `(*T).Apply`) do not carry the receiver stamp.

**Implication**: returning a bound method value of a `/p/`-type pointing into your state is equivalent to publishing the method to any holder. Don't return bound method values of `/p/`-types unless the method body is safe under attacker invocation.

### Conversion-time panic is not Gno-recoverable

`doOpConvert` Case 1 (foreign-readonly source conversion refused) uses raw Go `panic(...)`, **not** catchable by Gno `defer { recover() }`. The write-time readonly check is catchable. This is an implementation inconsistency, not a bug.

### Storage-construction-time check

Allocating a foreign `/r/`-declared type with a composite literal, `new()`, or `make()` panics: `cannot allocate <type> in realm <m.Realm>`. Attackers cannot fabricate impostor instances. Construction must go through constructors declared in the type's home realm.

## Audit signals (Phase-1 triage)

Quick-reference grep checklist. Run these from the realm root.

| Pattern | Signal | Class |
|---|---|---|
| `IsUser()` co-occurring with `OriginSend` | RED | payment-bypass via MsgRun |
| Helper/secondary `rlm.Previous()` / `rlm.Address()` without prior `rlm.IsCurrent()` check | RED | Class 2 — designation-forgery |
| Public method takes `caller address` / `pkgPath string` as identity parameter | RED | Class 2 — designation-forgery |
| `unsafe.PreviousRealm()` inside a non-crossing function used as caller identity | RED | Class 2 — does not identify the immediate caller |
| `interface { ... }` with `cur realm` parameter declared anywhere | YELLOW | Class 1a/1b — cur-disclosure surface |
| `interface { ... }` accepted as parameter and methods invoked without canonical-type assert | YELLOW | Class 3 — impl-substitution |
| `func(...)` parameters or function-typed state fields used in permission-gated paths | YELLOW | Class 4 — closed-over-authority |
| Function-valued state variable writable but never invoked | YELLOW | Class 4 — latent; wire-and-document-or-delete |
| Embedded `/p/`-type with `Iterate(cb func(*T))` / `Apply(fn func(*T))` shape on `/r/V`-data | YELLOW | (B) violation — no-anchor laundering surface |
| Exported field is a `/p/`-pointer or embedded `/p/`-type | YELLOW | (B) violation candidate |
| Exported function/var returns a pointer aliasing internal mutable state | YELLOW | mutator surface for D2 borrow back |
| Storing a `realm`-typed value in struct field / map / package var | RED | will panic at finalize; usually a Class 2 misunderstanding |
| Slice element mutation after round-trip through external realm | YELLOW | readonly-taint round-trip surprise (open issue #4765) |
| `crossing()` as a statement (pre-0.9 body marker) | RED | won't compile — migrate to `func F(cur realm, ...)` |

## Operational audit signals

Not Gno-language bug classes, but real audit signals an auditor should catch alongside the bug catalog.

| Pattern | Signal | Action |
|---|---|---|
| `unsafe.OriginSend()` consumed with no `SendCoins` / `Withdraw` / auto-forward elsewhere in the realm | RED | Funds lock in realm address. Either implement guarded withdraw, auto-forward to a treasury, or document burn-on-receipt intent. |
| Hardcoded `admin address` with no `TransferAdmin(cur realm, ...)` | YELLOW | Key loss = permanent loss of privileged ops. Ship rotation or document trade-off. |
| `_ := someAvl.Get(k)` / `n, _ := v.(int)` swallowing the second return | YELLOW | Silent fallback on missing key or wrong type. Future state-shape change corrupts the read invisibly. |
| `avl.Tree` storing only a single statically-named key | YELLOW | Over-typed; collapse to a scalar or expose multi-key API. Don't ship the middle. |
| Admin/privileged inputs with no bound check (negative `qty`, oversized strings) | YELLOW | Admin footgun; bound at the function boundary. |
| Exported `ErrXxx` declared but never returned anywhere in the package | YELLOW | Intent inconsistent with behavior — either wire the error or remove it. |
| Map iteration order surfaced as output ordering (`for k, v := range m` feeding `Render`, pagination, or a ranked list) | YELLOW | Gno map iteration is insertion-order and deterministic (the VM iterates an insertion-ordered list, not the Go map) — **not** a consensus risk. But insertion order is an unspecified impl detail, rarely the order users want, and unstable under delete+reinsert (a re-added key jumps to the tail). Store explicit sortable keys in an `avl.Tree`/`bptree` for any order users observe. |
| `math.MinInt` / `math.MaxInt` / `unsafe.Sizeof` / architecture-dependent ints | RED | Platform-dependent values; nodes on different architectures diverge. Use explicit-width types (`int64`, `uint32`). |
| `append` whose growth capacity is observed (`cap(s)` checked after append) | YELLOW | Go's append capacity-growth is version-dependent; code branching on `cap()` can diverge across Go versions. |
| Floating-point arithmetic in deterministic paths | YELLOW | Float formatting has platform edge cases. Prefer integer math; if floats are necessary, format with explicit precision. |
| Delete-and-recreate the same object identity within a tx | YELLOW | Realm finalization assigns object IDs by reachability. Refcount accounting can produce "unexpected object with id" / "unexpected zero object id" runtime errors. |

## Severity calibration

`RED` = exploitable today on current master, OR a block-worthy operational concern. Block deploy / send / interact.
`YELLOW` = exploitable depending on context (caller trust, CEI ordering, documentation). Investigate before clearing.
`GREEN` (implicit) = pattern matched but trust assumption is explicit and reasonable. Record so the next audit is faster.

Always cite the class number when reporting (`Class 1a`, `Class 4`, etc.) so the realm author can cross-reference back to this file. For operational signals, cite `security.md § operational`.

### `/p/` vs `/r/` audit lens

The catalog above is realm-audit-flavored. When auditing a `/p/` **package**:

| Severity | `/r/` realm | `/p/` package |
|---|---|---|
| **RED** | Exploitable today on master | Any naive importer ships a vulnerability (the library hands callers a footgun) |
| **YELLOW** | Exploitable depending on context | Importer must actively misuse it; library exposes the surface but doesn't force it |
| **GREEN** | Pattern matched, trust assumption explicit | Safe regardless of importer behavior |

A `/p/` package has no state of its own; its risk is **what every importing realm fails to wrap**. The same `ExecFunc func(realm) error` pattern that's RED in a deployed realm is YELLOW in a `/p/` library that documents the trust requirement and exposes the option only through a `With*` setter the importer can choose not to expose.

When auditing `/p/`, look for:
- Documentation honesty (`"v0 — Unaudited"` is honest calibration; raise it to the verdict).
- Whether dangerous shapes are necessary for the library's purpose or are convenience footguns.
- Whether the package documents the trust requirement on each callback/interface field.

Cite findings as `YELLOW (RED in any realm that exposes <surface> to public input)` when the shape is structurally necessary but importer-conditional.

## Verification checklist for realm authors

Before deploying a realm, verify:

- [ ] All logic-data types are declared in this package, OR `/p/`-declared types are stored in **unexported** package vars.
- [ ] Every exported function/method does one of: pure read (returns primitives or values, no internal pointers); is a crossing function whose first `cur realm` is used for caller identity; is a non-crossing helper that checks secondary `rlm.IsCurrent()` before trusting realm identity; documented intentionally permissive (faucet, public mint).
- [ ] No exported var or function returns a pointer aliasing internal mutable state.
- [ ] Every interface parameter from external callers is gated with a canonical-type assert before invoking methods.
- [ ] No method takes a `func(*MyPType)` callback (where `MyPType` is `/p/`-declared) and invokes it from within. If yes, retype the callback to use your own `/r/V`-typed parameter.
- [ ] No exported field is a `/p/`-pointer or embedded `/p/`-type with concretely-typed callback methods.
- [ ] Payment-guarded entry points use `cur.Previous().IsUserCall()`, not `IsUser()`.
- [ ] No `realm`-typed value is stored in package state, struct fields, maps, slices, or closure captures.
- [ ] Not imported `gno.land/r/tests/vm/test20` (deliberately insecure test fixture).

## Worked example — a secure counter realm

```go
// gno.land/r/example/counter
package counter

// /r/-declared data type. (A) satisfied.
type Counter struct {
    value int
    owner address
}

var gCounter *Counter   // unexported — only reachable through methods below

func init() {
    gCounter = &Counter{value: 0, owner: address("")}
}

// Public read. Returns a value, not a pointer.
func Value() int {
    return gCounter.value
}

// Authenticated mutator.
func Increment(cur realm) {
    gCounter.value++
}

// Authenticated owner-gated mutator.
func SetOwner(cur realm, newOwner address) {
    if gCounter.owner != "" && cur.Previous().Address() != gCounter.owner {
        panic("not the owner")
    }
    gCounter.owner = newOwner
}

// NO method like ApplyHook(fn func(*Counter)) — violates (C).
// NO method like GetCounter() *Counter — aliased-pointer leak.
```

Attackers can read `Value()` (returns a copy), call `Increment(cur)` (D1 keeps `m.Realm = counter`), call `SetOwner(cur, ...)` (owner check). They cannot write `gCounter.value` (unexported), Apply-launder it (no Apply method, no exported pointer), or spoof `cur.Previous().Address()` at the crossing entrypoint because the runtime supplies the live first `cur realm`.

## Cross-references

- `interrealm.md` — the spec model these violations build on (two contexts, three borrow rules, capability token semantics, readonly taint, conversion guards)
- `patterns.md` — preferred shapes that avoid these classes
- `stdlib.md` — `chain/banker`, `chain/runtime`, payment primitives
- `render.md` — XSS / untrusted-content posture for `Render()` output (separate trust axis)

## Source

- `docs/resources/gno-security.md` — five-class taxonomy (class numbers used in chain code comments).
- `docs/resources/gno-security-guide.md` — four structural defenses, safety hypothesis (A/B/C), encapsulation pattern, anti-patterns, surprising properties, checklist.
- `gnovm/tests/files/zrealm_launder_*.gno` — ~64 exploit-attempt filetests, each annotated with mechanism and outcome.
- `examples/quarantined/gno.land/p/test/seal/filetests/z_seal_*_filetest.gno` — four bypass tests showing why sealing is documentation, not defense.
