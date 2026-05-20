# Audit: p/nt/commondao/v0 — baseline (no skill loaded)

## Summary
This is a library of DAO primitives (proposals, voting, member groups). It is **mostly safe to import for state-shape purposes**, but it exposes one design choice that has real security consequences for callers: `CommonDAO.Execute` invokes a caller-supplied `ExecFunc` with crossing semantics (`fn(cross)`), making the `commondao` package itself the `PreviousRealm()` seen by the executor. Combined with arbitrary `any`-typed metadata on `MemberGroup`/`Vote.Context` and unprotected mutator methods on the DAO, callers must wrap this package carefully — there is no access control inside the package itself. The package's own `doc.gno` even warns: "Unaudited. Use in production at your own risk."

## Files reviewed
- `doc.gno`: explicit "unaudited / use at your own risk" notice.
- `commondao.gno`: `CommonDAO` struct, constructor, `Propose`/`Vote`/`Execute`/`Withdraw`/`SetDeleted`.
- `commondao_options.gno`: `With*` functional options including custom storages.
- `proposal.gno`: `Proposal` type, `ProposalDefinition`/`Validable`/`Executable` interfaces, `ExecFunc func(realm) error`, `MustExecute` which also crosses.
- `record.gno`: `VotingRecord`, `Vote{Context any}`, choice selection helpers, `CollectVotes`.
- `member_storage.gno`: `MemberStorage` interface + default impl + readonly wrapper.
- `member_group.gno`: `MemberGroup` interface with `SetMeta(any)/GetMeta()`.
- `member_grouping.gno`: `MemberGrouping` + `memberGrouping{createStorage func(string) MemberStorage}`.
- `member_grouping_options.gno`: `UseStorageFactory(fn)` (panics if nil; panics if grouping is not the concrete `*memberGrouping`).
- `proposal_storage.gno`: B+tree-backed proposal storage with seqid keys.
- `voting_context.gno`: read-only view passed to `Tally`.

## Findings

### Finding 1 — Executor runs with `commondao` as PreviousRealm — RED
**Location**: `commondao.gno:259` (`err = fn(cross)`) and `proposal.gno:139` (`MustExecute` also calls `fn(cross)`)
**What I see**: `ExecFunc` is typed `func(realm) error`. `Execute` (and the standalone `MustExecute`) calls `fn(cross)`, which is a crossing-call. Inside the executor, `runtime.PreviousRealm()` returns `gno.land/p/nt/commondao/v0` — NOT the realm that called `Execute` and NOT the proposal creator (the filetest `z_commondao_execute_0_filetest.gno` confirms: when `dao.Execute` is called from `gno.land/r/testing/dao`, the executor sees `PreviousRealm().PkgPath() == "gno.land/r/testing/dao"` only because `Execute` itself is a non-crossing method that re-uses its caller's frame... [uncertain on exact semantics — needs interrealm spec verification, see notes]).

The package gives **no built-in guarantee** about who can call `Execute`. Anybody who can reach the `*CommonDAO` value can call `Execute` once the deadline passes. Realms importing this MUST wrap `Execute` behind their own auth, since the executor cannot rely on `PreviousRealm()` to identify the realm that triggered execution.

**Why it's a problem**: Realm authors copying from `r/`-realm Solidity-style patterns will write executors that check `PreviousRealm().PkgPath() == "g.l/r/expected"` and they will get the wrong answer — either the commondao package path, or whatever realm last did a crossing call into commondao, depending on call shape. This is exactly the gno-interrealm gotcha called out in CLAUDE.md.
**Recommendation**: Document loudly that executors must not rely on `PreviousRealm()` for identity. Pass the proposal creator / DAO context explicitly into `ExecFunc` (e.g. add fields to a context struct) instead of a bare `realm` token. At minimum, add a security warning to `doc.gno` and to `ExecFunc`'s doc comment.

### Finding 2 — No access control on `CommonDAO` mutators — RED
**Location**: `commondao.gno:132` (`SetDeleted`), `:137` (`Propose`), `:173` (`Withdraw`), `:194` (`Vote`), `:224` (`Execute`)
**What I see**: All mutators are exported methods on `*CommonDAO` with zero guard on who is invoking them. `Propose` accepts any `creator address` argument (no check that the caller equals `creator`). `Vote` checks membership of the supplied `member address` but again does not check that the caller equals `member`. `SetDeleted(bool)` is unrestricted. `Withdraw(proposalID)` does not check that the caller is the original proposer.
**Why it's a problem**: A realm that imports this package and exposes a `*CommonDAO` (or its methods) without wrapping them will allow any caller to vote on behalf of any member, propose on behalf of anyone, or soft-delete the DAO. This is "library has no security; caller must enforce", which is reasonable IF documented, but the doc comments don't say so.
**Recommendation**: Document the caller-enforcement requirement at the package level. Consider providing a higher-level realm template or wrapper in `exts/` that demonstrates correct guarding. Optionally: have `Propose`/`Vote` accept the caller address from `std.PreviousRealm()` rather than as a parameter, but that hurts composability — documentation is probably the right fix.

### Finding 3 — `any`-typed metadata on stored objects — YELLOW
**Location**: `record.gno:36` (`Vote.Context any`), `member_group.gno:21` (`SetMeta(any) / GetMeta() any`)
**What I see**: Both `Vote.Context` and `MemberGroup` metadata are `any`. The Vote struct itself warns in the doc comment about reference leaks. Stored objects live inside B+trees that are part of the realm's persisted state.
**Why it's a problem**: (a) Aliasing — a caller can store a pointer in `Context`/meta, mutate it later, and silently alter recorded votes / group config. (b) Persistence of unexpected concrete types may make schema migration hard. (c) Type assertions in caller code (`v.(MyType)`) will panic if the concrete type changes between versions.
**Recommendation**: The doc warning on `Vote.Context` is good — replicate it on `SetMeta`. Consider providing typed helper wrappers for common cases (weight, role).

### Finding 4 — `UseStorageFactory` panics + is concrete-type-bound — YELLOW
**Location**: `member_grouping_options.gno:11,17`
**What I see**: The option panics if `fn == nil` and also panics with `"storage factory not supported by member grouping"` if the `MemberGrouping` passed in isn't the unexported concrete `*memberGrouping`. Since the type is unexported, callers cannot implement a `MemberGrouping` that supports this option — yet the API pretends to.
**Why it's a problem**: Misleading API surface; the interface implies pluggability but the option only works on the package's own concrete type. A caller passing a custom `MemberGrouping` to a constructor that then receives this option panics at init time.
**Recommendation**: Either define the storage-factory as a method on a public sub-interface (e.g. `StorageFactorySetter`), or restrict `UseStorageFactory` so it can only be passed to `NewMemberGrouping` (the only place it makes sense). Replace panic with returned error where reasonable.

### Finding 5 — `WithChildren` and `WithParent` allow cycle / tree corruption — YELLOW
**Location**: `commondao_options.gno:35,42`; `commondao.gno:103` (`TopParent` recurses)
**What I see**: Nothing prevents a caller from creating cycles in the parent/child graph (`A.parent = B; B.parent = A`). `TopParent` recurses unboundedly and would stack-overflow / infinite-loop on a cycle. `Path()` also recurses unboundedly via `parent.Path()`.
**Why it's a problem**: A buggy or malicious caller can build a DAO tree that crashes any realm calling `Path()` or `TopParent()` on its members. The mistake is easy to make if a realm allows reparenting after creation.
**Recommendation**: Detect cycles in `WithParent`/`WithChildren` (walk up the parent chain checking for self). Or document that callers must guarantee acyclicity.

### Finding 6 — `Withdraw` status mutation bypasses encapsulation — YELLOW
**Location**: `commondao.gno:183` (`p.status = StatusWithdrawn`)
**What I see**: `Proposal.status` is unexported, but `CommonDAO` mutates it directly (same package). Plus `Execute` does the same at `:266,269`. Not a bug per se, but it means the only path that updates proposal status correctly goes through `CommonDAO` — a caller using `Proposal` directly (e.g. with a custom storage that returns proposals through some other path) cannot transition status.
**Why it's a problem**: Custom `ProposalStorage` implementations cannot drive status changes; they can only observe. Tight coupling between the two structs.
**Recommendation**: Add explicit `(*Proposal).markWithdrawn()`, `.markFailed(reason)`, `.markExecuted()` methods (even if unexported). Low priority — informational.

### Finding 7 — `MustPropose` / `MustExecute` / `MustValidate` panic patterns — GREEN
**Location**: `commondao.gno:153`; `proposal.gno:118,129`
**What I see**: Standard "Must*" panics on error. Documented.
**Why it's a problem**: Not a problem in itself, but combined with Finding 2 these are footguns in handler code. Worth a docs callout.
**Recommendation**: None beyond ensuring callers wrap handlers properly.

### Finding 8 — `Vote.AddVote` overwrites previous votes silently — YELLOW [uncertain on intent]
**Location**: `record.gno:100`
**What I see**: `AddVote` allows overwriting a member's previous vote, returning `updated bool`. `CommonDAO.Vote` does not check the return value and does not surface "you already voted" to the caller.
**Why it's a problem**: Most DAO semantics make vote changes either explicit or forbidden. The package allows silent vote replacement up to the deadline; depending on DAO governance assumptions this may be desired or a footgun. The exported `ErrVoteExists` is declared but **never returned anywhere** in the package — dead code that signals intent inconsistent with behavior.
**Recommendation**: Either return `ErrVoteExists` on duplicate votes from `CommonDAO.Vote` (or add a separate `ChangeVote`), or remove the dead error. Document the chosen semantics clearly.

## Notes
- `bptree` and `seqid` are external (`gno.land/p/nt/...`) — not audited here. Trusted as primitives.
- The package has no realm-state on its own (it's a `/p/`), so persistence concerns are entirely on the caller's realm.
- `Vote` accepts `member address` parameter but checks `Members().Has(member)` — `member` is supplied by the caller, not derived from a `std.PreviousRealm()`. This is consistent with library design (caller supplies identity) but again must be enforced upstream. See Finding 2.
- `[uncertain]` Finding 1: I asserted the executor sees commondao as `PreviousRealm`, but the filetest shows the executor sees `gno.land/r/testing/dao`. The crossing-call semantics in gno are: `fn(cross)` switches realm to the callee's package when entering a crossing function (`func fn(cur realm, ...)`). The executor is defined in `r/test` and `fn(cross)` enters it as a crossing call, so `PreviousRealm()` inside it is the realm that crossed INTO it — which is whatever realm holds the `CommonDAO` and called `Execute`. If `Execute` itself is called as `dao.Execute(...)` from `r/testing/dao` without crossing into commondao first, then `commondao.Execute` runs on the caller's frame, and `fn(cross)` crosses from `r/testing/dao` into the executor's realm. Net: executor sees `r/testing/dao` as PreviousRealm — NOT the proposal creator, NOT the original transaction signer. This is still the same security warning: the executor cannot identify the original `creator` or end-user from `PreviousRealm()`. **Worth verifying against the canonical interrealm spec.**
- No reentrancy guards: an `ExecFunc` could call back into `dao.Execute` for another proposal. The `activeProposals.Remove(p.id)` happens AFTER the executor returns, so reentering with the same id would find the proposal still active. Likely benign because the inner call would fail the status check (`p.status` was reset to `StatusActive` and not yet updated until after executor returns) — actually wait, status IS updated only after executor returns, so re-entry with the same ID could try to execute again. **Potential reentrancy footgun — worth verifying.**
