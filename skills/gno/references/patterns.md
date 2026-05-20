# Patterns and idioms

> **Category: conventions / how to write Gno.** Update when a recommended pattern becomes the community standard or is deprecated. Anti-patterns are catalogued in `security.md`; this file is the "do this" companion.

## Purpose

Idiomatic shapes that fit Gno's grain. Drawn primarily from `docs/resources/effective-gno.md` and observed conventions in `examples/gno.land/`. When this reference and `effective-gno.md` disagree, `effective-gno.md` wins — file an update.

## Why these patterns — the blockchain-context drivers

Most Gno idioms are not stylistic preferences. They are responses to constraints that vanilla Go programmers don't face. Knowing the **why** lets an agent generalize the patterns to novel situations rather than pattern-matching them blindly.

| Force | Constraint it imposes | Patterns it drives |
|---|---|---|
| **Determinism** | Every node must produce the same result. Diverging on a single byte halts consensus. | `avl.Tree` over `map` (map iteration order); avoid `math.MinInt` / `MaxInt` (architecture-dependent); avoid float-format reliance; never let map iteration affect execution. |
| **Gas metering** | Every read, allocation, and store op costs gas the caller pays. | Lazy allocation — don't create state until needed; minimize cross-realm reads (readonly taint forces copies); single top-level struct for state (one root pointer instead of many globals); avoid unnecessary AVL traversals. |
| **Refcount persistence** | Realm finalization assigns object IDs and refcounts to every reachable object. Bugs in refcount accounting break the chain. | Avoid cyclic references in persisted state; don't re-create objects you just deleted in the same tx (refcount delete-then-recreate is a known bug class); prefer composition with `/p/` (pure) types over `/r/` types in state. |
| **Cross-realm authority** | Realm boundaries are security boundaries. Method receivers attach authority. | Prefer non-crossing methods; never accept caller-supplied `func()` or `interface{}` in permission-gated paths; explicit `cross` for every external state mutation. |
| **Verifiability** | Source is on-chain. Humans and agents read it. There is no off-chain doc to drift. | Godoc as user-facing API doc; explicit guards over clever invariants; simple type signatures; one obvious way to do each thing. |
| **Tx atomicity** | A transaction either fully succeeds or fully reverts. Half-completed state mutations don't ship. | `panic` for unrecoverable conditions (cleanly aborts the tx); CEI ordering (checks → effects → interactions) since `panic` after state mutation still reverts; `Must*` panic-wrappers are safe. |
| **No host access** | The gno VM has no `os`, `net`, `time.Now()` (deterministically), `syscall`. | No external API calls in realm code; events (`std.Emit`) for off-chain consumers; off-chain tooling reads events + state, doesn't poll. |

Cross-reference: `interrealm.md` for the spec-level mechanics behind cross-realm authority and readonly taint; `security.md` for the bug-class consequences when these drivers are ignored.

## 1. `/r/` vs `/p/` vs `/e/`

| Path | Use when |
|---|---|
| `/r/` (realm) | Persistent state is required. Exported crossing functions become the public API. Reachable via `MsgCall`. |
| `/p/` (pure package) | Reusable, stateless logic. Importable by anyone — `/p/` cannot import `/r/`. No crossing functions allowed. Behavior must be identical regardless of which realm calls it (the "p-code copied into r-code behaves the same" rule). |
| `/e/` (ephemeral) | Created on-the-fly by `MsgRun` for one transaction. Not authored by hand; users get one when running a `main()` against the chain. |

**Default to `/p/`.** Move to `/r/` only when persistent state is needed. Public utility types and interfaces belong in `/p/` so any realm can compose with them.

## 2. State shape

**Use package-level globals.** Embrace them — they ARE the persistence layer. Each `var X = …` becomes a heap item; new and modified objects under the realm-storage-context are persisted at transaction finalization.

**Why**: in vanilla Go, globals are an anti-pattern because they hide state and complicate testing. In Gno, globals ARE the durable state — every package-level `var` is a root in the persistence graph. The realm-finalization mechanism (realm.go) assigns object IDs to reachable objects rooted at globals; what isn't reachable is GC'd. So globals aren't side-effect-prone — they're how persistence works. Treat them as fields on a singleton.

```go
package shop

import "gno.land/p/demo/avl"

var (
    inventory = avl.NewTree()
    admin     std.Address
)
```

**Lazy initialization** for heavy state. If a sub-collection is expensive to create and not always needed, allocate on first use rather than at `init()`. **Why**: every reachable object at finalization gets an ID and a refcount, and every cross-tx read pays gas. Lazy init keeps the cold-path object set smaller, reducing both deploy gas and steady-state read costs.

```go
var members *avl.Tree

func ensureMembers() *avl.Tree {
    if members == nil {
        members = avl.NewTree()
    }
    return members
}
```

**Use `avl.Tree` (or `avl.Trees` for ordered iteration) for keyed collections.** Go `map` does not persist deterministically and will eventually break. **Why**: Go's `map` has non-deterministic iteration order — different nodes processing the same tx can iterate in different orders and produce different downstream state. `avl.Tree` is deterministically ordered, persists cleanly through realm finalization, and supports range traversal — three properties Go's `map` lacks. This is a consensus-correctness concern, not a style preference: a `map`-based realm CAN halt the chain.

`gno.land/p/nt/bptree/v0` (B+tree) is an accepted alternative for ordered keyed state — used by `commondao` and other community packages. Either is fine; pick one and stay consistent within a package. The same determinism + persistence rationale applies.

**Wrap state in a single top-level struct when a future upgrade is anticipated.** Proxy + Implementation patterns (PR #4816 line, see `future.md`) serialize cleanly from a single struct; scattered globals make migration harder.

**Don't compose with `r/` types you don't trust.** Storing a value of an external `r/` type in your realm's state grants that realm's code execution privileges over your storage (`security.md` §9). Prefer composition with `/p/` types only.

## 3. Crossing-function discipline

```go
// Public, called by users via MsgCall.
// Crossing functions take `cur realm` first.
func Buy(cur realm, sku string) {
    requireUserCall()           // gate first
    requirePayment()            // then payment
    buy(sku)                    // delegate to internal helper
}

// Internal, non-crossing. Does the work.
func buy(sku string) {
    // ...
}
```

Conventions:
- **`MsgCall` only dispatches to crossing functions.** Any function meant to be called externally MUST take `cur realm` first.
- **Methods should generally be non-crossing.** Per the spec author: methods are "pre-bound to an object — a quasi-realm in itself." A method that crosses into its declaring realm is "intrusive, but sometimes desired."
- **Authority lives in the crossing-function wrapper, not the helper.** The crossing function gates (caller checks, payment validation) and then delegates. Helpers don't repeat the gates.
- **`/p/` packages can't declare crossing functions** (compiler-enforced). If you need state-mutating cross calls, the realm is the right home.

## 4. Error vs panic

**Use `panic` for unrecoverable contract state.** Gno's convention departs from Go here. Reserve returned `error` values for caller-checkable conditions where the caller has a meaningful recovery path.

Examples:
- `Buy()` rejecting wrong payment amount → `panic("incorrect amount")`. Caller can't recover from a wrong payment; reject the transaction.
- `Get(id)` returning `(Item, error)` when the item may legitimately not exist → return error.

`panic` aborts the transaction cleanly. The chain rolls back state; the user sees the panic message.

**The state-revert guarantee is `panic`-specific.** A `panic` in any frame during tx execution reverts all state mutations performed during the tx — the `TransactionStore` boundary makes this atomic. **Returning an `error` does NOT revert state**; the caller has to choose to revert (typically by panicking themselves). For unrecoverable conditions where you want the tx to fail-and-revert, panic is the only correct choice. The `Must*` wrapper convention exists precisely so callers can opt into this behavior.

**`Must*` panic-wrapper convention.** Idiomatic Gno (and the Go stdlib that inspired it): when a function returns an `error`, expose a `Must` variant that panics on error. Pattern:

```go
func Execute(...) error { ... }
func MustExecute(...) { if err := Execute(...); err != nil { panic(err) } }
```

The caller picks: error-returning for code that branches on failure, `Must*` for "I want this to succeed or kill the tx." Both are safe; the panic-wrapper is not a smell.

## 5. `init()` and access control setup

`init()` runs during deploy under `StageAdd`. `runtime.PreviousRealm()` returns the *deployer* — the only chance a realm has to record who deployed it. Use this to set the initial admin.

```go
var admin std.Address

func init() {
    admin = runtime.PreviousRealm().Address()
}
```

After deploy, the deployer is no longer accessible; the realm must remember anything deploy-time it needs. See `docs/resources/effective-gno.md` § Understand the importance of `init()`.

## 6. Imports

**Prefer `/p/` over `/r/`.** A `/p/` import is pure; you inherit no authority and grant none. An `/r/` import means your realm may store values of types defined by that realm — and methods on those types run with your storage authority (`security.md` §9). Import `/r/` only when you need the imported realm's state to be observable or callable.

**Use canonical primitives**. Don't re-implement these — the existing implementations are reviewed, novel re-implementations are not.

| Need | Canonical import | Alt / notes |
|---|---|---|
| Fungible tokens | `gno.land/p/demo/tokens/grc20` | — |
| Non-fungible tokens | `gno.land/p/demo/tokens/grc721` | — |
| AVL trees (ordered keyed state) | `gno.land/p/demo/avl` | `gno.land/p/nt/bptree/v0` is an accepted alternative |
| Render-path routing | `gno.land/p/demo/mux` | for `Render(path)` dispatch |
| Pagination | `gno.land/p/jeronimoalbi/pager` | community standard at scaffold-time |
| Ownership / permissions | `gno.land/p/nt/ownable` | — |
| DAO primitives | `gno.land/p/nt/commondao/v0` | currently labeled "v0 — Unaudited" by its authors; review before reliance |
| Authorization patterns | `gno.land/p/moul/authz` | — |

## 7. Testing

Three flavors. Pick the right one.

| Flavor | Filename | Use for |
|---|---|---|
| Unit test | `*_test.gno` | Standard `testing.T` style — assert function-level behavior |
| File test | `*_filetest.gno` | Full file with a comment block declaring expected output. Best for spec-shaped tests (interrealm semantics, edge cases) |
| Integration | external (`gnodev`, txtar) | End-to-end against a real chain; catches `MsgCall` vs `MsgRun` semantic mismatches |

For realms accepting payment, **always include a `_test.gno` that asserts the payment guard rejects `IsUserRun` callers**. Without this, the `IsUser()` vs `IsUserCall()` regression class is invisible to unit tests that only check `IsUser()` callers.

Run `gnodev` interactively when designing the cross-realm surface — the txtar log shows what callers see in practice.

## 8. Events

Emit `std.Emit` events for any state change off-chain consumers would care about. They cost gas but are the canonical way to expose state changes to indexers without forcing them to re-render the whole realm.

```go
std.Emit("ItemSold", "sku", sku, "buyer", runtime.PreviousRealm().Address().String())
```

Off-chain consumers (tx-indexer, gnoweb activity views, agentic-AI tools) read these events; in their absence they have to diff full state snapshots. Be generous.

## 9. Naming and documentation

Exported names are part of the **on-chain ABI**. Renaming `func Buy` to `func Purchase` breaks every existing caller — same severity as a breaking API change.

**Godoc is user-facing documentation**, not internal commentary:

```go
// Buy purchases one widget for `price` ugnot.
//
// Sends the payment via OriginSend; reverts if the amount is incorrect
// or stock is depleted. EOA-only — `maketx run` callers are rejected.
func Buy(cur realm) { ... }
```

The gnoweb `:help` view and agent introspection both read godoc. Treat it as the API doc.

## 10. `Render(path string) string` conventions

- **No state mutation.** Read-only by convention; the chain does not enforce.
- **Pick one routing pattern and stick with it within a realm.** Mixing `mux` and `realmpath.Parse` within the same realm is confusing.
- **Return a useful 404 on unknown paths** — `> [!WARNING]\n> Path not found` or similar, not panic.
- **Pre-render expensive paths into events or indexed state** if the same view will be requested often. `Render()` is gas-cheap but bandwidth costs accrue at the consumer.

See `render.md` for the markdown extension surface.

## Anti-patterns at a glance

Negation of class catalog in `security.md`. Don't:

- Use `IsUser()` near `OriginSend` — use `IsUserCall()` (`security.md` §1, §7).
- Accept caller-supplied `interface{}` or `func()` in permission-gated paths (§2, §3).
- Store a value of an `r/` type without auditing every method on it (§9).
- Element-mutate a slice that crossed a realm boundary (§8).
- Forward `cur realm` as an argument to another call (§5).
- Use the `crossing()` body marker (pre-Gno-0.9 syntax, §10).
- Use `map` for persisted keyed state — `avl.Tree`.

## Operational anti-patterns

Not security bugs, but audit-worthy patterns that should block deployment until resolved or explicitly documented:

- **No exit for accumulated funds.** A realm that consumes `OriginSend()` but provides no `Withdraw` or auto-forwarding path locks funds in the realm address. Either implement a guarded withdraw, auto-forward to a treasury, or document the burn-on-receipt intent.
- **No admin rotation.** A hardcoded `admin std.Address` with no `TransferAdmin` function. Key loss = permanent loss of privileged operations. Either ship rotation or document the trade-off.
- **Dead function-valued state.** A `var cb func(...)` that's set-able but never invoked. See `security.md` §3 latent — this is YELLOW until wired, becomes RED on the commit that wires it.
- **Silent type-assertion fallbacks.** `n, _ := current.(int)` swallows the assertion failure; future state-shape changes corrupt the read silently. Either panic on failure (`n := current.(int)`) or use a typed wrapper.
- **Over-typed state for single-key use.** An `avl.Tree` storing one key is over-engineered. Either collapse to a scalar or expose the multi-key API. Don't ship the middle.
- **Unbounded admin inputs.** `Stock(cur realm, qty int)` accepting negative `qty` is an admin footgun. Bound the input at the function boundary.

## `gno fmt`

Required. Match surrounding style.

## Cross-references

- `interrealm.md` — the model these patterns work *with*
- `security.md` — anti-patterns this file's idioms avoid
- `stdlib.md` — APIs the patterns rely on (`std.Emit`, `avl.Tree`, `banker.OriginSend`, `runtime.PreviousRealm`)
- `render.md` — Render() and gnoweb-specific patterns
- `future.md` — patterns that change with PR #5669 and other in-flight work

## Source

`docs/resources/effective-gno.md` — canonical best-practice doc (10+ sections).
`docs/resources/gno-interrealm.md` § Guidelines — the spec author's mental model.
`examples/gno.land/` — observed conventions in deployed realms.
`.mynote/gno-agentic/reference/14b-realm-doc-audit.md` — survey of doc/idiom adoption in existing realms.
