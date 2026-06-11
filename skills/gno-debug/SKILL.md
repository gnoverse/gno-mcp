---
name: gno-debug
description: Diagnose and fix failed Gno transactions and calls. Use whenever a gno.land transaction failed, a gnomcp write tool returned an error (insufficient_funds, authentication_required, scope_mismatch, invalid sequence, panic, out of gas), a gno_call/gno_run/gno_addpkg result looks wrong, or the user pastes a chain error message — even if they never say "debug".
---

# Debugging a failed Gno transaction

1. Read `../gno/SKILL.md` first — the source index for everything Gno.
2. Read `../gno/references/debug.md` — its failure-signature table drives this flow.
3. Classify the error against the table. Reproduce cheaply (`gno_eval`, or `simulate=true`)
   BEFORE any broadcast.
4. Apply the table's fix flow, re-run the cheap reproduction, and only then re-broadcast.
5. Report which identity signed every write (agent key vs session). If the failure involves
   sessions, remember the session path is WIP — keep scopes tight.

If nothing in the table matches, route through `../gno/SKILL.md`'s index instead of guessing
(`security.md` for authorization questions, `interrealm.md` for caller/crossing semantics) —
and say plainly what remains unknown.
