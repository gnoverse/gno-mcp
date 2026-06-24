# Stdlib API surface

> **Category: API reference.** Designed for **live introspection**, not static restatement.
> **Authoritative spec**: `docs/resources/gno-stdlibs.md` in the `gnolang/gno` repo. This reference names the packages and security-relevant primitives, then defers to live introspection for current API details.

## Purpose

Stdlib APIs change. A static reference goes stale; an agent that emits stale API names produces broken code. This reference is intentionally thin — package names, the security-relevant primitives, and the gotchas — then defers to `gno_read`'s outline for exact signatures.

## How to use this reference

For exact, up-to-date API surface, the gnomcp design exposes introspection tools that query the chain directly:

- **`gno_read <pkgPath>`** — the default outline: per-file function signatures + docs against the current chain (add `symbols=[...]` for specific bodies).
- **`gno_render <pkgPath>`** — rendered docs / examples if the package exposes `Render()`.
- **`gno_eval <pkgPath> <expr>`** — evaluate an expression in the package context.

When emitting code that calls stdlib, **prefer querying live** over recalling from training data. This reference tells you *what to query*, not what to copy.

## Package map

The 0.9 release reorganized the stdlib into a `chain/...` family. Old code that imported `std` directly may need migration.

| Import path | Purpose |
|---|---|
| `chain` | Core types — `Coin`, `Coins`, `Emit`, `PackageAddress`, `CoinDenom` |
| `chain/runtime` | Chain/realm observation — `AssertOriginCall()`, `ChainID()`, `ChainDomain()`, `ChainHeight()`, `GetSessionInfo()` |
| `chain/runtime/unsafe` | Stack-walking caller identity — `OriginCaller()`, `CurrentRealm()`, `PreviousRealm()`, `OriginSend()`. |
| `chain/banker` | Coin handling — `NewBanker()`, `SendCoins()`, `GetCoins()`, `IssueCoin()`, `RemoveCoin()` |
| `chain/markdown` | Markdown escaper natives — internal building blocks; realm authors use the `gno.land/p/nt/markdown/sanitize/v0` helpers layered on top (see `render.md`) |
| `testing` | Test scaffolding — `SkipHeights`, `SetOriginCaller`, `SetOriginSend`, `IssueCoins`, `SetRealm`, `NewUserRealm`, `NewCodeRealm` |

## Uverse — builtins always in scope

These do not require an import:

- **`address`** — bech32 address type. Methods: `IsValid() bool`, `String() string`.
- **`realm`** — the capability token surfaced as the `cur realm` parameter on crossing functions. Methods: `Address()`, `PkgPath()`, `Previous()`, `IsCurrent()`, `IsCode()`, `IsUser()`, `IsUserCall()`, `IsUserRun()`, `IsEphemeral()`, `String()`. See `interrealm.md`.
- **`cross(rlm)`** — uverse function used to mark cross-calls: `bakery.MakeBread(cross(cur), "flour", "water")`. See `interrealm.md`.
- **`revive(fn)`** — boundary-aware recover; currently enabled only in test/filetest mode.

## Caller-identity primitives

Two surfaces, both supported, with different safety properties:

### Modern: `cur realm` parameter

A crossing function `func F(cur realm, ...)` receives a typed capability token. Use this whenever possible:

```go
func Buy(cur realm) {
    if !cur.IsCurrent() { panic("spoofed realm") }
    if !cur.Previous().IsUserCall() { panic("not an EOA call") }
    coins := unsafe.OriginSend()   // import "chain/runtime/unsafe"
    // ...
}
```

The `IsCurrent()` check is the authentication primitive — see `security.md` Class 2 (designation-forgery).

### Stack-walking: `chain/runtime/unsafe`

```go
import "chain/runtime/unsafe"

func F() {
    if unsafe.PreviousRealm().Address() != owner {
        panic("caller isn't the owner")
    }
}
```

The `unsafe` package name reflects what these primitives do: **stack-walking** that returns the realm prior to the most recent boundary, regardless of which function you're in. **Calling `unsafe.PreviousRealm()` inside a non-crossing function does NOT identify the immediate caller** — it returns whatever was previous at the last realm boundary, possibly an unrelated frame upstream.

Use `chain/runtime/unsafe` only when you genuinely need the stack-walking form:
- A non-crossing function with no `cur` in scope (remembering it returns the last-boundary realm, not the direct caller — see the warning above).
- Code that deliberately wants the realm before the most recent boundary.

`OriginCaller`, `CurrentRealm`, `PreviousRealm`, and `OriginSend` are exported only from `chain/runtime/unsafe` — there is no `runtime.OriginCaller()` or `banker.OriginSend()`. Inside a crossing function, `cur.Previous()` under `cur.IsCurrent()` identifies the immediate caller; `unsafe.PreviousRealm()` stack-walks to the last boundary instead (the distinction above).

### The trichotomy — `IsUserCall` vs `IsUserRun` vs `IsUser`

| Predicate | EOA via `MsgCall` | EOA via `MsgRun` (ephemeral) | Realm via `MsgCall` |
|---|---|---|---|
| `IsUserCall()` | ✓ | ✗ | ✗ |
| `IsUserRun()` | ✗ | ✓ | ✗ |
| `IsUser()` | ✓ | ✓ | ✗ |

`IsUser()` is **insufficient for payment guards** — the `MsgRun` ephemeral realm can consume `OriginSend()` and forward control. Use `IsUserCall()` for payment-gated paths. See `security.md` § Payment-guard canonical pattern.

## `chain` — coin types and events

### `Coin` / `Coins`

```go
type Coin struct {
    Denom  string
    Amount int64
}

type Coins []Coin   // set semantics (no duplicate denoms)
```

`Coins` can be carried with transactions made by user addresses or realms. Specific banker subtypes manipulate them subject to access rights.

### `Emit`

Events log on-chain activity for off-chain consumers. The `Emit()` function takes an event-type string followed by an even number of key/value-pair strings:

```go
chain.Emit("OwnershipChange", "oldOwner", oldAddr.String(), "newOwner", newAddr.String())
```

Each event is recorded in the ABCI results of the block; off-chain services consume them via `/block_results` RPC. Events are pkg-path-stamped (the emitting realm) and func-stamped (the emitting function), both available in the ABCI envelope.

## `chain/banker` — coin handling

The Banker handles balance changes of native coins: issuance, transfers, burning. It exposes four subtypes via `NewBanker()`:

| Subtype | What it grants |
|---|---|
| `BankerTypeReadonly` | Read-only access to coin balances |
| `BankerTypeOriginSend` | Full access to coins sent with the transaction that called the banker |
| `BankerTypeRealmSend` | Full access to coins the realm itself owns (including those sent with the tx) |
| `BankerTypeRealmIssue` | Can issue new coins |

Security-relevant primitives:

- **`OriginSend()`** — coins included with the originating transaction. Pair with `cur.Previous().IsUserCall()` and an amount check. See `security.md` § Payment-guard.
- **`SendCoins(from, to, coins)`** — outbound transfer.
- **`IssueCoin(addr, denom, amount)`** — only `BankerTypeRealmIssue`.

**Gotcha**: a realm that consumes `OriginSend()` but never `SendCoins`/`Withdraw` locks funds at the realm address. See `security.md` § Operational signals.

## `chain/markdown` — render sanitization natives

Low-level escaper natives (`EscapeInline`, `EscapeTitle`, `PercentEncodeURL`, `StripBidiAndZeroWidth`, `NormalizeBreaks`, …) for markdown emitted by `Render()`. These are internal building blocks — realm authors use the `gno.land/p/nt/markdown/sanitize/v0` helpers (`sanitize.InlineText`, `.Block`, `.URL`, …) that wrap them with the policy layer. See `render.md` § Sanitizing untrusted text and the extension surface.

## `testing` — test scaffolding

Filetest helpers for realm tests. Common entries:

- `SetOriginCaller(addr)` — fake the EOA for a test.
- `SetOriginSend(coins)` — fake an inbound `OriginSend` envelope.
- `SetRealm(realm)` — install a fake realm on the **calling frame** (popped on return — call it directly in each test, never via a helper; see `build.md`).
- `NewUserRealm(addr)` / `NewCodeRealm(pkgPath)` — construct test realm values.
- `SkipHeights(n)` — advance the simulated block height.
- `IssueCoins(addr, coins)` — mint coins to an address for setup.

See `build.md` for filetest layout and authoring patterns.

## Common community packages (kept in `examples/`)

The packages below survived the test-13 quarantine (`examples/quarantined/` got everything else) — safe to import:

| Purpose | Import path |
|---|---|
| AVL tree (canonical persisted keyed collection) | `gno.land/p/nt/avl/v0` |
| B+tree (alternative for ordered keyed state) | `gno.land/p/nt/bptree/v0` |
| Render-path routing (mux) | `gno.land/p/nt/mux/v0` |
| Realm-path parsing | `gno.land/p/moul/realmpath` |
| Ownership / single-owner pattern | `gno.land/p/nt/ownable/v0` |
| Authorization patterns | `gno.land/p/moul/authz` |
| Pagination | `gno.land/p/jeronimoalbi/pager` |
| DAO primitives | `gno.land/p/nt/commondao/v0` |
| Fungible tokens (canonical safe example) | `gno.land/p/demo/tokens/grc20` |
| Non-fungible tokens | `gno.land/p/demo/tokens/grc721` |

**Use `avl.Tree` (or `bptree`) instead of Go's `map`** for growing keyed state — a persisted map rewrites wholesale on every mutation, and its insertion-order iteration is an impl detail, not an ordering contract. (Iteration is deterministic, so it's a gas/design issue, not a consensus risk.) See `patterns.md` and `memory.md` § Map iteration order.

## What NOT to import

- **Go host stdlib**: `net`, `os`, `syscall`, `runtime` (the Go one), and anything implying network or filesystem access. The VM has no host access.
- `time.Now()` and similar non-deterministic functions are resolved from block time — usable, but pay attention to determinism (see `patterns.md`).
- **`gno.land/r/tests/vm/test20`** — deliberately insecure GRC20 fixture exporting `PrivateLedger`. Importing it in production code = instant compromise. See `security.md` § Encapsulation pattern.

## Cross-references

- `interrealm.md` — how `cur` and `chain/runtime` interact with crossing semantics; capability-token methods in detail
- `security.md` — payment-guard pattern, predicate trichotomy, attacker-class derivations
- `patterns.md` — AVL-over-map, event-emission idioms, testing patterns
- `render.md` — `chain/markdown` sanitization, `Render()` output
- `build.md` — `testing` package usage and filetest layout

## Source

- `docs/resources/gno-stdlibs.md` in the gnolang/gno repo — full prose reference.
- Live introspection via `gno_read` (outline) against the current chain.
- Master tree under `gnovm/stdlibs/` is the source of truth when offline.
