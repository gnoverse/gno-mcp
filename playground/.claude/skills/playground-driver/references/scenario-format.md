# Scenario format

A scenario is one markdown file under `e2e/scenarios/<tier>/`. Copy `e2e/TEMPLATE.md`.

## Frontmatter
| Key | Values | Meaning |
|---|---|---|
| id | kebab-case, unique | report key |
| tier | local \| external | local must be green; external tolerates `blocked` |
| category | reads, writes, connect, skill, audit, sessions, localdev, … | feature set; `--category` filters on it |
| timeout-minutes | int | whole-scenario budget incl. debrief |
| covers | list of feature keys | documentation only (driver never acts on it); keys are canonical in `e2e/COVERAGE.md` — update that ledger when adding/removing one |
| image | e2e (default) \| e2e-faucetcap \| l1-fresh \| l2-gnomcp \| l3-full | optional; Docker build target the AUT container runs (`up.sh <image>`). `e2e-faucetcap` is the e2e harness with the faucet's per-address cap tightened to 1 (a second fund of one address trips it) — same simnet/gnoquery, use it to exercise the per-address faucet limit. Non-e2e layers carry NO simnet/gnoquery/gnokey — Verify facts must be turn-log or container-state, never chain ground truth |

## Sections
- Free text after the title = DRIVER-ONLY context (assumptions, what to watch).
- `## Step N: <name>` with:
  - optional driver-context lines before Instruct — may script a FIXED reply for a
    permission round-trip ("if the AUT asks X, reply exactly Y once"); the step is then
    judged across its turns. Scripted replies are user-voice and never reveal expectations.
  - `### Instruct` — sent to the AUT VERBATIM (after `$RUN_ID` substitution). The ONLY part the AUT ever sees. USER VOICE: write what a person would type to their Claude. Name tools/`/gno` explicitly ONLY in scenarios that test direct invocation.
  - `### Expect` — judgment criteria (answer + turn log), tagged by dimension where useful. Mandatory.
  - `### Verify` — binding FACTS. The fact binds; the method is the driver's, using the verify
    toolkit (`references/verify-toolkit.md`). Two kinds: chain ground truth (`gnoquery` — never
    skip; an answer the chain contradicts fails no matter how plausible) and turn-log facts (read
    the transcript schema, never raw jsonl — see verify-toolkit.md + judging.md § Observing).
    State the fact precisely (the expected value); the driver chooses how to check it. Optional per step.
- `## Debrief` — seed questions, driver-side. The driver adds probes per finding.

## Rules
- NEVER leak Expect/Verify/frontmatter to the AUT.
- Steps share one session; later steps may reference earlier ones.
- On-chain names that writes create MUST embed `$RUN_ID` (chain state persists across scenarios in a run).
- A goal-based scenario = one step with a goal-shaped Instruct + outcome-shaped Expect. Same format.
