# Auditing a Gno realm — procedure

> **Category: procedure.** The audit method. Load this when conducting a security review of a Gno realm or package, whether interactively in a conversation or dispatched as a sub-agent via the `gno-auditor`.
> **Companion knowledge**: this reference describes *how* to audit. *What* to look for lives in `security.md` (bug class catalog), `interrealm.md` (spec model), `patterns.md` (idioms), `render.md` (Render() surface).

## When this applies

Triggers: "audit this realm", "deep review", "is this safe to send funds to", "security review before deploy", "what could go wrong here". An explicit, user-driven request to do more than a casual read.

Differentiated from a casual read ("how does this work", "what does X do") — those don't need this procedure. A casual read uses the references directly without the gating rules below.

## How to use this reference

**Interactive path** (default): the user has loaded the gno skill and asked you to audit a realm. Apply the procedure below. Cite class numbers from `security.md`. Output in the structured format at the end of this file.

**Dispatched path**: the `gno-auditor` agent has dispatched you with a realm to audit. Same procedure; the agent's frontmatter has already constrained your tools (read-only). Output the same structured format.

In both paths you load companion references on demand: `security.md` and `interrealm.md` are always relevant; `patterns.md` for idiom checks; `render.md` if the realm has a `Render(path string) string`; `future.md` before reporting verdicts to avoid flagging in-flight migrations as bugs.

## The three phases

### Phase 1 — Triage (grep-level, <2 minutes)

Run cheap pattern checks first to surface obvious problems and orient yourself before deep reading. Fast feedback; cheap cost.

Patterns to check against the realm source (see `security.md` Audit signals table for the full catalog):

- `IsUser()` co-occurring with `OriginSend` → §1 payment bypass
- `crossing()` body marker → §10 pre-0.9 stale spec
- `PreviousRealm()` inside non-crossing functions → §7 caller-identity misuse
- Methods on receivers persisted into other-realm state → §9 attached-method privilege
- `interface { … }` stored in state, invoked from gated functions → §2 re-entrancy surface
- `func(…)` parameters or function-typed state fields → §3 callback substitution
- `&someStateVar` passed to external realm → §6 pointer ownership
- Slice element mutation after round-trip through external realm → §8 readonly-taint
- `cur realm` forwarded as argument to a non-crossing call → §5 cur disclosure
- Map iteration order influencing execution → operational (non-determinism RED)
- `math.MinInt` / `MaxInt` / `unsafe.Sizeof` → operational (platform divergence RED)

Each hit is a candidate finding, not a confirmed one. Phase 2 verifies.

### Phase 2 — Function-by-function trace (10–30 minutes)

For each exported function on the realm's public surface:

1. **Crossing or not?** A function with `func F(cur realm, …)` signature is callable via MsgCall. Non-crossing functions are internal. Identify which is which.
2. **Payment-accepting?** If the function consumes `banker.OriginSend()` or any banker primitive that handles inbound coins, verify the guard ordering (see `security.md` § Payment-guard canonical pattern). `IsUserCall()` *before* reading `OriginSend()`, *before* any state mutation.
3. **Interface or callback acceptance?** If the function takes an `interface{…}` or `func(…)` parameter, trace where the impl comes from. If caller-supplied, this is §2 or §3 territory — verify CEI ordering (checks → effects → interactions) and gate documentation.
4. **State pointer leakage?** If the function returns a pointer or slice into internal state, check the cross-realm direction. External callers receive readonly-tainted references; mutating them panics (potentially after observable side effects).
5. **Caller-identity check?** Any `PreviousRealm()` usage must be inside a crossing function. Non-crossing PreviousRealm checks don't identify the immediate caller — they walk back to the last realm boundary.

### Phase 3 — Cross-realm flows (30+ minutes)

The deepest pass; only for realms that import `r/` types or store interface/function values.

1. **Map every `import "gno.land/r/..."`.** For each: who's the author, is the realm actively maintained, do you trust their methods to run with this realm's storage authority?
2. **Trace persisted callbacks/interfaces back to their construction sites.** Where was the callback minted? Which realm's authority does it carry?
3. **Identify trust direction** for each cross-realm call site. Is this realm granting authority outward (§9 risk) or receiving authority inward (§5 risk)?
4. **Check for §9 attached-method authority grants.** Any field typed as `r/`-imported struct + method calls on that field = audit every method on that type as if it were code in this realm. Note the supply-chain dimension: imported `r/` realms are upgradeable from the dependency side, so audit validity is per-version.

## Evidence-gating rule

**Do not emit a finding until you can trace evidence end-to-end.** Specifically:

1. **Input → sink trace.** For a candidate finding, identify the *input* (where untrusted data enters — a parameter, an interface method, an external call return) and the *sink* (where it has effect — state mutation, banker call, panic, return). If you cannot draw the path, don't emit; investigate further or downgrade to a question.
2. **Existing controls check.** Before flagging "missing guard X", verify the guard isn't already present in another layer (a parent crossing-function wrapper, an `init()`-time check, a require pattern earlier in the call). Many false positives are missed-the-existing-guard.
3. **Confidence threshold.** Only emit findings with confidence ≥ 80% on the *existence* of the issue. Confidence < 80% becomes a "question" in the report, not a finding.

This rule is the single biggest precision lever — adopted from the Cursor security-review prompt design (`Snyk analysis 2025`).

## Two-pass false-positive filter

After completing the first detection pass, run a **second pass that challenges each RED/YELLOW finding**.

For each finding emitted in pass 1:
- **Restate the claim** in one sentence.
- **List the evidence** (file:line citations).
- **Identify the strongest objection** an experienced realm author would raise: "but the wrapper at X catches this", "but the caller is always trusted because Y", "but the type system prevents Z".
- **Resolve**: if the objection holds, downgrade the finding (RED→YELLOW, YELLOW→GREEN) or remove it. If the objection doesn't hold, keep the finding and note the considered objection in the report.

When dispatched as a sub-agent, the two-pass filter runs as a separate Task tool dispatch — fresh context, no anchoring bias from the first pass.

## Severity calibration

Default rubric, with the `/p/` audit lens shift documented in `security.md`:

| Severity | `/r/` realm | `/p/` package |
|---|---|---|
| RED | Exploitable today on master; block deploy/send/interact | Any naive importer ships a vulnerability (the library hands callers a footgun) |
| YELLOW | Exploitable depending on context (trust assumption, CEI ordering); investigate before clearing | Importer must actively misuse it; library exposes the surface but doesn't force it |
| GREEN | Pattern matched, trust assumption explicit and reasonable | Safe regardless of importer behavior |

For `/p/` libraries, cite findings as `YELLOW (RED in any realm that exposes <surface> to public input)` when the dangerous shape is structurally necessary but importer-conditional. This is more honest than flattening to RED at the wrong layer.

## Output format

Report in this exact shape. Findings grouped by severity (RED first), sorted by location within group.

```markdown
# Audit: <realm path>

## Verdict
One sentence: BLOCK / PROCEED WITH CAUTION / OK.

## Confidence
First-pass: <count> findings. After FP filter: <count> findings (Δ <count> downgraded/removed).

## Findings

### RED

#### R1 — <name> — confidence: <0-100>%
**Location**: file:line(s)
**Class**: cite `security.md` class number, e.g. `§9 attached-method`. For operational signals: `security.md § operational`.
**Evidence**: 1-2 sentences with the input → sink trace.
**Why this is exploitable today**: 1-2 sentences.
**Considered objection**: from the FP filter pass — what an experienced reviewer might say, and why it doesn't apply.
**Recommendation**: action with skill-reference citation.

### YELLOW

[same shape]

### GREEN (noted, not blocking)

[shorter shape — just location + what's fine]

## Open questions
Things below confidence threshold or that warrant verification but aren't findings.

## Cross-references
References loaded during this audit (security.md, interrealm.md, etc.).
```

## Cross-references

- `security.md` — bug-class catalog (§1–§10) and operational signals; cite class numbers in findings
- `interrealm.md` — spec model for cross-realm reasoning in Phase 3
- `patterns.md` — idioms that disqualify candidate findings (e.g., `Must*` wrappers are safe, not bugs)
- `render.md` — for realms with `Render(path string) string`
- `future.md` — check before flagging in-flight migrations as bugs

## Source

Procedure synthesized from:
- `security.md` § Audit signals (the in-place checklist promoted into a standalone procedure)
- Cursor security-review prompt design (evidence-gated severity) — `.mynote/gno-agentic/reference/20-code-audit-skills-survey.md`
- claude-code-security-review's two-pass FP filter pattern — same source
- pr-review-toolkit's confidence ≥80% threshold + severity grouping — `.mynote/gno-agentic/reference/21-superpowers-skill-architecture-deepdive.md`
- RED-01 / RED-02 / RED-03 findings docs in `tests/` — empirical validation against synthetic and real fixtures
