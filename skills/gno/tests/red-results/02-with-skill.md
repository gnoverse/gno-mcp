# Audit: manager realm — with gno skill loaded

## Summary
BLOCK. Manager stores a third-party realm type whose method drains manager's treasury to attacker-chosen addresses; also forwards `cur` instead of `cross`, leaks open registration, and lacks payment guards.

## Findings

### Finding 1 — Attached-method privilege grant (widget.Widget stored in manager state) — RED
**Location**: manager.gno:14 (`widgets = avl.NewTree()`), :24 (`widgets.Set(name, w)`), :35-36 (`w := v.(widget.Widget); w.Tip(cur)`)
**Class**: security.md §9 (attached-method authority grant); interrealm.md "Implicit borrow-cross on methods"
**What I see**: manager persists `widget.Widget` (an `r/`-imported struct) into its AVL state, then later invokes `w.Tip(...)` — a method declared in `r/example/widget`. The body of `Tip` runs `banker.GetBanker(BankerTypeRealmSend)` and `bnk.SendCoins(runtime.CurrentRealm().Address(), w.Owner, coins)`.
**Why it's a problem**: Storing an `r/` type and calling its method gives the *declaring* realm's code authority over the *storing* realm's state/coins via the implicit storage-realm borrow-switch. `runtime.CurrentRealm().Address()` resolves to manager's address under that borrow, so widget's method drains manager's treasury to `w.Owner` — an address the attacker controls because `Register` is open (see Finding 3). The compiler will not catch this; per Jae Kwon, *"don't attach objects/receivers to your realm unless you know it's safe."*
**Recommendation**: Do not store `widget.Widget` (an `r/` type) in manager state. Either (a) re-model as a manager-local struct that holds only the data fields (`Name`, `Owner std.Address`) — composition with pure values — or (b) audit every method on `widget.Widget` as if it were manager's own code and accept the third-party privilege grant explicitly. Reference: security.md §9.

### Finding 2 — `cur realm` disclosure to third-party realm — RED
**Location**: manager.gno:36 (`w.Tip(cur)`)
**Class**: security.md §5 (cur disclosure)
**What I see**: manager forwards its own `cur realm` parameter into a method declared in another realm instead of using `cross`.
**Why it's a problem**: `cur` is the proof of a deliberate crossing into manager; passing it onward to widget code leaks manager's realm-context authority outward. The convention is that `cur` is consumed at the function boundary — subsequent external calls use `cross`.
**Recommendation**: If a call into widget is genuinely needed, use `cross`: e.g., declare `Tip` as a free function and call `widget.Tip(cross, w)`, or otherwise re-architect so manager never forwards `cur`. Reference: security.md §5.

### Finding 3 — Open registration enables payment redirection — RED
**Location**: manager.gno:23-26 (`Register`)
**Class**: security.md § operational (admin/input bounds) + composes with Finding 1
**What I see**: `Register` accepts any caller and any name; `widget.New(name)` records `Owner: runtime.PreviousRealm().Address()`, i.e. the caller. There is no name-collision check (`widgets.Set` will silently overwrite a prior entry — see Finding 5) and no caller gating.
**Why it's a problem**: Any user can register a widget under any name with themselves as `Owner`. Combined with Finding 1, every subsequent `Promote(...)` routes both `OriginSend()` *and* coins charged against manager's realm address to that attacker. The `admin` variable captured in `init()` is never used to gate writes.
**Recommendation**: Either gate `Register` on `admin` (with a crossing function check `if std.CurrentRealm().Address() != admin && runtime.PreviousRealm().Address() != admin { panic(...) }` — but better, do the check in the crossing fn against `runtime.PreviousRealm()`), or document the open-registration trust model and remove the unused `admin`. Also reject name collisions: `if _, ok := widgets.Get(name); ok { panic("exists") }`.

### Finding 4 — `Promote` has no payment guard / no `IsUserCall` — RED
**Location**: manager.gno:30-37
**Class**: security.md §1 / §7 (payment guard trichotomy)
**What I see**: `Promote` does no caller-identity check. `Tip` consumes `banker.OriginSend()` with no `runtime.PreviousRealm().IsUserCall()` guard anywhere on the call path.
**Why it's a problem**: `MsgRun` ephemeral realms can drain `OriginSend()` and re-invoke; without `IsUserCall()`, the payment-receipt invariant is broken (canonical §1). Even setting that aside, there is no bound on what's sent and no check that anything is sent at all.
**Recommendation**: At the manager boundary, gate with `if !runtime.PreviousRealm().IsUserCall() { panic("EOA via MsgCall only") }`. Validate `coins.AmountOf("ugnot") > 0` (or a configured floor) before forwarding. Reference: security.md §1 + §7, canonical pattern at top of security.md.

### Finding 5 — Silent overwrite + unchecked type assertion — YELLOW
**Location**: manager.gno:25 (`widgets.Set(name, w)`), :35 (`w := v.(widget.Widget)`)
**Class**: security.md § operational (silent fallback)
**What I see**: `Set` overwrites any existing key; the type assertion has no `, ok` and will panic on a future state-shape change.
**Why it's a problem**: First is an input-validation gap (Finding 3 leverages it). Second is brittle — if `widgets` ever holds anything else (migration, multi-type), `Promote` panics for everyone.
**Recommendation**: Reject duplicates in `Register`. Use `w, ok := v.(widget.Widget); if !ok { panic("type") }` in `Promote`.

### Finding 6 — `admin` set in `init()` then never used — YELLOW
**Location**: manager.gno:13, :17-19
**Class**: security.md § operational (hardcoded admin + dead code)
**What I see**: `admin` is captured at deploy time but no function consults it, and there is no `TransferAdmin`.
**Why it's a problem**: Either it's dead code (delete) or it's a forgotten gate (Finding 3 wants it). Either way it's a signal of incomplete intent.
**Recommendation**: Delete `admin`, or wire it into `Register` / a `TransferAdmin(cur realm, next std.Address)` crossing function.

### Finding 7 — `Tip` is declared as a crossing method but called with `cur` — YELLOW
**Location**: widget.gno:28 (`func (w *Widget) Tip(cur realm)`); manager.gno:36 (`w.Tip(cur)`)
**Class**: interrealm.md "Crossing functions" / spec guideline "methods should generally be non-crossing"
**What I see**: `Tip` takes `cur realm`, making it a crossing function; callers should pass `cross`. Manager passes `cur`. Additionally, the receiver `w` is a *value* type but `Tip` is declared on `*Widget` — `v.(widget.Widget)` extracts a value, and `w.Tip(...)` auto-takes its address against the local copy, so any state changes in `Tip` would be lost (here they aren't, but the shape is wrong).
**Why it's a problem**: Composes with Finding 2. The widget author chose a non-idiomatic shape (crossing method) that is hard to use safely; manager copied it wrong. Either way the cross-call discipline is broken.
**Recommendation**: This is a widget-side smell, but manager should not import this method at all (see Finding 1). If kept, the only correct call shape is `(&w).Tip(cross)` against a persisted pointer — which still doesn't fix §9.

## Notes
- `Render` is benign — no user input is interpolated; `itoa(widgets.Size())` is safe. No render.md concerns.
- Per skill router note, I did not load `future.md`; none of the findings hinge on in-flight PRs. `realm.SentCoins()` (PR #5039) would be the modern replacement for `OriginSend()` here but does not change the §9 / §5 verdict.
- The root cause across Findings 1, 2, 4 is the same: manager treats widget as a trusted utility library when it is in fact a third-party `/r/` realm. The fix is structural — don't import realm types into your state.
