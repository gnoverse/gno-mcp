# RED-phase 01 — shop fixture audit comparison

Date: 2026-05-15
Fixture: `tests/fixtures/shop/shop.gno` (52 lines, 4 exported funcs + Render)
Method: two general-purpose subagents, same prompt except one had access to the skill.

## Planted issues in fixture

| Location | Class | Severity |
|---|---|---|
| `Buy` — `IsUser()` + `OriginSend()` | §1 / §7 | RED |
| `SetCallback` — `func(realm)` stored in state, admin-gated | §3 latent (callback never invoked) | YELLOW |
| `Stock` — `PreviousRealm().Address() == admin` inside crossing function | not a bug — control case | GREEN |
| `Render` — no untrusted input | not a bug — control case | GREEN |

## Results side-by-side

| | Baseline (no skill) | With-skill |
|---|---|---|
| RED findings | 3 | 1 |
| YELLOW findings | 4 | 1 |
| GREEN explicit | 0 | 2 |
| §1 IsUser/OriginSend caught | ✓ (RED) | ✓ (RED, cited §1 + §7) |
| §3 callback caught | ✓ (RED, severity overcalibrated) | ✓ (YELLOW, correctly noted "latent") |
| Control: admin check in crossing fn correctly NOT flagged | implicit (not mentioned) | explicit (called out as GREEN with anti-pattern-reflex citation) |
| Control: Render is safe | not mentioned | mentioned (GREEN) |
| **Operational concerns caught** | **3** (no withdraw, no admin rotation, defensive guards) | **1** (admin rotation only) |
| Uncertainty markers | banker API name, closure persistence semantics | none on Gno specifics |

## Where the skill helped

1. **Precise classification.** With-skill cited exact class numbers (§1, §7, §3) and used skill vocabulary ("receipt invariant", "trichotomy", correct `/e/` ephemeral path). Baseline was correct in substance but less precise; the agent self-reported uncertainty about banker API names and closure persistence — uncertainties the skill eliminated.

2. **Severity calibration.** With-skill graded the callback YELLOW (latent — never invoked) rather than RED (baseline). The YELLOW is more accurate: the bug only fires once a future commit wires the callback into `Buy`. Skill's class-based grading rubric helped.

3. **Explicit GREEN findings.** With-skill explicitly documented that `PreviousRealm().Address() == admin` inside a crossing function is fine, citing the "anti-pattern reflex" in SKILL.md. Without this, an auditor either silently passes (no recorded "I checked this") or false-positives. The skill provides the language to record GREEN.

4. **No Gno-specific uncertainty.** Baseline marked banker API names and foreign-closure persistence as `[uncertain]`. With-skill agent didn't need those caveats — the skill resolved them.

## Where the skill missed (real gaps)

1. **Operational concerns not in the catalog.** Baseline caught:
   - Coins accumulating in realm with no withdraw path (RED) — *real audit signal*, not a Gno-language bug class
   - Admin role with no rotation function (YELLOW)
   - Stock accepting negative `qty` (defensive programming, YELLOW)
   - Silent type-assertion fallback on `inventory.Get` (defensive programming, YELLOW)

   The skill's `security.md` is bug-class-focused; it doesn't carry "operational" or "defensive programming" signals. These are real audit value-adds that the skill misses today.

2. **Latent callback patterns** — with-skill agent's own gap report: §3 should mention "registered-but-never-called function-valued state" as a footgun-in-waiting, not just the active-invocation case. The agent identified the gap from inside the skill.

3. **Test fixture might be too easy.** The §1 IsUser/OriginSend bug is the most-publicized Gno gotcha; even the baseline agent called it "the documented project gotcha." A more discriminating test would use §9 (attached-method privilege escalation) or §8 (slice readonly-taint surprise) — bug classes LLMs DON'T have in training data. The §1 test shows the skill matches the baseline's headline finding precision; it doesn't demonstrate the skill's unique value on less-publicized classes.

## Actions taken on the skill

1. Added "latent callback" one-liner to `security.md` §3 (skill self-identified gap).
2. Did NOT add operational concerns to `security.md` — those belong in `patterns.md` as anti-patterns (no exit path, no admin rotation), not in the bug-class catalog. Added a note to `patterns.md` § "Anti-patterns at a glance".

## Actions deferred

1. **Build a §9 / §8 fixture** for the next RED phase. Those classes are the skill's actual differentiated value.
2. **Decide on operational-signals scope.** The baseline agent's "no withdraw path" finding is real and valuable. If the skill is meant to be the auditor's full toolkit, it should cover that. If it's strictly bug-class-focused, it shouldn't. Worth a design decision before the next iteration.
3. **Test against a real `examples/` realm**, not a synthetic fixture, to see how the skill scales to noisier code.

## Honest read

This is a controlled-fixture test of one bug class (§1 / §3). The skill **demonstrably shifts behavior** in three measurable ways (precision, severity calibration, GREEN documentation), but **does not change the headline finding** — both agents identified §1 correctly. The skill's value on this test is in audit *quality*, not in *catch rate*.

A higher-signal RED test would use a bug class the baseline LLM doesn't already know. That's the next step.
