# Audit: shop realm — baseline (no skill loaded)

## Summary
Block — multiple RED issues, including a payment-bypass auth bug and an arbitrary-code-execution callback.

## Findings

### Finding 1 — IsUser() guard with OriginSend payment — RED
**Location**: shop.gno:25-31 (`Buy`)
**What I see**: `Buy` gates on `runtime.PreviousRealm().IsUser()` and then consumes `banker.OriginSend()` to charge `price` ugnot.
**Why it's a problem**: `IsUser()` returns true for ephemeral `maketx run` realms as well as direct EOA calls. A `maketx run` script can intercept the origin-send envelope and consume the coins before/instead of reaching `Buy`, or can wrap the call so the user pays but the buyer-identity logic is wrong. The correct guard for any function that reads `OriginSend()` is `runtime.PreviousRealm().IsUserCall()` (rejects `maketx run`).
**Recommendation**: Replace `IsUser()` with `IsUserCall()`.

### Finding 2 — Attacker-controlled callback stored and (presumably) invoked — RED
**Location**: shop.gno:52-59 (`SetCallback`, `afterBuy`)
**What I see**: `afterBuy` is a package-level `func(realm)` set via `SetCallback`. The comment says it's "called after each Buy", though `Buy` does not currently invoke it.
**Why it's a problem**: (a) Even with the admin guard, storing a foreign closure means every future `Buy` would re-enter attacker realm code while holding shop state — classic reentrancy / privilege-escalation surface; the admin can be socially engineered or compromised once and the realm is permanently hijacked. (b) The current `Buy` doesn't actually call `afterBuy`, so the feature is dead code that still ships an admin-settable hook — pure attack surface with zero functionality. (c) Storing a closure captured from another realm has lifetime/serialization implications in Gno [uncertain on exact semantics, but cross-realm closure storage is at minimum a design smell].
**Recommendation**: Remove `SetCallback`/`afterBuy` entirely (YAGNI). If a hook is genuinely needed, use an event/emit pattern or a typed interface restricted to a vetted package path, and never invoke it while shop invariants are mid-mutation.

### Finding 3 — Coins received but never withdrawn / no treasury accounting — RED
**Location**: shop.gno:28-31 (`Buy`)
**What I see**: `Buy` verifies `OriginSend` equals `price` ugnot but never moves the coins anywhere. There is no `GetBanker(...).SendCoins(...)`, no admin withdraw function, no treasury address.
**Why it's a problem**: The ugnot accumulates in the realm address with no way to retrieve it. Either (a) this is intended and the coins are effectively burned/locked — should be explicit — or (b) the author forgot to wire a payout. Either way, money in with no money out is a bug worth blocking on.
**Recommendation**: Add an admin-only `Withdraw` (or auto-forward to a treasury address in `Buy`), and document the intended destination.

### Finding 4 — Admin is a hardcoded constant with no rotation — YELLOW
**Location**: shop.gno:19
**What I see**: `admin` is a fixed `std.Address` literal; no `TransferAdmin` function.
**Why it's a problem**: Key loss = permanent loss of `Stock` ability. Not catastrophic for buyers (they can still `Buy` until inventory hits 0), but the realm becomes un-restockable.
**Recommendation**: Add a guarded `TransferAdmin(cur realm, next std.Address)` if rotation is in scope; otherwise document the trade-off.

### Finding 5 — Inventory keyed by single SKU but typed as a tree — YELLOW
**Location**: shop.gno:15, 20, 32, 37, 45, 47, 62
**What I see**: `inventory` is an `avl.Tree` but only ever stores one key, `itemSKU = "widget"`.
**Why it's a problem**: Over-engineered for current functionality (YAGNI) — a plain `var stock int` would do. Not a security issue, but indicative of unfinished design; reviewers should clarify whether multi-SKU is in scope before this ships.
**Recommendation**: Either collapse to a scalar `int` or actually expose multi-SKU APIs. Don't ship the middle ground.

### Finding 6 — Type assertion on nil interface relies on zero-value fallback — YELLOW
**Location**: shop.gno:32-34, 45-47, 62-63
**What I see**: `current, _ := inventory.Get(...)` then `n, _ := current.(int)`. When the key is absent, `current` is `nil` and `n` defaults to `0`. The second `_` swallows the assertion failure.
**Why it's a problem**: Works correctly today, but if anyone ever `Set`s a non-int value at `itemSKU` (e.g. a future refactor), `Buy`/`Stock` silently treat stock as 0 instead of panicking. Violates "errors should never pass silently."
**Recommendation**: Either panic on assertion failure (`n := current.(int)` without the second return) for fail-fast behavior, or use a strongly-typed wrapper around the tree.

### Finding 7 — `Stock` accepts negative `qty` — YELLOW
**Location**: shop.gno:41-48
**What I see**: `Stock(cur realm, qty int)` has no sign check; `n+qty` can underflow stock to negative.
**Why it's a problem**: Admin-only, so blast radius is limited, but a negative stock makes `Buy`'s `n == 0` guard wrong (negatives would pass and decrement further). Defense in depth.
**Recommendation**: `if qty <= 0 { panic("qty must be positive") }` — or accept it intentionally as a "remove stock" feature and rename.

## Notes
- I did not verify exact Gno API names: `banker.OriginSend()` is called as a package function here rather than via `GetBanker(...).GetCoins(realmAddr)`; I believe `std.OriginSend()` is the more common form [uncertain — could be a recent rename or a fixture-only stub].
- The `cur realm` parameter on every exported function suggests these are crossing functions; the file does not import a package boundary marker, so I'm trusting the `cur realm` convention as the indicator.
- `SetCallback` storing `func(realm)` across realms: I'm not 100% sure Gno permits persisting closures captured from a foreign realm at all — if it doesn't, Finding 2(c) becomes "this will panic at runtime" rather than a reentrancy concern. Worth confirming against the spec.
- No `Render` ID/path handling — fine for a single-item shop, but if multi-SKU is added, `Render(path)` should switch on the path.
