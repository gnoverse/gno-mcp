---
name: gno-debug
description: Use when a gno.land transaction failed, when a gnomcp write tool returned an error (insufficient_funds, authentication_required, scope_mismatch, invalid sequence, panic, out of gas), when a gno_call/gno_run/gno_addpkg result looks wrong, or when a chain error message is pasted — even if "debug" is never said. Entered BEFORE you retry, re-sign or re-broadcast anything, since a blind retry costs the fee again and hides the cause.
---

# Debugging a failed Gno transaction

1. Read `../gno/SKILL.md` first — the source index for everything Gno.
2. Read `../gno/references/debug.md` — its failure-signature table drives this flow.
3. Classify the error against the table. Reproduce cheaply (`gno_eval`, or `simulate=true`)
   BEFORE any broadcast.
4. Apply the table's fix flow, re-run the cheap reproduction, and only then re-broadcast.
5. Report which identity signed every write (agent key vs session). If the failure involves
   sessions, remember the session path is WIP — keep scopes tight.

For a gas, fee, storage-deposit, or `account/sequence/chain-id` signature failure, pull
`../gno/references/gnokey.md` — it has the `Fee = {GasWanted, GasFee}` model and the failure-mode
table (out-of-gas vs insufficient-fee, "not enough deposit", the stale-binary cause of a sig-verify
error), which explain *why* these fail and the exact fix.

If nothing in the table matches, route through `../gno/SKILL.md`'s index instead of guessing
(`security.md` for authorization questions, `interrealm.md` for caller/crossing semantics) —
and say plainly what remains unknown.
