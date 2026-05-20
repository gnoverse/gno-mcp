# Audit: p/nt/commondao/v0 — with gno skill loaded

## Summary

This package is a DAO primitives library. As a `/p/` it cannot itself hold persistent state, so the worst-case blast radius lives in **calling realms** that pass it untrusted `ProposalDefinition`, `Executable`, `MemberStorage`, or `ProposalStorage` instances. The dominant risk is the `Executable` interface plus `ExecFunc func(realm) error` callback path — a textbook security.md §3 (behavior substitution via user-supplied callbacks) shape — followed by interface-typed state holders that re-enter on every method call (§2) and pluggable storage factory functions that survive in DAO state (§3 latent).

Verdict: safe-to-import **only if** importing realm's callers cannot influence which `ProposalDefinition` / `Executor` / `MemberStorage` ends up wired into the DAO. Wrap, never expose `Propose`/`Execute` directly to public input.

## Files reviewed

- `doc.gno` — declares "v0 — Unaudited" status, useful for severity calibration.
- `commondao.gno` — `CommonDAO` struct, `New`, `Propose`, `Vote`, `Withdraw`, `Execute`. Contains the `fn(cross)` call site.
- `commondao_options.gno` — 10 `With*` options including `WithMemberStorage`, `WithActiveProposalStorage`, `WithFinishedProposalStorage` (caller-supplied interface-typed state).
- `proposal.gno` — `Proposal`, `ProposalDefinition` interface, `Validable`, `Executable`, `CustomizableVoteChoices`, `ExecFunc func(realm) error`, `MustExecute`, `MustValidate`. Core of the callback risk.
- `voting_context.gno` — read-only context passed into `Tally`; minimal surface.
- `record.gno` — `VotingRecord`, `Vote{Context any}`, tally helpers (`CollectVotes`, `FindMostVotedChoice`, etc.).
- `member_storage.gno` — `MemberStorage` interface, `memberStorage` impl, `ReadonlyMemberStorage` wrapper.
- `member_group.gno` — `MemberGroup` interface with `SetMeta(any)/GetMeta() any` — opaque mutable cell.
- `member_grouping.gno` — `MemberGrouping` interface, `memberGrouping` impl with stored `createStorage func(...)`.
- `member_grouping_options.gno` — `UseStorageFactory(fn func(group string) MemberStorage)`, `WithGroups(...)`.
- `proposal_storage.gno` — `ProposalStorage` interface, default `bptree`-backed impl.

`exts/definition/options.gno` is named in security.md §3 as a known `WithExecutor` site but is out of audit scope here.

## References loaded

- `SKILL.md` — router; identified that §3 and §2 of `security.md` are the lead classes for "function-typed fields or parameters" and "caller-supplied interface stored in state."
- `references/security.md` — full read. Used class numbers §2, §3, §5, §9; operational signals (function-valued state never invoked, opaque `any` cells, silent type assertions).
- `references/patterns.md` — anti-patterns at-a-glance (§154 "caller-supplied interface{} or func() in permission-gated paths") and operational anti-patterns (dead function-valued state, over-typed state, silent type-assertion fallbacks).
- `references/interrealm.md` — peeked: confirmed `cur realm` is **illegal in `/p/` packages**, which makes the `func(realm) error` callback signature in a `/p/` package noteworthy in its own right. See finding 5.

Did not load `render.md` (package has no `Render`), `stdlib.md` (no `banker`/`OriginSend` here — package is realm-agnostic), `future.md` (no in-flight migration is decisive for this verdict).

## Findings

### Finding 1 — `Executable` + `ExecFunc func(realm) error` runs caller-supplied code under DAO `Execute` authority — YELLOW (RED in any realm that exposes Execute to public input)

**Location**: `proposal.gno:54` (`ExecFunc func(realm) error`), `proposal.gno:102-105` (`Executable` interface), `commondao.gno:255-260` (`fn(cross)` call site), `proposal.gno:129-142` (`MustExecute`).

**Class**: security.md §3 (behavior substitution via caller-supplied callbacks). Also referenced by name in §3's "Status: ACTIVE. Live in `examples/gno.land/p/nt/commondao/v0/exts/definition/options.gno` (WithExecutor, ExecFunc func(realm) error) and surrounding code."

**What I see**: `Execute` resolves `Definition()` to an `Executable`, calls `e.Executor()` to obtain an `ExecFunc`, and invokes `fn(cross)` — a bare cross-call into whatever realm declared the closure. The `ProposalDefinition` arrived via `Propose(creator, d)` from the caller, so `d` and its `Executor()` body are caller-controlled. Nothing in the package constrains which realm `fn`'s closure was minted in, what authority it carries, or what its body does.

**Why it's a problem (or not)**: at the `/p/` boundary this is by design — the package can't know what's safe in the calling realm. But every importing realm that exposes `dao.Propose` (or any wrapper) to untrusted callers is shipping a §3 hazard with the DAO's reputation behind it. After-call state mutation in `Execute` (`p.status = StatusExecuted`, `dao.activeProposals.Remove`, `dao.finishedProposals.Add` on lines 273-275) happens *after* `fn(cross)` returns, so a re-entrant `fn` that calls back into `dao.Execute(p.id)` while the original is mid-flight could see the same proposal still in `activeProposals` and re-execute. No in-flight flag, no generation counter, no CEI. **This is §2 (interface re-entrancy) layered on §3.**

**Recommendation**: package-level docs should aggressively call out that `ProposalDefinition` must come from a trusted realm and that re-entry from `Executor()` into the same DAO is undefined. Strongly consider:
- Moving the `dao.activeProposals.Remove(p.id) / finishedProposals.Add(p)` and the status assignment **before** the `fn(cross)` call (CEI ordering) — even if `fn` fails, the proposal is finished by then. Trade-off: status reflects "executed" before execution completes. Could use an intermediate `StatusExecuting`.
- Or add an in-flight flag on `Proposal` cleared on entry to `Execute`. Cite `security.md` §2 fix (c).

The same hazard exists in `MustExecute` (proposal.gno:139, `fn(cross)`) but that's a free helper, not the gated path. Less severe.

### Finding 2 — `MemberStorage` / `ProposalStorage` / `MemberGrouping` are caller-supplied interfaces stored in `CommonDAO` state and re-invoked from every gated method — YELLOW

**Location**: `commondao.gno:32-34` (interface fields on `CommonDAO`), `commondao_options.gno:59-66` (`WithMemberStorage`), `:71-78` (`WithActiveProposalStorage`), `:83-89` (`WithFinishedProposalStorage`). Invocation sites: `commondao.gno:195` (`dao.Members().Has(member)` inside `Vote`), `:163-167` (`Get` inside `GetProposal`), `:174-186` (storage mutation inside `Withdraw`), `:225-275` (storage operations inside `Execute`).

**Class**: security.md §2 (interface-based re-entrancy).

**What I see**: `MemberStorage`, `ProposalStorage` and `MemberGrouping` are interfaces. Their methods are called from `Vote`, `Withdraw`, `Execute`, etc., interleaved with state mutations on `*Proposal`. A malicious storage impl can re-enter the calling realm and mutate state between, say, `dao.activeProposals.Get(id)` (line 199) and `p.record.AddVote(...)` (line 212).

**Why it's a problem (or not)**: at `/p/` level the storage interface is necessary for extensibility (the doc explicitly says "Custom storage implementations can be used to store proposals in a different location"). The risk is again realized only in calling realms — and only if the realm exposes the option setters to untrusted input. The default in-package impls (`memberStorage`, `proposalStorage`, `memberGrouping`) are safe.

**Recommendation**: document that `MemberStorage` / `ProposalStorage` / `MemberGrouping` implementations must come from trusted realms; storage methods will be invoked from within state-mutating DAO operations and must not re-enter. Cross-reference `security.md` §2.

### Finding 3 — `UseStorageFactory(fn func(group string) MemberStorage)` persists a caller-supplied closure into `memberGrouping.createStorage` — YELLOW

**Location**: `member_grouping_options.gno:9-22`, `member_grouping.gno:52` (`createStorage func(group string) MemberStorage` field), `:71` (invocation site inside `Add`).

**Class**: security.md §3, latent variant ("function-valued state that's writable but currently never invoked" — here it IS invoked, when groups are added at runtime). Also patterns.md operational anti-pattern "Dead function-valued state" (slightly stronger — this one is live).

**What I see**: The factory closure is stored on the grouping struct and called every time `Add(name)` runs. The factory returns a `MemberStorage` that becomes the storage of the new group — see §2 for what happens with that returned interface. Compounds two §3 surfaces: the function pointer itself and the interface it returns.

**Why it's a problem (or not)**: as with the rest of the package, only realized in calling realms that expose group creation. But this is the *only* place a closure becomes long-lived persistent state inside the package's own types (`createStorage` lives on `memberGrouping`, which is held by `memberStorage.grouping`, which is held by `CommonDAO.members`). If the calling realm persists a `CommonDAO`, the closure persists, and §9 (attached-method authority grant) becomes relevant for whatever realm the closure was minted in.

**Recommendation**: explicitly document that `UseStorageFactory` accepts a closure that will be invoked during `Add(group)` with whatever realm authority the surrounding `dao.Members().Grouping().Add(...)` call has. Importing realms should treat the factory as trusted-code-only. Cite `security.md` §3 and §9.

### Finding 4 — `MemberGroup.SetMeta(any) / GetMeta() any` is an opaque mutable cell carrying arbitrary caller values across the DAO surface — YELLOW

**Location**: `member_group.gno:18-25` (interface), `:69-77` (impl). `Vote.Context any` (`record.gno:31-37`) is the same shape on the vote record.

**Class**: not a `security.md` numbered class on its own; combination of §3 (whatever ends up in the cell can be a function value), §6 / §8 (whatever ends up in the cell can be a slice/pointer with cross-realm taint), patterns.md "silent type-assertion fallbacks" (consumers will inevitably do `meta.(MyType)` and either swallow or panic).

**What I see**: `SetMeta` accepts `any`, `GetMeta` returns `any`. The doc comment on `Vote.Context` does call out the indirect-mutation risk ("Warning: When using context be careful if references/pointers are assigned to it..."); the doc on `SetMeta` does not.

**Why it's a problem (or not)**: classic "trust the field's type — wait, the type is `any`" trap. A function value stored here becomes a §3 surface that's not visible at any signature. Cross-realm references stored here will hit readonly-taint (§8) or pointer-stealing (§6) and panic at use, which is at least loud — but the failure mode is "your DAO unable to read its own group meta."

**Recommendation**: mirror the `Vote.Context` doc-comment warning onto `MemberGroup.SetMeta`. Better: provide a typed wrapper helper in this package (e.g. `SetWeights(g, map[address]uint64)` ↔ `GetWeights(g)`) for the documented use cases (voting weights / tiers / roles per the existing doc). Cite `security.md` §3 latent and §8.

### Finding 5 — `ExecFunc func(realm) error` declared in a `/p/` package is a smell (not a bug today) — YELLOW

**Location**: `proposal.gno:54`.

**Class**: not a bug class. Defensive observation that builds on `interrealm.md` "cur realm is illegal in /p/ packages."

**What I see**: `ExecFunc` is a function *type*, not a function definition, so the `/p/` rule against declaring crossing functions is not violated — but the type's only purpose is to be implemented by realms (`/r/`) and invoked under `cross`. The package is essentially defining a contract that "the implementer must be a crossing function." This is fine, but the `func(realm)` syntactic form here is not the same as `func(cur realm)` — `realm` is the *type*, no `cur` keyword needed in a function-type declaration. Worth verifying that calling realms that implement `Executor()` return a closure whose underlying function has the `(cur realm, ...)` signature, otherwise the `fn(cross)` call site (`commondao.gno:259`) will not be a real crossing call.

**Why it's a problem (or not)**: not a bug; an audit-readability issue. The implementer contract is not stated in the doc and the type signature alone can't enforce it. The skill flagged this surface as §3 — the spec wrinkle is bonus.

**Recommendation**: add a doc comment to `ExecFunc` stating "Implementations must be crossing functions (`func(cur realm, ...) error`). The DAO will invoke the function via a `cross` call." Cite `interrealm.md`.

### Finding 6 — `CommonDAO.parent *CommonDAO` and `children list.IList` form a mutable hierarchy with no caller-gating; `TopParent()` recurses unbounded — YELLOW

**Location**: `commondao.gno:30-31`, `:91-99`, `:103-109`.

**Class**: not a `security.md` class. Operational concern.

**What I see**: `WithParent(p *CommonDAO)` (commondao_options.gno:35) and `WithChildren(...)` (line 42) set the topology at construction. There is no method to mutate parent/children after construction, but `Children() list.IList` returns the underlying list interface — depending on `list.IList`'s API, the caller might be able to mutate the children list directly. Also `TopParent()` is a naive recursion: a cycle in the parent chain (set up via two `WithParent` calls plus pointer fiddling, or via direct field aliasing across two DAOs) infinite-loops the call stack until gas runs out.

**Why it's a problem (or not)**: parent-chain cycles are constructable only if a caller holds two `*CommonDAO` pointers and somehow swaps fields; the package doesn't expose `SetParent`, so this requires misuse. Worth a one-line bound or a cycle check.

**Recommendation**: convert `TopParent()` to an iterative loop with a step cap, or check for cycles. Either is a one-liner. Document that `Children()` returns a live reference; consider returning a read-only view.

## Notes

Things general-Gno-knowledge flags that the skill did NOT cover:

- **`/p/` packages declare contracts for `/r/` realms to implement.** The skill discusses `/p/` vs `/r/` and the §3 shape, but doesn't have a dedicated section on the auditing posture for a `/p/` library that hands out callback-typed interfaces. The right mental model here is "this package's risk surface is whatever every importing realm fails to wrap." That framing is implicit in §3 status notes but not centralized.
- **The doc.gno header literally says "v0 - Unaudited."** Worth elevating from a code comment to the audit verdict's calibration. The package is honestly labeling itself.
- **`bptree.NewBPTree32()` vs `avl.Tree`.** The skill's patterns.md says "use `avl.Tree` for persisted keyed state" but this package uses `bptree` instead. Cursory check (the package is `gno.land/p/nt/bptree/v0`) suggests it's an alternative ordered-map impl. Not flagged in the skill — worth knowing whether `bptree` has the same realm-persistence guarantees as `avl.Tree` before signing off on the storage layer. Out of scope for this audit.
- **`MustPropose`/`MustValidate`/`MustExecute` as panic-on-error wrappers.** Idiomatic Gno; the skill doesn't explicitly call out the "Must*" convention as safe but it's worth a one-liner since it's everywhere in the ecosystem.
- **Severity rollup**: 0 RED, 6 YELLOW. The RED would only land if a calling realm exposes `Propose`/`Execute` directly to public input — which is outside the package boundary.
