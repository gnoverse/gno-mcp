# Audit: manager realm — baseline (no skill loaded)

## Summary
Block — multiple high-severity issues let any user hijack widgets and redirect funds; access control is missing.

## Findings

### Finding 1 — Widget hijack via unrestricted Register overwrite — RED
**Location**: `manager.gno:23-26`
**What I see**: `Register(cur, name)` is callable by anyone and does `widgets.Set(name, w)` without checking whether `name` is already taken. The new widget is constructed via `widget.New(name)`, where `Owner` is captured from `runtime.PreviousRealm().Address()` — i.e. the caller becomes Owner.
**Why it's a problem**: An attacker can call `Register(cur, "alice-widget")` to overwrite an existing widget, replacing the legitimate Owner with their own address. Subsequent `Promote` calls then route tips to the attacker instead of the original owner.
**Recommendation**: Reject `Register` when `widgets.Has(name)`; or gate Register behind the `admin` set in `init`; or namespace entries by caller address.

### Finding 2 — Unused `admin`, no access control anywhere — RED
**Location**: `manager.gno:13, 17-19`
**What I see**: `admin` is captured at `init` from `PreviousRealm().Address()` but is never read. No function checks caller identity.
**Why it's a problem**: The realm advertises a notion of administration via the `admin` global, but every mutating entrypoint (`Register`, `Promote`) is fully public. This is either dead code masking a missing guard, or an actual TODO never wired up. Combined with Finding 1, it is the root cause of the hijack.
**Recommendation**: Either delete `admin` (YAGNI) or actually enforce it on `Register` / privileged ops.

### Finding 3 — `w.Tip(cur)` is not crossed into the widget realm — RED
**Location**: `manager.gno:36`
**What I see**: `Tip` is declared as a crossing function in widget (`func (w *Widget) Tip(cur realm)`), but manager calls it as `w.Tip(cur)` without the `cross` keyword. [uncertain] — exact semantics of calling a crossing method across realms without `cross` need verification, but per project CLAUDE.md, realm shift requires explicit `fn(cross, ...)`.
**Why it's a problem**: If no shift occurs, the `banker.GetBanker(BankerTypeRealmSend)` inside `Tip` runs under manager's storage authority and `runtime.CurrentRealm()` returns manager — meaning manager's own realm balance funds the tip, paid to a Caller-controlled `w.Owner`. Although `OriginSend()` credits matching coins back to manager (net-zero per call), this still means manager's treasury is being puppeted by an unauthenticated caller — and any imbalance (e.g., the receiving realm has unrelated prior balance, or future widget upgrade pays out more than OriginSend) becomes an immediate drain. If a shift does happen, then widget — not manager — pays, but the receiver is still arbitrary attacker-controlled.
**Recommendation**: Do not delegate banker calls to a third-party realm method at all. Re-implement the tip path inside manager with an explicit Owner lookup and explicit `IsUserCall()` + `OriginSend` matching check. Never call a foreign realm's method that internally invokes a banker with your realm's authority.

### Finding 4 — Trusting third-party `widget.New` for ownership capture — RED
**Location**: `manager.gno:24`
**What I see**: Owner is set inside widget.New via `PreviousRealm().Address()` rather than determined by manager.
**Why it's a problem**: widget is an external, mutable third-party realm. A future version of `widget.New` could record any owner it wants (e.g. always the widget author) without manager noticing — manager has surrendered control of an authorization-critical field to a dependency it does not own. This is a supply-chain risk on a hot path.
**Recommendation**: Manager should construct/own the Owner value itself (capture caller in manager, then build the Widget locally or pass owner explicitly).

### Finding 5 — No `IsUserCall` guard on entrypoints that handle value — YELLOW
**Location**: `manager.gno:23, 30`
**What I see**: Neither `Register` nor `Promote` checks `runtime.PreviousRealm().IsUserCall()`. `Promote` is the one that ultimately triggers `OriginSend`-based logic.
**Why it's a problem**: Per project guidance, any path that consumes `OriginSend()` must gate on `IsUserCall()` (not `IsUser()`), otherwise an intermediate `maketx run` realm can consume the envelope before the check.
**Recommendation**: Add `assertUserCall()` at the top of `Promote` (and `Register` if it ever takes value).

### Finding 6 — Storing struct value, mutation via pointer receiver won't persist — YELLOW
**Location**: `manager.gno:25, 35-36`
**What I see**: `widgets.Set(name, w)` stores by value; later retrieval `v.(widget.Widget)` then calls `Tip` via pointer receiver. The pointer points to a local copy.
**Why it's a problem**: Today `Tip` doesn't mutate `w`, so no functional bug. But if widget v2 adds mutation in `Tip` (e.g., a tip counter), manager's stored copy will silently fail to update — a future foot-gun.
**Recommendation**: Store `*widget.Widget` (or document the value-store contract explicitly and never call pointer-receiver methods that mutate).

### Finding 7 — `Render` parameter ignored — GREEN
**Location**: `manager.gno:39`
**What I see**: `Render(_ string)` ignores its path argument.
**Why it's a problem**: Not a bug; just no per-path views. Fine for a stub.
**Recommendation**: None.

## Notes
- The most urgent fix is Finding 1 + Finding 2 (anyone can hijack any widget by overwrite). Even without any banker concerns, this realm has broken authorization.
- Finding 4 is a category bug: relying on third-party realm internals for security-critical decisions. Treat external realms as untrusted code on every upgrade.
- [uncertain] Exact behavior of calling `Tip(cur)` without `cross` keyword — worth confirming against `docs/resources/gno-interrealm.md`. Either way, the design (delegating banker ops to a foreign realm) is wrong.
- `itoa` is fine; nothing to flag.
