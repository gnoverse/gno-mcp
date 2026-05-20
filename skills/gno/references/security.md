# Security

> **Category: security / audit.** Update when a new bug class is identified or a fix changes the canonical pattern. In-flight fixes belong in `future.md` until merged.

## Purpose

Catalog of bug classes with concrete code shapes, grep signals, and fix patterns. Loaded by an agent reviewing a Gno realm for safety or guarding against known footguns while authoring one.

## How to use this reference

Each class follows the same template:

- **Shape** — minimal code example showing the bug.
- **Why wrong** — one or two lines naming the invariant violated.
- **Detection** — grep / search pattern an auditor agent can run.
- **Fix** — minimal code example showing the correction.
- **Status** — `ACTIVE` (the pattern still appears in master), `HISTORIC` (swept out of master but appears in deployed realms or imported code), `OPEN` (known bug not yet fixed).

Cross-link to `interrealm.md` for the spec model these violations build on. Load `future.md` before reporting an audit verdict — some classes have fixes in flight.

## Payment-guard canonical pattern (master, 2026-05)

The single most common security-relevant operation in `/r/` code. Pattern:

```go
func Buy(cur realm) {
    if !runtime.PreviousRealm().IsUserCall() {
        panic("only EOA via MsgCall can fund this")
    }
    coins := banker.OriginSend()
    if coins.AmountOf("ugnot") != price {
        panic("incorrect amount")
    }
    // mutate state
}
```

Why `IsUserCall()` and not `IsUser()`: `IsUser()` accepts both EOAs and the user's ephemeral `gno.land/e/g1<user>/run` realm created by `MsgRun`. The ephemeral realm can consume the `OriginSend` envelope *before* forwarding control, bypassing the receipt invariant.

Successor pattern `realm.SentCoins()` (PR #5039, merged 2026-04) is frame-local and re-entrancy-safe; adoption sweep across `examples/` is pending. See `future.md` for posture during the transition.

## Bug classes

### 1. `OriginSend()` + `IsUser()` — payment bypass via `MsgRun`

**Shape**:

```go
func Buy(cur realm) {
    if !runtime.PreviousRealm().IsUser() { panic("EOA only") }
    if banker.OriginSend().AmountOf("ugnot") != price { panic("bad amount") }
    // grant goods
}
```

**Why wrong**: `IsUser()` accepts `IsUserCall()` AND `IsUserRun()`. The user's `MsgRun` ephemeral realm can call `Buy()` after already consuming the same `OriginSend` envelope — the receipt invariant breaks and the goods are granted without payment.

**Detection**: file contains `banker.OriginSend(` AND `IsUser(` (no `Call` suffix). Both in the same function = high severity. `OriginSend` purely read-for-display = low severity.

**Fix**: replace `IsUser()` with `IsUserCall()`. Or migrate to `realm.SentCoins()` once adopted (see `future.md`).

**Status**: HISTORIC in master `examples/` (verified 2026-05: 0 co-occurrences). Documented in `docs/resources/effective-gno.md § Verifying inbound Coin payments`. Pattern still appears in older deployed realms and external code; the auditor agent should expect to find it in user-supplied input.

### 2. Interface-based re-entrancy (caller-supplied interface)

**Shape**:

```go
type Voter interface { Cast(cur realm, p ProposalID) bool }

// In DAO realm:
func Vote(cur realm, v Voter, p ProposalID) {
    requirePermission()
    if v.Cast(cross, p) {     // external call into UNKNOWN realm
        markVoted(p)          // state mutation AFTER external call
    }
}
```

**Why wrong**: `v` is caller-supplied. `v.Cast(cross, ...)` transfers execution into the caller's realm, which can re-enter `Vote()` with a different `Voter` or interleave other state changes. Classic re-entrancy; Gno has no implicit `nonReentrant` guard.

**Detection**: interface types whose methods take `cross` or `cur realm`, with instances stored in realm state or accepted as parameters. Cross-check: any function that calls an interface method AND mutates state on the same path, without a generation counter or in-flight flag.

**Fix**: either (a) restrict implementations to known realms via path-allowlist (boards2 pattern, PR #4750), (b) finalize all state mutations *before* the external call (checks-effects-interactions), or (c) gate with an in-flight flag cleared on entry/exit.

**Status**: ACTIVE class. The pre-#4884 `samcrew/daokit` `Permissions` interface is the canonical bad example. The daokit upgrade (PR #4884, see `future.md`) addresses some instances; new code should still follow the discipline.

### 3. Behavior substitution via user-supplied callbacks

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

**Why wrong**: `cb` is captured from the caller's realm at config time. When invoked later under DAO authority, the function body executes with the storage realm's privileges. The seminal PR #4890 review thread (`alice.SetClosure(cross, peter.F(alice.AllowedList))`) shows variants where this is exploitable across multiple realms; readonly-taint stickiness (#4890) limits but does not eliminate the trust requirement.

**Detection**: function signatures containing `func(...)` or function-typed parameters; realm-stored function values (`var cb func(...)`); `SetXxx(cur realm, fn func(...))` style setters; calls to caller-supplied function values from permission-gated functions. **Also flag latent cases** — a function-valued state variable that's writable but currently never invoked. Today it's harmless; one future commit that wires it into a gated path turns it into an active §3 hazard. Mark as YELLOW with a "wire-and-document-or-delete" recommendation.

**Fix**: prefer typed interfaces with known implementations over `func()` parameters. Where callbacks are unavoidable, document the trust assumption and never use them in payment- or permission-gated paths.

**Status**: ACTIVE. Live in `examples/gno.land/p/nt/commondao/v0/exts/definition/options.gno` (`WithExecutor`, `ExecFunc func(realm) error`) and surrounding code. PR #4890 hardened the readonly-taint side; the caller-trust requirement is by design.

### 4. Cross-realm method calls on unattached / unpersisted receivers

**Shape**:

```go
obj := alice.NewObject()    // obj not yet persisted to alice's state
bob.Do(obj.Method)           // bob calls obj's method
```

**Why wrong**: the receiver's storage realm determines which realm the method runs as. An unattached object has no storage realm, so the call either is rejected (post-#4890) or used the wrong realm (pre-fix, obscure panic).

**Detection**: method expressions (`obj.Method` without parens) passed across realm boundaries; `new(T).Method` or `(*T)(nil).Method` patterns.

**Fix**: persist the receiver to realm state before taking the method expression. Or use the "virtual realm" pattern from PR #4890 review for nil-receiver type-based dispatch.

**Status**: HISTORIC. PR #4890 produces a clear VM error rather than silent misbehavior.

### 5. `cur realm` disclosure to untrusted callees

**Shape**:

```go
func F(cur realm) {
    thirdParty.Hook(cur)   // forwarding our realm authority outward
}
```

**Why wrong**: `cur realm` is the proof that the immediately-preceding caller crossed into us deliberately. Forwarding it (instead of using `cross` for a new explicit crossing) leaks authority. Currently the type system makes this hard to express, but the convention is enforced by review (PR #4394 "Don't make an entry field for cur realm" in gnoweb help is the UX confirmation).

**Detection**: function signatures with `cur realm` AND that pass `cur` (not `cross`) to subsequent cross-realm calls. Should be very rare; flag any occurrence.

**Fix**: call subsequent realms with `cross`. `cur` is consumed at the function boundary, full stop.

**Status**: ACTIVE class (defensive lint). Compiler today rejects most expressions of this; the lint is for the cases it doesn't.

### 6. Pointer-stealing via realm assignment

**Shape** (historical, kept for the model):

```go
// realm A
var x = uint32(42)
steal.Steal(&x)   // realm B captures &x

// realm B
var ptr *uint32
func Steal(p *uint32) { ptr = p }   // B persists A's pointer
```

**Why wrong**: assigning a cross-realm pointer to B's state caused ownership transfer to B. Subsequent `*ptr = ...` in realm A would panic with "cannot modify external-realm object".

**Detection**: `&someVar` where `someVar` is a top-level realm-state variable, passed to a function in another realm; the receiving realm stores it in state.

**Fix**: pass by value, or copy first — `localCopy := remote.X; otherRealm.Foo(&localCopy)`.

**Status**: HISTORIC. Issue #974 (2023). Absorbed into #4028's readonly-taint design — cross-realm references now carry the readonly bit and can't be assigned into external-realm state without explicit `cross`.

### 7. `MsgRun` vs `MsgCall` semantic confusion — the trichotomy

This is the *conceptual* framing of class §1; same fix, different angle. An auditor agent benefits from catching both surface and concept.

**Shape**:

```go
func Buy(cur realm) {
    // dev wants "user only", reaches for IsUser
    if !runtime.PreviousRealm().IsUser() {
        panic("user only")
    }
    coins := banker.OriginSend()
    // ...
}
```

**Why wrong**: `Realm` exposes three caller-identity predicates that look interchangeable but aren't.

| Predicate | EOA via `MsgCall` | EOA via `MsgRun` (ephemeral realm) | Realm via `MsgCall` |
|---|---|---|---|
| `IsUserCall()` | ✓ | ✗ | ✗ |
| `IsUserRun()` | ✗ | ✓ | ✗ |
| `IsUser()` | ✓ | ✓ | ✗ |

The `MsgRun` ephemeral realm (`gno.land/e/g1<user>/run`) can consume `OriginSend` and then forward control, so any payment-gated function whose guard accepts `IsUser()` (broad) is exploitable. The author's intent ("only allow user-initiated calls") was correct; the API choice was too broad.

**Detection**: `IsUser()` (no `Call` suffix) in payment-gated paths; `PreviousRealm().PkgPath() == ""` used as a user-only check (technically captures only `MsgCall`-from-user, but suggests the author didn't understand the trichotomy — flag for review).

**Fix**: `IsUserCall()` for "only EOA via MsgCall" (the payment-guard case). `IsUserRun()` only when explicitly building dev tooling. Prefer the named API over `PkgPath()` comparisons.

**Status**: HISTORIC in master `examples/` (verified 2026-05: 0 co-occurrences). External / older code may still ship the antipattern. Root cause of §1; documented in PR #4192.

### 8. Slice mutation across realms (readonly-taint surprise)

**Shape**:

```go
// In realm crossrealm
var Allowed = S{AllowedList: []int{1, 2}}
func Set(cur realm, v []int) { Allowed.AllowedList = v }
func Edit(cur realm, i, v int) { Allowed.AllowedList[i] = v }

// In main
crossrealm.Set(cross, crossrealm.Allowed.AllowedList)   // alias
crossrealm.Edit(cross, 0, 5)                            // PANIC: readonly tainted
```

**Why wrong**: when a slice from realm X passes through realm Y and back to X, the slice descriptor inherits a transient readonly taint that isn't fully cleared on re-entry. Element mutation then panics.

**Detection**: a function in realm X that accepts a slice and re-assigns or element-mutates a state slice from the same realm. High risk if the slice originated from an `import`'d realm path.

**Fix**: clone on entry — `v := append([]int(nil), v...)`. Or replace whole slices instead of element-mutating — `Allowed.AllowedList = []int{...}`, not `Allowed.AllowedList[i] = ...`.

**Status**: OPEN. Issue #4765 (2025-09), not fixed at scaffold-time. The advice is don't round-trip slices.

### 9. Soft cross to FuncValue declaration realm vs storage realm — attached-method authority grant

**Shape**:

```go
// In realm bob:
var obj alice.Object = alice.New()   // bob persists an alice.Object to its state

// In alice's code, method declared as:
func (*Object) Drain() {
    bob.Treasury = 0                 // runs with bob's storage authority via borrow-switch
}

// In bob (later):
obj.Drain()                          // method body executes with bob's privileges, drains bob's treasury
```

**Why wrong**: a method on a struct can be **declared in realm A but the struct stored in realm B**. The implicit storage-realm borrow-switch targets B (where the receiver lives), not A (where the method was declared). Whoever attaches the struct to their state has effectively granted A's code execution privileges over B's storage.

**Detection**: realm-state variables typed with imports from other realms (especially `r/...` imports, not just `p/...`); methods on such types called as `obj.Method()` where the storage-realm borrow-switch kicks in. Pay special attention to imports from less-trusted authors.

**Fix**: prefer composition with `p/` (pure) types only. If you must store an `r/` type, audit every method on it as if it were code in your own realm. **Note the supply-chain dimension**: imported `r/` realms are runtime-upgradeable from the dependency author's side — your audit is valid only for the version you reviewed. A future widget release can change behavior without manager noticing. Jae Kwon (PR #4584 review): *"don't attach objects/receivers to your realm unless you know it's safe."*

**Status**: ACTIVE — policy not bug. Acknowledged by core as a deliberate design trade-off (#4584 closed unmerged). PR #4890 hardens the readonly-taint side; the attachment-as-privilege model itself remains. **This is the single largest LLM-auditable class** — the compiler will not save you, the audit must.

### 10. Stale-spec code copy

**Shape**:

```go
func F() {
    crossing()                       // pre-Gno-0.9 body marker
    // ...
}
```

Or:

```go
// Old caller-side syntax:
F(cross, args...)                    // calling a function that doesn't have `cur realm` first param
```

**Why wrong**: pre-#4060 / pre-#4264 syntax. Won't compile against current chain. If a transpiler accepts it, may produce subtly different semantics.

**Detection**: function body contains `crossing()` as a statement; function `F(...)` with no `cur realm` first parameter called as `F(cross, ...)`; `gno.mod` pinning version before 0.9.

**Fix**: generate only Gno 0.9+ syntax. Auditor agent should flag and offer transpile-style rewrite.

**Status**: HISTORIC in master (verified 2026-05: 0 `crossing()` body markers in `examples/`). Common in user-supplied or copy-pasted external code.

## Audit signals (grep checklist)

Quick-reference for Phase-1 triage. Run these from the realm root.

| Pattern | Signal | Action |
|---|---|---|
| `IsUser()` co-occurring with `OriginSend` | RED | §1 — replace with `IsUserCall()` |
| `crossing()` as a statement | RED | §10 — pre-0.9 stale; migrate to `func F(cur realm, ...)` |
| `PreviousRealm()` inside a non-crossing function used as caller identity | RED | §7 — does not identify the immediate caller |
| Method on receiver persisted into other realm's state | YELLOW | §9 — implicit storage-realm authority grant; audit the method |
| `interface { ... }` stored in state, methods invoked from gated functions | YELLOW | §2 — re-entrancy risk; verify CEI ordering |
| `func(...)` parameters or function-typed fields in realm state | YELLOW | §3 — behavior substitution; verify trust assumptions |
| `&someStateVar` passed to external realm | YELLOW | §6 — ownership-transfer risk (mostly defended by readonly taint now) |
| Slice element mutation after round-trip through external realm | YELLOW | §8 — open issue #4765; clone on entry |
| `cur realm` forwarded as an argument to a non-crossing function call | RED | §5 — authority disclosure |

## Operational audit signals (non-bug-class but block-worthy)

Not Gno-language bug classes, but real audit signals an auditor should catch alongside the bug catalog. Detail and idioms in `patterns.md` § "Operational anti-patterns" — duplicated here as grep signals so audit-mode loads them.

| Pattern | Signal | Action |
|---|---|---|
| `banker.OriginSend()` consumed with no `SendCoins` / `Withdraw` / auto-forward elsewhere in the realm | RED | Funds lock in realm address. Either implement guarded withdraw, auto-forward to a treasury, or document burn-on-receipt intent. |
| Hardcoded `admin std.Address` with no `TransferAdmin(cur realm, ...)` function | YELLOW | Key loss = permanent loss of privileged ops. Ship rotation or document trade-off. |
| `var cb func(...)` set-able but never invoked anywhere in the realm | YELLOW | Latent §3 — see "Behavior substitution" above. Wire-and-document or delete. |
| `_ := someAvl.Get(k)` / `n, _ := v.(int)` swallowing the second return | YELLOW | Silent fallback on missing key or wrong type. Future state-shape change corrupts the read invisibly. |
| `avl.Tree` storing only a single statically-named key | YELLOW | Over-typed; either collapse to a scalar or expose multi-key API. Don't ship the middle. |
| Admin / privileged inputs with no bound check (negative `qty`, oversized strings, etc.) | YELLOW | Admin footgun; bound at the function boundary. |
| Pointer-receiver method called on a value extracted from `interface{}` (e.g. `v.(T)` then `v.Method()` where `Method` is on `*T`) | YELLOW | Go auto-addresses the local copy; mutations in the method body don't persist to the stored value. Subtle Gno-vs-Go gotcha when the stored value lives in an `avl.Tree`. Store `*T` or document the value-store contract. |
| Exported `ErrXxx` declared in the package but never returned anywhere in the package | YELLOW | Intent inconsistent with behavior — the error name implies a check that the code doesn't perform (typical case: `ErrVoteExists` declared but `AddVote` silently overwrites). Either wire the error or remove it. |
| Map iteration order influencing execution (`for k, v := range m { … emit / sum / mutate }`) | RED | Go's map iteration is non-deterministic. Different nodes processing the same tx will iterate in different orders and diverge — a consensus halt risk. Convert to `avl.Tree` (or `bptree`) for ordered traversal. |
| Use of `math.MinInt` / `math.MaxInt` / `unsafe.Sizeof` / architecture-dependent ints | RED | Values are platform-dependent. Two nodes on different architectures produce different results. Use the explicit-width types (`int64`, `uint32`) instead. |
| `append` whose growth capacity is observed (e.g., `cap(s)` checked after append) | YELLOW | Go's `append` capacity-growth strategy is version-dependent. Code that branches on `cap()` post-append can diverge across nodes running different Go versions. Don't observe capacity. |
| Floating-point arithmetic in deterministic paths | YELLOW | Float formatting has platform edge cases. If you must use floats, format with explicit-precision (`strconv.FormatFloat`) and validate against the spec; prefer integer math wherever possible. |
| Delete and re-add the same object identity in a single tx (e.g., `tree.Remove(k); tree.Set(k, newObj)` where `newObj` overlaps the prior object's reachable graph) | YELLOW | Realm finalization assigns object IDs by reachability. Refcount accounting on delete-then-recreate within a tx can produce "unexpected object with id" / "unexpected zero object id" runtime errors. Replace whole values; don't intermix delete and recreate of overlapping graphs. |

## Severity calibration

`RED` = exploitable today on current master, OR a block-worthy operational concern. Block deploy / send / interact.
`YELLOW` = exploitable depending on context (caller is trusted? trust assumption documented? CEI ordering preserved?). Investigate before clearing.
`GREEN` (implicit) = pattern matched but trust assumption is explicit and reasonable. Record so the next audit is faster.

Always cite the class number when reporting (`§1`, `§9`, etc.) so the realm author can cross-reference back to this file. For operational signals, cite `security.md § operational`.

### Audit lens: `/p/` package vs `/r/` realm

The bug-class catalog above is realm-audit-flavored — "is this realm exploitable today?" When the audit target is a `/p/` **package**, the rubric shifts:

| Severity | `/r/` realm | `/p/` package |
|---|---|---|
| **RED** | Exploitable today on master | Any naive importer ships a vulnerability (the library hands callers a footgun) |
| **YELLOW** | Exploitable depending on context | Importer has to actively misuse it; library exposes the surface, doesn't force it |
| **GREEN** | Pattern matched, trust assumption explicit | Safe regardless of importer behavior |

A `/p/` package has no state of its own and no direct attack surface; its risk is **what every importing realm fails to wrap**. The same `ExecFunc func(realm) error` pattern that's RED in a deployed realm is YELLOW in a `/p/` library that documents the trust requirement and exposes the option only through a `With*` setter the importer can choose not to expose.

When auditing a `/p/`, look for:
- Documentation honesty (e.g., `"v0 — Unaudited"` in `doc.gno` is honest calibration, raise it to the verdict)
- Whether dangerous shapes are necessary for the library's purpose or are convenience footguns that could be removed
- Whether the package documents the trust requirement on each callback/interface field

Cite findings as `YELLOW (RED in any realm that exposes <surface> to public input)` when the dangerous shape is structurally necessary but importer-conditional. This is more honest than flattening to RED at the wrong layer.

## Cross-references

- `interrealm.md` — the spec model these violations build on (especially "Implicit borrow-cross on methods", "Readonly taint", "Closure capture", and the "Two contexts" table)
- `patterns.md` — preferred shapes that avoid these classes
- `stdlib.md` — `banker.OriginSend`, `std.PreviousRealm.IsUserCall()`, `realm.SentCoins()`
- `render.md` — XSS / untrusted-content posture for `Render()` output (separate trust axis)
- `future.md` — fixes in flight (PR #5669 `cross2(rlm)`, `realm.SentCoins()` adoption sweep, daokit upgrade #4884)

## Source (internal)

`.mynote/gno-agentic/reference/15-security-evolution-interrealm.md` §4 — 10 bug classes with PR archaeology.
`.mynote/gno-agentic/reference/17-pr5669-and-security-comments.md` §3 — repo-wide `// SECURITY` comment audit; provides additional evidence for §2 (treasury banker iface), §3 (sealed-interface bypass), §9 (token-identity attacks).
`docs/resources/gno-interrealm.md` — canonical spec.
`docs/resources/effective-gno.md § Verifying inbound Coin payments` — canonical payment guard.
