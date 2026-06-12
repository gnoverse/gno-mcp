---
id: my-scenario
tier: local
category: reads
timeout-minutes: 15
covers: [read.render]
---
# One line: what regression/behavior this catches

Driver-only context: assumptions, what to watch for, which protocol it mirrors.

## Step 1: short-step-name
### Instruct
User-voice prompt. `$RUN_ID` available for unique on-chain names.
### Expect
- correctness: …
- tool-selection: …
### Verify
- A binding fact (the driver picks the method — see verify-toolkit.md). Chain truth: `gnoquery render gno.land/r/test/counter` — output matches the AUT's claim. Behavior: state it plainly (e.g. "a `gno_render` tool_use in turn 1"), established from the transcript schema, not a verbatim jq.

## Debrief
- Seed question about the choices this scenario exposes.
