# Patterns and idioms

> **Category: conventions / how to write Gno.** Update when a recommended pattern becomes the community standard or is deprecated. Anti-patterns are catalogued in `security.md`; this file is the "do this" companion.
> **Authoritative spec**: `docs/resources/effective-gno.md` in the `gnolang/gno` repo. When this reference and the upstream disagree, the upstream wins — file an update.

## Purpose

Idiomatic shapes that fit Gno's grain. Most idioms here are not stylistic preferences — they are responses to constraints vanilla Go programmers don't face. Knowing the **why** lets an agent generalize patterns to novel situations rather than match them blindly.

## Why these patterns — the chain-context drivers

| Force | Constraint it imposes | Patterns it drives |
|---|---|---|
| **Determinism** | Every node must produce the same result. Diverging on a single byte halts consensus. | `avl.Tree` over `map` (map iteration order); avoid `math.MinInt` / `MaxInt` (architecture-dependent); avoid float-format reliance; never let map iteration affect execution. |
| **Gas metering** | Every read, allocation, and store op costs gas the caller pays. | Lazy allocation — don't create state until needed; minimize cross-realm reads (readonly taint forces copies); single top-level struct for state (one root pointer instead of many globals); avoid unnecessary AVL traversals. |
| **Storage deposit** | Persisting bytes locks GNOT tokens; deleting refunds them. | Small structs, scalar types where possible, scheduled cleanup of expired data — refund-on-delete enables cleanup-as-revenue. |
| **Refcount persistence** | Realm finalization assigns object IDs and refcounts to every reachable object. | Avoid cyclic references in persisted state; don't delete-and-recreate the same object identity within a tx (refcount accounting); prefer composition with `/p/` types over `/r/` types in state. |
| **Cross-realm authority** | Realm boundaries are security boundaries. | Prefer non-crossing methods; never accept caller-supplied `func()` or `interface{}` in permission-gated paths; explicit `cross(cur)` for every external state mutation. |
| **Verifiability** | Source is on-chain. Humans and agents read it. There is no off-chain doc to drift. | Godoc as user-facing API doc; explicit guards over clever invariants; simple type signatures; one obvious way to do each thing. |
| **Tx atomicity** | A transaction either fully succeeds or fully reverts. | `panic` for unrecoverable conditions; CEI ordering (checks → effects → interactions); `Must*` panic-wrappers are safe. |
| **No host access** | The Gno VM has no `os`, `net`, `time.Now()` (deterministically), `syscall`. | No external API calls; events (`chain.Emit`) for off-chain consumers; off-chain tooling reads events + state, doesn't poll. |

Cross-reference: `interrealm.md` for the spec mechanics behind cross-realm authority and readonly taint; `security.md` for what goes wrong when these drivers are ignored.

## Counter-intuitive idioms (vs vanilla Go)

### Embrace global variables in realms

Globals in `/r/` are not the Go anti-pattern. They ARE the persistence layer. Every `var X = ...` is a root in the persistence graph; new and modified objects under the realm-storage-context are persisted at transaction finalization. Treat globals as fields on a singleton.

```go
package shop

import "gno.land/p/nt/avl/v0"

var (
    inventory = avl.NewTree()
    admin     address
)
```

**Do not export globals.** An exported variable is readable AND writable by anyone with the import. Use lowercase + getter/setter functions to control access.

In `/p/` packages, globals are different — `/p/`-immutability gate freezes them post-init. Use `const` for shared values; `var` is restricted to init-time setup.

### Embrace `panic`

Gno departs from Go's "don't panic" advice. Panic is a **control-flow primitive** here — when something goes wrong (invalid input, failed precondition, unmet payment), panic to abort the transaction cleanly. The chain rolls back state; the user sees the panic message.

```go
if !cur.Previous().IsUserCall() { panic("must be an EOA call") }
if amount <= 0 { panic("amount must be positive") }
```

The state-revert guarantee is `panic`-specific. **Returning an `error` does NOT revert state**; the caller has to choose to revert. For unrecoverable conditions where you want fail-and-revert, panic is the only correct choice.

`error`-return remains right for caller-checkable conditions with a meaningful recovery path: `Get(id)` returning `(Item, error)` when the item may legitimately not exist.

Reusable `/p/` packages should avoid `panic()` except in `Must*` / `Assert*` wrappers (which are explicit about their behavior). The caller picks: error-returning for branching, `Must*` for "succeed or kill the tx."

### `init()` is the constructor

`init()` runs once during deploy (`MsgAddPackage`) and never again. It's the only chance the realm has to:

1. Set up initial state (`admin = cur.Previous().Address()` — but in init the modern form is `runtime.OriginCaller()`).
2. Register with other realms (`registry.Register(cross(cur), "myID", ...)`).

```go
import "chain/runtime"

var admin address

func init() {
    admin = runtime.OriginCaller()   // the deployer
}
```

After deploy, the deployer is no longer accessible. If you need them, capture during init.

### A little dependency beats a little copying

Vanilla Go: "a little copying is better than a little dependency." Gno inverts this for `/p/` packages, more like the NPM model: small, focused modules; many small dependencies. Reasons:

- Contracts are user-readable; explicit imports give users transparency about what code runs.
- Gno's permission rules mean a published package can never be silently overwritten — `gno.land/p/X` is fixed.
- A reviewed, audited, high-usage `/p/` package is a smaller audit footprint than re-implementation.

Trust assumptions still apply — vet what you import — but the supply-chain attack surface differs from generic open-source. Prefer existing well-used `/p/` libraries.

## Package organization

- **Last path element matches the package name.** `gno.land/p/myorg/dao` should declare `package dao`. The `/vN` version suffix is ignored when matching. Avoid abbreviations unless they're widely known.
- **Realm namespace**: `gno.land/r/NAMESPACE/DAPP` is the primary location, similar to GitHub paths.
- **Internal packages**: a package with `internal` in its path can only be imported by packages sharing the same root.

  ```
  gno.land/p/demo/mypackage
  ├── utils
  └── internal/
      ├── helpers
      └── crypto
  ```

  Here `mypackage/internal/...` is importable only by `mypackage` and `mypackage/utils`. Use this to restrict reach.

### Types and interfaces belong in `/p/`

For any concept you expect other realms or packages to import, define types and interfaces in `/p/`, runtime in `/r/`:

- `/p/myorg/dao` — the `DAO` and `Vote` interfaces; the `Proposal` type
- `/r/myorg/dao` — the actual running DAO instance

This works because `/p/` can only import `/p/`, while `/r/` can import anything. Standards must be expressible in `/p/` so any realm can compose with them.

If you're writing a one-off app, you can do everything in `/r/`. The `/p/` separation matters when a concept is reusable.

## Realm as public API

A `/r/` realm is not a Go program. It's a **public API**. Anything exported can be called by any other realm; assume hostile callers and bound every input.

```go
func PublicMethod(cur realm, nb int) {
    if !cur.IsCurrent() { panic("spoofed realm") }
    if nb <= 0 || nb > MaxBatch { panic("nb out of range") }
    privateMethod(cur.Previous().Address(), nb)
}

func privateMethod(caller address, nb int) { /* ... */ }
```

Public functions gate (caller checks, payment validation, input bounds) and then delegate to private helpers. Helpers don't repeat gates; they trust their inputs.

## Crossing-function discipline

```go
// Public, called by users via MsgCall. Crossing function — first param is cur realm.
func Buy(cur realm, sku string) {
    if !cur.IsCurrent() { panic("spoofed realm") }
    if !cur.Previous().IsUserCall() { panic("EOA call only") }
    requirePayment()
    buy(sku)
}

// Internal helper. Non-crossing.
func buy(sku string) { /* ... */ }
```

Conventions:

- **`MsgCall` only dispatches to crossing functions.** Functions meant to be called externally MUST take `cur realm` first.
- **Methods should generally be non-crossing.** Per the spec author: methods are "pre-bound to an object — a quasi-realm." A method that crosses is "intrusive, but sometimes desired."
- **Authority lives in the crossing-function wrapper, not the helper.** Crossing function gates; helpers do work.
- **`/p/` packages can't declare crossing functions** (compiler-enforced).

### `cross(cur)` for explicit cross-calls

When calling a crossing function in another realm, use `cross(cur)` as the first argument:

```go
import "gno.land/r/some/registry"

func Register(cur realm, ...) {
    if !cur.IsCurrent() { panic("spoofed realm") }
    registry.Register(cross(cur), "myID", myCallback)
}
```

Inside the same realm, you can also call crossing functions non-crossing via `fn(cur, ...)` — no boundary, no finalization, no realm-context change. Use the non-crossing form when you don't want to spend the boundary cost.

## State shape

### `avl.Tree` over `map` for persisted keyed state

```go
// AVL (right for persisted state)
import "gno.land/p/nt/avl/v0"
var users avl.Tree
users.Set("alice", &User{...})
users.Iterate("", "", func(key string, value any) bool {
    user := value.(*User)
    return false   // continue
})

// map (wrong for persisted state)
users := make(map[string]User)
```

**AVL Trees**: O(log n) lookup, **lazy loading** (only the search path loads), **sorted iteration**, **deterministic**. Required for any keyed state that grows or that you iterate.

**Maps**: O(1) lookup, type-safe values. Use only for **small bounded** in-memory state (e.g. config values). Never for persisted growth state — non-deterministic iteration is a consensus halt risk.

`gno.land/p/nt/bptree/v0` (B+tree) is an accepted alternative for ordered keyed state — used by `commondao` and others. Either is fine; pick one and stay consistent.

### Lazy initialization for heavy state

If a sub-collection is expensive to create and not always needed, allocate on first use:

```go
var members *avl.Tree

func ensureMembers() *avl.Tree {
    if members == nil {
        members = avl.NewTree()
    }
    return members
}
```

Saves both deploy gas and steady-state read costs.

### Single top-level struct for upgradability

Proxy + Implementation patterns serialize cleanly from a single struct; scattered globals make migration harder. If you anticipate an upgrade, wrap state:

```go
type State struct {
    Inventory *avl.Tree
    Admin     address
    Counters  *Counters
}

var state = &State{...}
```

### Compose with `/p/` types, not `/r/` types

Storing a value of an `/r/` type in your realm's state grants that realm's code execution privileges over your storage (via D2 — the storage-site borrow rule). See `security.md` for the (A) safety hypothesis.

```go
// Risky: importing /r/widget and storing one of its types in our state
import "gno.land/r/widget"
var item *widget.Widget   // every widget method now runs with our authority via D2
```

When you must use a `/p/`-declared type, follow the **encapsulation pattern** from `security.md`: lowercase fields, no exported method returning interior pointers, authority transitions gated by `cur.IsCurrent()`. The `grc20` package is the canonical example.

## Safe-object pattern

A safe object is designed to be tamper-proof: instantiated by `/r/V`, exported as a value other realms can link to, but with airtight access control:

```go
type MySafeStruct struct {
    counter int
    admin   address
}

func NewSafeStruct(cur realm) *MySafeStruct {
    if !cur.IsCurrent() { panic("spoofed realm") }
    return &MySafeStruct{
        counter: 0,
        admin:   cur.Previous().Address(),
    }
}

func (s *MySafeStruct) Counter() int { return s.counter }

func (s *MySafeStruct) Inc(cur realm) {
    if !cur.IsCurrent() { panic("spoofed realm") }
    if cur.Previous().Address() != s.admin { panic("permission denied") }
    s.counter++
}
```

Other realms can register the object but cannot bypass its mutation gates. Note: returning a pointer to internal mutable state is generally a red flag — only do it when the methods on the pointer enforce sufficient guards.

## Payment handling

### Coins vs GRC20 — which to pick

| Need | Use |
|---|---|
| Native chain currency, IBC-ready, strict semantics, off-chain transferable | **Coins** (via `chain/banker`) |
| Programmable balance logic, allowance/approve patterns, contract-owned tokens | **GRC20** (`gno.land/p/demo/tokens/grc20`) |

Coins are simpler and more constrained. GRC20 is the right choice when you need ERC20-style behaviors.

### Payment-guard canonical pattern

```go
import (
    "chain/banker"
)

func Buy(cur realm) {
    if !cur.IsCurrent() { panic("spoofed realm") }
    if !cur.Previous().IsUserCall() {
        panic("only EOA via MsgCall can fund this")
    }
    if banker.OriginSend().AmountOf("ugnot") != price {
        panic("incorrect amount")
    }
    // mutate state
}
```

**Always pair `IsUserCall()` with the `OriginSend()` amount check.** Either alone is unsafe — `OriginSend()` without the EOA guard is lying about receipt (an intermediate realm could have consumed the coins); the EOA guard without the amount check lets users pay nothing.

Why **`IsUserCall()` and not `IsUser()`**: `IsUser()` accepts `IsUserCall()` AND `IsUserRun()`. The ephemeral `MsgRun` realm can consume the origin-send envelope and forward control, bypassing the receipt invariant. See `security.md` § Payment-guard.

### Inbound payment without an exit

A realm that consumes `OriginSend()` but provides no `Withdraw` / auto-forward path locks funds at the realm address. Either implement a guarded withdraw, auto-forward to a treasury, or document burn-on-receipt intent. Audit checklist item.

### No ERC-20-style `transferFrom`

Gno banker is **push-only**: `banker.SendCoins(from, to, amt)` requires `from == pkgAddr` (your own realm's address). You cannot pull from a caller's address. Payment flow is the `-send` envelope + `OriginSend()` amount check + `IsUserCall()` guard.

## Events

Emit events for any state change off-chain consumers care about. They cost gas but are the canonical way to expose state changes to indexers, gnoweb activity views, and agentic-AI tools — without forcing them to diff full state snapshots.

```go
import "chain"

func ChangeOwner(cur realm, newOwner address) {
    if !cur.IsCurrent() { panic("spoofed realm") }
    if cur.Previous().Address() != owner { panic("not the owner") }
    owner = newOwner
    chain.Emit("OwnershipChange", "newOwner", newOwner.String())
}
```

The ABCI block envelope carries event type, the emitting `pkg_path`, the emitting `func`, and the key/value attrs — all indexable. Be generous; emit for any externally-observable state change.

## Access control idioms

Four common shapes:

### `cur.Previous().Address()` — generic caller identity

```go
func AdminOnly(cur realm) {
    if !cur.IsCurrent() { panic("spoofed realm") }
    if cur.Previous().Address() != admin { panic("permission denied") }
    // ...
}
```

Works for any caller (EOA or another realm). Use when the calling identity is what matters.

### `runtime.OriginCaller()` — the tx-signing EOA

Returns the public address of the account that signed the transaction, regardless of intermediaries. Use when **the original user identity** is what matters (not whichever realm forwarded to you).

```go
import "chain/runtime"

func SignedByEOA() address {
    return runtime.OriginCaller()
}
```

### `runtime.AssertOriginCall()` — strict EOA-only gate

Panics if anything but a direct `MsgCall` from an EOA reaches this function. Stricter than `IsUserCall()`: rejects all `MsgRun` invocations. Use for governance-only or strictly-no-intermediary functions.

### Allowlist via `ownable` / `authz`

For multi-admin or rotateable ownership, use the canonical packages: `gno.land/p/nt/ownable` (single-owner pattern) or `gno.land/p/moul/authz` (richer auth schemes).

## Cost-aware design

### Storage deposit drives state-shape decisions

Storing data locks GNOT tokens proportional to the bytes. Deleting refunds them. The price is a global parameter governed by GovDAO — the default is **100 ugnot per byte** (`storagePriceDefault` in `gno.land/pkg/sdk/vm/params.go`), but treat the live value as authoritative since GovDAO can change it.

Consequences for design:

- **Small structs over fat structs.** Each field costs.
- **Scalar where possible.** A single-key `avl.Tree` is over-typed (and over-paid); collapse to a scalar.
- **Schedule cleanup.** Expired listings, old proposals, dead sessions — implement removal. Refunded deposits cover storage cost; clean realms run cheaper.
- **Cleanup-as-revenue.** Deposits are tracked per-realm, not per-user; whoever deletes the data gains the released deposit. You can design realms where third parties profit from sweeping expired state.

The `-max-deposit` flag on `MsgCall` / `MsgRun` / `AddPkg` caps the GNOT the caller is willing to lock. Realms that overshoot may have their tx rejected.

### Gas drives execution-path decisions

Gas is a measure of computational and storage cost. Every read, write, allocation, and AVL traversal counts. Optimizations that matter:

- **Lazy AVL allocation** (see § Lazy initialization).
- **Bounded loops.** Loops over user-controlled inputs can spike gas; bound the iteration count.
- **Index over scan.** If you'll query by both "name" and "id," maintain two AVLs:
  ```go
  var (
      usersById   avl.Tree // map id   -> user
      usersByName avl.Tree // map name -> id
  )
  ```
- **Avoid cross-realm reads in hot paths.** Readonly taint forces copies that cost gas; cache the result.
- **Events instead of dynamic Render().** If consumers need to see "X happened," emit an event instead of rendering it on demand.

## Common library imports

Packages that survived the `examples/quarantined/` cull — the test-13 safe-list (PR #5726) that moved unaudited/personal-namespace packages to `examples/quarantined/`. Re-verify against the current master tree before relying on an import; the split moves over time:

| Need | Canonical import |
|---|---|
| AVL trees (ordered keyed state) | `gno.land/p/nt/avl/v0` |
| B+tree (alternative for ordered keyed state) | `gno.land/p/nt/bptree/v0` |
| Render-path routing (mux) | `gno.land/p/nt/mux/v0` |
| Realm-path parsing | `gno.land/p/moul/realmpath` |
| Ownership / single-owner pattern | `gno.land/p/nt/ownable` |
| Authorization patterns | `gno.land/p/moul/authz` |
| Pagination | `gno.land/p/jeronimoalbi/pager` |
| DAO primitives | `gno.land/p/nt/commondao/v0` |
| Fungible tokens (canonical safe example) | `gno.land/p/demo/tokens/grc20` |
| Non-fungible tokens | `gno.land/p/demo/tokens/grc721` |

Prefer these over re-implementation — they're reviewed, used, and stable.

**Don't** import `gno.land/r/tests/vm/test20` — deliberately insecure test fixture exporting `PrivateLedger`. Using it in production code = instant compromise (see `security.md` § Encapsulation pattern).

## Naming and documentation

Exported names are part of the **on-chain ABI**. Renaming `func Buy` to `func Purchase` breaks every existing caller — same severity as a breaking API change. Pin names before deploy.

**Godoc is user-facing documentation**, not internal commentary. The gnoweb `:help` view and agent introspection both read godoc. Treat it as the API doc.

```go
// Buy purchases one widget for `price` ugnot.
//
// Sends the payment via OriginSend; reverts if the amount is incorrect
// or stock is depleted. EOA-only — `maketx run` callers are rejected.
func Buy(cur realm) { ... }
```

## `Render(path string) string` conventions

- **No state mutation.** Read-only by convention; not enforced by the chain.
- **Pick one routing pattern and stick with it** within a realm. Mixing mux and `realmpath.Parse` is confusing.
- **Return a useful 404** on unknown paths — `> [!WARNING]\n> Path not found` or similar, not panic.
- **Pre-render expensive paths into events** if the same view will be requested often.

See `render.md` for the markdown extension surface and `chain/markdown` sanitization.

## Testing — three flavors

| Flavor | Filename | Use for |
|---|---|---|
| Unit test | `*_test.gno` | Standard `testing.T` style — assert function-level behavior |
| File test | `*_filetest.gno` | Full file with a comment block declaring expected output. Best for spec-shaped tests |
| Integration | external (`gnodev`, txtar) | End-to-end against a real chain; catches `MsgCall` vs `MsgRun` semantic mismatches |

For payment-accepting realms, **always include a test that asserts the payment guard rejects `IsUserRun` callers**. Use `testing.NewCodeRealm()` to simulate an intermediate attacker realm.

See `build.md` for filetest layout and authoring patterns.

## Operational anti-patterns

Not security bugs, but audit-worthy patterns that should block deployment until resolved or explicitly documented. Mirror in `security.md` § Operational signals.

- **No exit for accumulated funds.** Implement guarded withdraw, auto-forward, or document burn-on-receipt.
- **No admin rotation.** Hardcoded `admin address` with no `TransferAdmin(cur realm, ...)`. Key loss = permanent loss of privileged operations.
- **Dead function-valued state.** A `var cb func(...)` writable but never invoked — Class 4 latent.
- **Silent type-assertion fallbacks.** `n, _ := current.(int)` swallows assertion failure; future state-shape changes corrupt the read silently. Panic on failure, or use a typed wrapper.
- **Over-typed state for single-key use.** Single-key `avl.Tree` is over-engineered — collapse to a scalar or expose multi-key API.
- **Unbounded admin inputs.** `Stock(cur realm, qty int)` accepting negative `qty` is an admin footgun. Bound at the function boundary.

## `gno fmt`

Required. Match surrounding style.

## Cross-references

- `interrealm.md` — the model these patterns work *with* (crossing functions, borrow rules, `cur` capability, readonly taint)
- `security.md` — the bug classes these idioms avoid (Classes 1a/1b/2/3/4, (A)/(B)/(C) safety hypothesis, encapsulation pattern)
- `stdlib.md` — APIs the patterns rely on (`chain.Emit`, `avl.Tree`, `banker.OriginSend`, `chain/runtime`)
- `render.md` — Render() and gnoweb-specific patterns
- `build.md` — project setup, testing flavors, Go-Gno compatibility
- `memory.md` — typed value model and persistence semantics

## Source

- `docs/resources/effective-gno.md` in the gnolang/gno repo — canonical best-practice doc.
- `docs/resources/gas-fees.md` and `docs/resources/storage-deposit.md` — cost model.
- `docs/resources/gno-interrealm-v2.md` § Guidelines — the spec author's mental model.
- `examples/gno.land/` — observed conventions in deployed realms (post-test-13 quarantine).
