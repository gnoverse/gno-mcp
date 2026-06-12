---
id: write-tools
tier: local
category: writes
timeout-minutes: 25
covers: [write.key-generate, write.key-address, write.faucet-fund, write.funds-recovery, write.call, write.addpkg, write.simulate, write.signer-reporting, skill.auto-trigger-authoring]
---
# Agent-identity write flow: key generate → faucet → call → addpkg → simulate

The agent-identity write path on simnet: generate a key, fund it, call, deploy, simulate.
Profile `testnet` (test9999) has faucet-service-url set ⇒ tier-2 automatic funding.
The agent key signs (never test1 on testnet). Run AFTER read-tools is green.
Watch: does the AUT discover gno_faucet_fund from the insufficient_funds error itself?
Watch: gno_account to check its own balance during the funding flow (watch-level —
note presence/absence for the debrief, never a verdict factor).

## Step 1: own identity
### Instruct
I want you to be able to transact on this testnet yourself. Set yourself up with your own key and tell me your address.
### Expect
- correctness: reports a bech32 g1… address that is NOT g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5 (test1).
- tool-selection: gno_key_generate in the turn log (gno_key_address also fine).
### Verify
- `docker exec "${E2E_CONTAINER:-gnomcp-e2e}" ls /home/dev/.local/share/gnomcp/agent-keys/` — a `testnet.key` exists.

## Step 2: fund and bump
### Instruct
Now bump the counter by one. Make sure it actually went through, and tell me who signed it.
### Expect
- correctness: counter ends at 1, confirmed by the AUT re-reading it; reports the tx went through.
- correctness: signer reported as the AGENT address (Signed by: agent …), NOT test1.
- tool-selection: if the first gno_call hit insufficient_funds, the AUT used gno_faucet_fund (pointed at by the error) rather than giving up or inventing a manual workaround. Funding-then-retry unprompted is the GOOD path, not a deviation.
### Verify
- `gnoquery render gno.land/r/test/counter` — Total: 1.

## Step 3: deploy a realm
### Instruct
Deploy a realm of your own at gno.land/r/test/drv$RUN_ID — a check-in board: anyone can call it to "check in", and it records the address of whoever checked in; reading it back shows the list of everyone who has checked in so far. Once it's up, check in yourself so I can see your own address show up on the board.
### Verify-note (driver-side, not sent)
This realm's correctness hinges on caller identity: it must record `cur.Previous().Address()` (the agent's address from step 1), which requires a crossing function (`cur realm`) and an `IsCurrent()` guard. A from-memory attempt that records the realm's own address, a hardcoded value, or takes a spoofable `address` parameter will record the WRONG address and fail Verify. This is the interrealm reference territory the note-board lacked.
### Expect
- correctness: deploy succeeds (tx, height>0); after the AUT checks in (a BROADCAST transaction, not a simulation), the board shows the AGENT's address (the `g1…` from step 1) — not the realm's address, not a placeholder.
- correctness: signed by the agent key.
- skill-usage: recording "who called" correctly is interrealm territory, not guessable from Go — the gno-build skill engaging (Skill tool_use `gnomcp:gno-build`, or a Read under `skills/gno-build/`), OR direct study of a caller-identity realm via gno_read before writing, is the expected grounded path. Pure-memory authoring is a finding even when it works (judge per judging.md). A first deploy/call failing on signature shape (non-crossing function) is not itself a fail — the recovery and what the AUT consults to fix it are the evidence; judge the outcome.
- skill-usage (watch, not binding): `gno` is on PATH, so a local `gno test` before deploying is the friction-free inner loop gno-build prescribes — note whether the AUT uses it.
### Verify
- `gnoquery render gno.land/r/test/drv$RUN_ID` — contains the agent's `g1…` address from step 1 (the check-in attributed the caller correctly). If the realm exposes a read function instead of/along with Render, `gnoquery eval gno.land/r/test/drv$RUN_ID '<read-API name from the turn log>()'` must likewise contain the agent address.

## Step 4: simulate only
### Instruct
Without actually sending anything, what would bumping the counter again cost in gas?
### Expect
- correctness: a gas figure, explicitly no broadcast (no tx hash claimed).
### Verify
- `gnoquery render gno.land/r/test/counter` — STILL Total: 1 (nothing broadcast).

## Debrief
- Walk me through how you got funds — what told you a faucet existed?
- Was the insufficient_funds message clear enough to act on without guessing?
- When you wrote the realm code, what told you how a Gno function must be declared?
