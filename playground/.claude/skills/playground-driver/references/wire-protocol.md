# Wire protocol — driver ↔ AUT

All paths relative to `playground/`. Scripts: `e2e/scripts/`. Container default: `gnomcp-e2e`.

## Run setup
- `RUN_ID`: 6+ alnum chars, lowercase, unique per run (derive from UTC time, e.g. `r$(date -u +%H%M%S)`). Substitute every literal `$RUN_ID` in Instruct text BEFORE sending.
- Run dir: human-readable + sortable — `e2e/reports/$(date -u +%Y-%m-%d_%H-%M-%S)-<title>/` where
  `<title>` = the scenario id for a single-scenario run, else `<tier>[-<category>]`. When invoked
  with `--report <dir>` (batch), use that dir VERBATIM — never rename it. Per-scenario subdir
  named by scenario id.
- Per scenario: fresh `SID=$(uuidgen | tr 'A-Z' 'a-z')`; one scenario = one AUT session.

## Lifecycle (driver-owned)
- **Batch preflight: ALWAYS `up.sh` at sweep start.** A fresh conversation cannot know what a
  previous run left on the chain (keys, counter state, deployed packages), and scenarios may
  assume a fresh chain — recreating is the only verifiable clean state.
- **Interactive preflight (judged reuse):** `docker inspect -f '{{.State.Running}}' gnomcp-e2e`.
  Reuse iff running AND `gnoquery height` works AND no write-scenario ran in it this conversation
  AND no selected scenario assumes state the chain no longer has (when in doubt, spot-check with
  `gnoquery render` against the scenario's assumptions); else `up.sh` (always recreates).
- After a batch: green → `down.sh`. Any fail → LEAVE the container running; record in report:
  "container kept for postmortem — cleanup: `e2e/scripts/down.sh`".
- Interactive: always leave it up.

## Turns
- Send: `printf '%s' "<instruct>" | e2e/scripts/turn.sh <sid> <n> <scenario-run-dir> [--first]` — `--first` only on turn 1.
- Set the Bash tool timeout to the step budget (default 300000 ms). A killed turn = step `fail` (note: timeout).
- stdout = the AUT's answer; `AUT_ERROR(...)` prefix = harness-level error (judge as `blocked`, not `fail`, unless reproducible).
- Evidence: `<scenario-run-dir>/turn-<n>.jsonl`. Tool calls appear as `assistant` events with `tool_use` content (`name`, `input`). The AUT loads MCP tools lazily — a `ToolSearch` call before the first gnomcp call is normal, never a finding.
- **MCP in-memory state resets at every turn boundary.** Each turn is a fresh claude process with a fresh stdio gnomcp child: dynamic profiles (`gno_profile_add`) and any other in-process MCP state from earlier turns are GONE. Scenarios must not assume cross-turn MCP state; flows that need it must fit in one turn (and the product gap is a recorded finding, not the AUT's fault).

## Verify
- Establish each step's Verify fact with the verify toolkit (`references/verify-toolkit.md`) — `gnoquery` for chain truth, the transcript schema for behavior. The fact binds, not the command. Compare against the AUT's claims: an answer the chain contradicts is `fail` even if plausible.

## Step loop
1. Substitute `$RUN_ID`, send Instruct.
2. Read answer + turn log; run Verify.
3. Judge per judging.md; record verdict + evidence + findings.
4. pass → next step. fail → ≤2 diagnostic probe turns (free-form "what did the tool return?", judged against nothing, logged), then abort scenario: remaining steps `skipped`; continue with the next scenario.

## Debrief (after steps, same session)
- Compose from: the scenario's `## Debrief` seeds + one probe per queued finding. ≤5 turns total.
- Mirror the AUT's own wording when probing ("you said X — why?"). Never reveal Expect/Verify content.
- Verbatim Q/A into the report; close with 1–3 "improvement leads" (skill/MCP-description changes the answers suggest).
