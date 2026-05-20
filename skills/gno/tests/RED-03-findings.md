# RED-phase 03 — commondao (real production code)

Date: 2026-05-15
Fixture: `examples/gno.land/p/nt/commondao/v0/` — 14 source files, ~1900 LOC. Real deployed `/p/` package.
Method: two general-purpose subagents, same prompt except one had access to the skill.

## Why this test matters

First test against a real, noisy codebase rather than a synthetic fixture. The commondao package is also the documented source for `security.md` §3 (`ExecFunc func(realm) error`), so the test validates the skill's ability to recognize patterns it claims to catalogue.

## Headline

The skill's biggest single-test win to date: **with-skill correctly re-framed severity for a `/p/` library**.

| | Baseline | With-skill |
|---|---|---|
| RED findings | 2 | 0 |
| YELLOW findings | 5 | 6 |
| Severity framing | applied realm-audit rubric to a library | "0 RED at the `/p/` boundary; RED in any importing realm that exposes the option setters to untrusted input" |
| §3 ExecFunc callback caught | yes (no class name; framed as "interrealm gotcha") | yes (cited §3 + ACTIVE status note that names this exact file) |
| §2 interface re-entrancy layered on §3 | partial (F8 silent overwrite related) | yes (explicit, with CEI ordering fix recommendation) |
| §9 implications of persisted closure (`UseStorageFactory`) | not flagged | yes (cited §3 latent → §9 attached-method authority for closure-minting realm) |
| `cur realm` illegal in `/p/` consequence on `ExecFunc` | not raised | yes (skill prompted the realization; spec wrinkle becomes audit-readability note) |
| References loaded | n/a | SKILL.md + security.md + patterns.md + peek at interrealm.md (skipped render/stdlib/future correctly) |

## Where baseline was better

- **`Vote.AddVote` silent overwrite + dead `ErrVoteExists`** — F8 in baseline. `AddVote` returns `updated bool` but `CommonDAO.Vote` doesn't check; exported error never returned anywhere. With-skill missed this entirely. Pattern worth catching: "exported error declared but never used" suggests intent inconsistent with behavior.
- **`Withdraw` status mutation breaks encapsulation** — baseline F6. Custom `ProposalStorage` impls can observe but not drive status. Architectural observation, low-stakes, but real.
- **Mechanical re-entrancy explanation** — baseline traced the exact mechanism ("activeProposals.Remove happens AFTER the executor returns, so re-entering with the same id would find the proposal still active"). With-skill said the same but baseline's mechanical phrasing was sharper.

## Skill gaps the with-skill agent identified

Verbatim from the with-skill report's Notes:

1. **No centralized `/p/` audit lens.** "This package's risk surface is whatever every importing realm fails to wrap." Implicit in §3 status notes; deserves its own paragraph in `security.md` or SKILL.md.
2. **`bptree` not addressed.** patterns.md recommends `avl.Tree` for persisted keyed state; commondao uses `gno.land/p/nt/bptree/v0` instead. Skill should at minimum acknowledge alternatives exist.
3. **`Must*` panic-on-error convention.** Used throughout Gno (`MustExecute`, `MustValidate`, `MustPropose`); the skill's patterns.md doesn't acknowledge it as safe idiom.

## Actions taken on the skill

1. **Added a `/p/` audit lens subsection** to `security.md` under "Severity calibration" — when the audit target is a `/p/` package, the rubric shifts: RED becomes "any naive importer ships a vulnerability", YELLOW becomes "importer must actively misuse it", GREEN "safe regardless of importer behavior". The package's own documentation honesty (e.g. `"v0 - Unaudited"`) feeds severity calibration.
2. **Added operational signal** for "exported error declared but never returned (intent inconsistent with behavior)" — directly catches baseline's F8 finding.
3. **Added `bptree` note** to `patterns.md` § state shape — alongside `avl.Tree`, mention that `bptree` (`gno.land/p/nt/bptree/v0`) is an alternative ordered keyed collection with similar persistence properties; either is canonical.
4. **Added `Must*` convention** to `patterns.md` § panic vs error — explain Must wrappers as safe idiom (caller chooses panic-on-error or err-returning sibling).

## Verdict

After three RED phases, the skill's value beyond baseline LLM Gno knowledge is consistent:
- Names canonical bug classes (§1, §2, §3, §5, §7, §9) with citations
- Eliminates `[uncertain]` markers on Gno specifics
- Resolves severity calibration via class status + new `/p/` lens
- Operational signals layer catches non-bug-class audit concerns

Persistent skill limitation: it's a **knowledge pack**, not a code reader. It only catches what it's been told to grep for. Anything novel (the dead `ErrVoteExists` + silent overwrite combo from baseline F8) the auditor still has to spot on their own — until that pattern enters the catalog.

## Open questions for next iteration

1. **Should the skill load multiple references by default for audit mode?** Today the with-skill agent decided per-finding which to load. The router-and-on-demand approach worked, but a "for audits, always load security.md + patterns.md + interrealm.md" hint in SKILL.md might shave latency on multi-class audits.
2. **§8 (slice readonly-taint, OPEN) fixture remains undone.** Worth building since the open class hasn't been tested.
3. **Test against `r/gnoland/blog` or similar production realm** to see how the skill scales to noisier real code, not just real library code.
