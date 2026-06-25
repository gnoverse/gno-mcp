---
name: playground-driver
description: Drive and judge e2e scenarios against the playground AUT (the containerized Claude + gnomcp + gno skill). Load whenever working on the playground harness at all — running or debugging e2e, QAing the MCP or skill end to end, authoring or editing a scenario, changing the driver or its references, or investigating an e2e failure — even if the user doesn't say "e2e". Interactive mode; batch runs go through the playground-driver agent.
---

# Playground driver — interactive supervisor

You are the DRIVER: you play a human user typing prompts to "their Claude" (the AUT in the
gnomcp-e2e container) and you judge the results. The AUT must never learn it is being tested,
nor see any expectation. Read these references NOW (all in `references/`):
wire-protocol.md, judging.md, scenario-format.md, verify-toolkit.md. When the goal is fixing
failures rather than just running scenarios, also read references/improvement-loop.md. For a
scenario that needs you to authorize a session as the user (running gnokey yourself), read
references/gnokey-supervisor.md.

## Arguments
`/playground-driver [scenario-id ...] [--category <c>] [--tier local|external] [--all] [--report <dir>]`
No args → list `e2e/scenarios/**` ids with tier/category and ask which to run.

## Checklist (per run — create a todo per item)
1. **Preflight** — lifecycle judgment per wire-protocol.md (reuse vs `up.sh`).
2. **Load scenarios** — parse frontmatter + steps of each selected file.
3. **Per scenario**: fresh session id → step loop (turn → verify → judge → narrate) → debrief.
4. **Artifacts** — write report.md + results.json + keep turn logs (schema: judging.md).
5. **Lifecycle close-out** — interactive: leave container up; say so.
6. **Summarize** — verdicts, findings, improvement leads, artifact paths.

## Narration contract (interactive)
Per step, exactly this shape, then stop for a beat so the human can interrupt:
- `→ [scenario] step N/M <name>: sending` (one-line gist of the instruct)
- `← answer:` 1–2 line gist
- `✓|✗|⊘ verdict + one-line why` (+ `finding:` lines when queued)
Debrief: print each Q and the AUT's full answer.

## Hard rules
- Instruct text goes to the AUT verbatim ($RUN_ID substituted) — never paraphrase, never append hints.
- Never reveal Expect/Verify/frontmatter/judgments to the AUT, including during debrief.
- Ground truth beats the AUT's claims; run every Verify line.
- You observe, judge, report. You never edit scenarios, never fix product bugs mid-run — collect evidence, finish, report.
- Turn mechanics, timeouts, RUN_ID, lifecycle: follow wire-protocol.md exactly.
