# Audit: shop realm — with gno skill loaded

## Summary
Block. Payment guard is exploitable today (§1); a latent callback-substitution vector is also present.

## Findings

### Finding 1 — Payment bypass via `IsUser()` + `OriginSend()` — RED
**Location**: shop.gno:25-31 (`Buy`)
**Class**: §1 `OriginSend()` + `IsUser()` — payment bypass via `MsgRun` (also §7 trichotomy)
**What I see**:
```go
if !runtime.PreviousRealm().IsUser() { panic("EOA only") }
coins := banker.OriginSend()
if coins.AmountOf("ugnot") != price { panic("incorrect amount") }
inventory.Set(itemSKU, n-1)
```
**Why it's a problem**: `IsUser()` accepts the user's ephemeral `gno.land/e/g1<user>/run` realm (MsgRun). That realm can consume the `OriginSend` envelope before forwarding to `Buy`, so the price check passes against an empty envelope and the item is granted without payment. Receipt invariant broken.
**Recommendation**: replace with `runtime.PreviousRealm().IsUserCall()` per the canonical payment-guard pattern in `security.md` (top of file). Track `realm.SentCoins()` migration noted in `future.md` for the longer-term fix.

### Finding 2 — Caller-supplied callback stored in state — YELLOW
**Location**: shop.gno:52-59 (`afterBuy`, `SetCallback`)
**Class**: §3 Behavior substitution via user-supplied callbacks
**What I see**:
```go
var afterBuy func(realm)
func SetCallback(cur realm, cb func(realm)) {
    if runtime.PreviousRealm().Address() != admin { panic("admin only") }
    afterBuy = cb
}
```
**Why it's a problem**: a `func(realm)` value captured from the admin's realm is persisted into shop's state. Whoever holds the admin key can swap in arbitrary code that, when invoked, runs under shop's storage authority. Admin-gated reduces the blast radius to a trusted-admin assumption — but the trust assumption is not documented, and there's no use site for `afterBuy` in this file (dead hook). If a future change wires `afterBuy(cur)` into `Buy`, this becomes an active §3 hazard.
**Recommendation**: (a) if the hook is unused, delete it (YAGNI); (b) if needed, prefer a typed interface with a path-allowlisted implementation over `func(realm)`; (c) document the admin-trust assumption inline. See `security.md` §3 and `patterns.md` for typed-interface alternatives.

### Finding 3 — `PreviousRealm().Address() == admin` inside a crossing function — GREEN (noted)
**Location**: shop.gno:42 (`Stock`), shop.gno:55 (`SetCallback`)
**Class**: not a violation; flagged because §7 / SKILL.md "Anti-pattern reflex" warns about `PreviousRealm()` misuse
**What I see**: `if runtime.PreviousRealm().Address() != admin { panic("admin only") }` inside `func Stock(cur realm, ...)` and `func SetCallback(cur realm, ...)`.
**Why it's not a problem**: both are crossing functions (have `cur realm` first param). `PreviousRealm()` does correctly shift to the immediate caller on a crossing call, so identifying the admin EOA is valid here. Compare with the anti-pattern: same check inside a non-crossing function would be a security bug.
**Recommendation**: no action. Optional polish — combine with an `IsUserCall()` assertion if admin must always be an EOA via MsgCall (defense-in-depth against any future routing through a third realm).

### Finding 4 — `Render` output is trusted/static — GREEN
**Location**: shop.gno:61-65
**Class**: render.md XSS axis (not loaded; surface-level check)
**What I see**: Render concatenates only static strings and integers from internal state. No untrusted user input is rendered.
**Why it's not a problem**: no path for injected markdown or HTML; gnoweb output is safe.
**Recommendation**: none.

## Notes
- `afterBuy` callback is declared and writable but never invoked. Either delete or wire-and-document — current state is a footgun waiting on a future commit.
- `admin` is a hard-coded address literal; consider whether transfer-of-admin is needed before launch (out-of-scope for the bug-class audit).
- `Stock` accepts negative `qty` (decrements inventory). Not a security bug, but lets the admin "unstock" silently. Worth a sanity bound if that's not intentional.
