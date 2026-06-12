# Improvement loop — fix the right layer, don't overfit

The loop that turns failed runs into product/skill fixes. The driver runs and
judges (SKILL.md); this is the procedure for what happens AFTER a report exists.
Run it as a separate activity from driving — never fix mid-run.

## The loop

1. **Sweep**: run the full local tier (`make playground-e2e`, or interactively
   per scenario). Collect report.md + results.json.
2. **Classify every fail** before touching anything (one classification each):
   - **product** — gnomcp/chain behavior wrong or unhelpful (bad error text,
     missing capability, wrong output)
   - **skill** — the gno skill failed to trigger, routed to the wrong
     reference, or its content gave a wrong answer
   - **scenario** — Expect asserts something wrong or untestable; Instruct is
     ambiguous enough that a reasonable human would also be confused
   - **harness** — driver/scripts/simnet defect
3. **Fix at the classified layer only.** One variable per iteration. Product
   fixes follow TDD (failing Go test first). Skill fixes follow the
   skill-creator practices below. Scenario fixes are the LAST resort — see the
   overfit guard.
   - **MCP/skill boundary:** the MCP must be complete, natural, and simple for
     ANY client — tool descriptions and errors speak chain-native language
     (gno_read, gno_packages, profiles.toml) and never reference skills,
     plugins, or one client's machinery. Agent-side behavior (when to load
     which knowledge, how to route a task) belongs in the skills. Patching one
     layer to work around the other is a hack, not a fix.
4. **Re-verify**: rerun the failed scenario (its `--category` filter keeps this
   cheap). For trigger/routing fixes, rerun the probe in ≥2 fresh sessions —
   a single pass on a probabilistic behavior is noise.
5. Repeat until the tier is green, then run the full sweep once more end-to-end
   (fixes interact; a green category is not a green tier).

## Skill-fix practices (from skill-creator)

- **Generalize from the failure** — fix the class, not the example. If the AUT
  rationalized "snippet too small for the skill", the fix addresses
  rationalization, not that snippet.
- **Explain why over commanding** — a reason the model can reason with beats an
  ALL-CAPS MUST it can rationalize past.
- **Keep it lean and natural** — prefer removing or reframing over adding;
  every token of skill text costs context on every load. If a fix only adds
  rules, look again.
- **Domain-indexed claims only** — no dates, no change-narrative, no claims
  about what models know; those rot and then mistrigger.
- **A/B against baseline** — when the fix targets triggering or answer quality,
  compare with-fix vs without-fix runs on the same prompt before declaring it.

## Overfit guard (scenario edits)

A scenario edit is legitimate only if the written Expect is wrong about the
PRODUCT (asserts behavior the product never promised) or the Instruct is
genuinely ambiguous. "The AUT keeps failing it" is never by itself a reason to
weaken an Expect — that is the regression the harness exists to catch. When in
doubt, the Expect stands and the fail is reported upward.

## Verdict for the loop itself

Done = every local scenario green in one full sweep, with each fix that
targeted probabilistic behavior re-verified in fresh sessions, and all
remaining known issues recorded as findings (not silently absorbed into
weakened scenarios).
