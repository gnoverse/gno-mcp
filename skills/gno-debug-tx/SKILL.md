---
name: gno-debug-tx
description: Use when a transaction failed and the user wants to understand why. Given a tx hash or error payload, classifies the failure (gas / unknown func / type / panic / auth / balance), reads the called function, explains root cause in plain language, and proposes a concrete fix.
---

# Debug a Failed Transaction

## When to use

- User pastes a tx hash, an error string, or says "why did my tx fail".

## Flow

1. Fetch tx + receipt via `gno_address_info` (if hash) or parse the error payload.
2. Classify:
   - `out of gas` → gas exhaustion
   - `unknown func` / `unknown method` → wrong function name or path
   - `type mismatch` → argument types wrong
   - `panic:` → contract panic, read the line
   - `unauthorized` → access control
   - `insufficient funds` → balance
3. Call `gno_read` on the function that was called (by symbol).
4. Explain root cause in one paragraph.
5. Propose a fix: exact parameter change, funding amount, or alternative function.
6. If the user has a draft call, offer to re-simulate with the fix via `gno_call` (simulate-only).

## Judgment

- Be specific. "Increase gas" is useless; "set gas=500000 (current: 200000)" is useful.
- If you need source to explain, fetch a slice — never guess.
