# Stdlib API surface

> **Category: API reference.** Designed for **live introspection**, not static restatement.

## Purpose

Stdlib APIs change. A static reference goes stale; an agent that emits stale API names produces broken code. This reference is intentionally thin — it names the packages and the security-relevant primitives, then defers to live introspection for current API details.

## How to use this reference

For exact, up-to-date API surface, the gnomcp design (see `contribs/gnomcp/` skills' parent) exposes introspection tools that query the chain directly:

- **`gno_inspect <pkgPath>`** — per-function godoc and function signatures, queried via `vm/qdoc` against the current chain.
- **`gno_render <pkgPath>`** — rendered docs / examples if the package exposes `Render()`.
- **`gno_eval <pkgPath> <expr>`** — evaluate an expression in the package context.

When emitting code that calls stdlib, **prefer querying live** over recalling from training data. This reference tells you *what to query*, not what to copy.

## Packages a builder will encounter

### `std`

The core runtime interface — caller identity, addresses, coins.

Load via `gno_inspect std`. Security-relevant primitives:

- **`std.PreviousRealm()`** / **`std.CurrentRealm()`** — caller-identity primitives. Only `PreviousRealm()` shifts on explicit cross-calls. See `interrealm.md`.
- **`Realm.IsUserCall()`** vs **`Realm.IsUser()`** vs **`Realm.IsUserRun()`** — the trichotomy. `IsUser()` accepts MsgRun ephemeral realms (`/e/` paths) and is **insufficient for payment guards**. See `security.md` §1, §7.
- **`std.Emit`** — event emission for off-chain consumers.

### `std/banker`

Coin handling.

Load via `gno_inspect banker`. Security-relevant:

- **`banker.OriginSend()`** — coins from the originating tx. Guard with `IsUserCall()`. See `security.md` § payment-guard.
- **`banker.SendCoins(from, to, coins)`** — outbound transfer.
- Successor `realm.SentCoins()` (PR #5039) — frame-local; re-entrancy-safe. See `future.md` until adoption sweep lands.

### `std/avl` / `gno.land/p/demo/avl`

The canonical persisted keyed collection. Load via `gno_inspect avl`.

Use this (or `gno.land/p/nt/bptree/v0`) instead of Go's `map`. Map iteration is non-deterministic → consensus halt risk. See `patterns.md` § Why these patterns / Determinism.

### `std/crypto`

Hashes, address parsing. Load via `gno_inspect crypto` when needed.

### `std/testing`

Test scaffolding (`t.RunTransaction`, txtar helpers). Load via `gno_inspect testing` when writing tests.

## What NOT to import

- **Go host stdlib**: `net`, `os`, `syscall`, `time` (with non-deterministic functions like `time.Now()` resolved deterministically from block time), `runtime` (the Go one — `runtime` in Gno is `std.runtime`, different package).
- Anything implying network or filesystem access at runtime — the VM has no host access.
- See `patterns.md` § Why these patterns / No host access for the driver.

## Cross-references

- `interrealm.md` — how `std.PreviousRealm` / `std.CurrentRealm` interact with crossing semantics
- `security.md` — `IsUser()` vs `IsUserCall()` bug class, banker guard pattern
- `patterns.md` — AVL-over-map, testing idioms
- `future.md` — `realm.SentCoins()`, anything pending merge

## Source

Live introspection via gnomcp's `gno_inspect` against the current chain. Master tree under `gnovm/stdlibs/` is the source of truth when offline.
