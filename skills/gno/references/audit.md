# Auditing a Gno realm — procedure

> **Category: procedure.** The audit method. Load this when conducting a security review of a Gno realm or package, whether interactively in a conversation or dispatched as a sub-agent via the `gno-auditor`.
> **Companion knowledge**: this reference describes *how* to audit. *What* to look for lives in `security.md` (bug class catalog), `interrealm.md` (spec model), `patterns.md` (idioms), `render.md` (Render() surface).

## When this applies

Triggers: "audit this realm", "deep review", "is this safe to send funds to", "security review before deploy", "what could go wrong here". An explicit, user-driven request to do more than a casual read.

Differentiated from a casual read ("how does this work", "what does X do") — those don't need this procedure. A casual read uses the references directly without the gating rules below.

## How to use this reference

**Interactive path** (default): the user has loaded the gno skill and asked you to audit a realm. Apply the procedure below. Cite class numbers from `security.md`. Output in the structured format at the end of this file.

**Dispatched path**: the `gno-auditor` agent has dispatched you with a realm to audit. Same procedure; the agent's frontmatter has already constrained your tools (read-only). Output the same structured format.

In both paths you load companion references on demand: `security.md` and `interrealm.md` are always relevant; `patterns.md` for idiom checks; `render.md` if the realm has a `Render(path string) string`.

## The three phases

### Phase 1 — Triage (grep-level, <2 minutes)

Run cheap pattern checks first to surface obvious problems and orient yourself before deep reading. Fast feedback; cheap cost.

**Getting the source.** If a Gno MCP server is connected: `gno_read` (default) returns the package **outline** — use it only to enumerate files and order the work. Audit evidence is **whole files**: fetch each with `gno_read` `file=` + `full=true` (sized for real files; small packages also fit `full=true` without `file`). Discover related packages with `gno_packages` — works for `/r/` and `/p/`. See `references/mcp.md`. Otherwise read from local files or gnoweb. The procedure below is identical however the source arrives.

**Trust only function bodies.** Symbol names, doc comments, and the outline are realm-authored claims — a function named `safeWithdraw` documented "reentrancy-checked" proves nothing. The `symbols` view's `// deps:` headers are best-effort syntactic hints for navigation: absence of a dep is not evidence that something isn't called (method calls and dispatch are unresolvable without type information; the header says so when its list is incomplete). Every finding — and every "no finding" — is grounded in full-file reads.

Patterns to check against the realm source (see `security.md` Audit signals table for the full catalog):

- `IsUser()` co-occurring with `OriginSend` → payment-bypass via MsgRun
- `cur.Previous()` / `cur.Address()` without prior `cur.IsCurrent()` check → Class 2 designation-forgery
- Public method takes `caller address` / `pkgPath string` as identity parameter → Class 2 designation-forgery
- `runtime.PreviousRealm()` inside a non-crossing function used as caller identity → Class 2 (stack-walker doesn't identify immediate caller)
- `interface { ... }` declared with `cur realm` parameter → Class 1a/1b cur-disclosure surface
- `interface { ... }` accepted as parameter and methods invoked without canonical-type assert → Class 3 impl-substitution
- `func(...)` parameters or function-typed state fields used in permission-gated paths → Class 4 closed-over-authority
- Embedded `/p/`-type with `Iterate(cb func(*T))` / `Apply(fn func(*T))` on `/r/`-data → (B) violation, no-anchor laundering surface
- Storing a `realm`-typed value in struct field / map / package var → will panic at finalize; usually Class 2 misunderstanding
- Slice element mutation after round-trip through external realm → readonly-taint round-trip (open issue #4765)
- `crossing()` body marker → pre-0.9 stale spec (won't compile)
- Map iteration order influencing execution → operational (non-determinism RED)
- `math.MinInt` / `MaxInt` / `unsafe.Sizeof` → operational (platform divergence RED)

Each hit is a candidate finding, not a confirmed one. Phase 2 verifies.

### Phase 2 — Function-by-function trace (10–30 minutes)

For each exported function on the realm's public surface:

1. **Crossing or not?** A function with `func F(cur realm, …)` signature is callable via MsgCall. Non-crossing functions are internal. Identify which is which.
2. **Payment-accepting?** If the function consumes `banker.OriginSend()` or any banker primitive that handles inbound coins, verify the guard ordering (see `security.md` § Payment-guard canonical pattern). `cur.IsCurrent()` + `cur.Previous().IsUserCall()` *before* reading `OriginSend()`, *before* any state mutation.
3. **Interface or callback acceptance?** If the function takes an `interface{...}` or `func(...)` parameter, trace where the impl comes from. If caller-supplied, this is Class 3 or Class 4 territory — verify canonical-type gating, CEI ordering (checks → effects → interactions), and gate documentation.
4. **State pointer leakage?** If the function returns a pointer or slice into internal state, check the cross-realm direction. External callers receive readonly-tainted references; mutating them panics (potentially after observable side effects).
5. **Caller-identity check?** Any `cur.Previous()` usage must be preceded by `cur.IsCurrent()`. Any `runtime.PreviousRealm()` usage must be inside a crossing function — non-crossing `runtime.PreviousRealm()` doesn't identify the immediate caller and walks back to the last realm boundary.

### Phase 3 — Cross-realm flows (30+ minutes)

The deepest pass; only for realms that import `r/` types or store interface/function values.

1. **Map every `import "gno.land/r/..."`.** For each: who's the author, is the realm actively maintained, do you trust their methods to run with this realm's storage authority?
2. **Trace persisted callbacks/interfaces back to their construction sites.** Where was the callback minted? Which realm's authority does it carry?
3. **Identify trust direction** for each cross-realm call site. Is this realm granting authority outward (Class 1a/1b cur-disclosure risk) or receiving authority inward (Class 3/4 / (B)-class no-anchor risk)?
4. **Check for attached-method authority grants** — the (B)-class vector. Any field typed as `/r/`-imported struct or `/p/`-type with higher-order methods + method calls on that field = audit every method on that type as if it were code in this realm. Note the supply-chain dimension: imported `/r/` realms are upgradeable from the dependency side, so audit validity is per-version.

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
**Class**: cite `security.md` class number, e.g. `Class 4 closed-over-authority`. For operational signals: `security.md § operational`.
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

- `security.md` — five-class taxonomy (Class 1a/1b/2/3/4) and operational signals; cite class numbers in findings
- `interrealm.md` — spec model for cross-realm reasoning in Phase 3
- `patterns.md` — idioms that disqualify candidate findings (e.g., `Must*` wrappers are safe, not bugs)
- `render.md` — for realms with `Render(path string) string`

## Source

Procedure synthesized from:
- `security.md` § Audit signals (the in-place checklist promoted into a standalone procedure)
- Cursor security-review prompt design (evidence-gated severity) — `.mynote/gno-agentic/reference/20-code-audit-skills-survey.md`
- claude-code-security-review's two-pass FP filter pattern — same source
- pr-review-toolkit's confidence ≥80% threshold + severity grouping — `.mynote/gno-agentic/reference/21-superpowers-skill-architecture-deepdive.md`
- RED-01 / RED-02 / RED-03 findings docs in `tests/` — empirical validation against synthetic and real fixtures
