---
id: multi-key
tier: local
category: writes
timeout-minutes: 30
covers: [write.key-multi, write.key-selector, write.key-send, write.key-list, write.key-delete, write.signer-reporting]
---
# Multiple named agent keys: generate a second key, fund it by transfer, transact as both, list, delete

The multi-key write path on simnet (test9999 is testnet-tier, so named keys + gno_key_send apply).
A profile holds several named agent keys; the `key` arg selects which one signs. Run AFTER
write-tools (02) is green — the keystore persists in the container, so a `default` key likely
already exists; this scenario adds a second one.
Watch: the instruction says move funds FROM the main account — the intended path is gno_key_send
(an own-keys transfer), NOT a second gno_faucet_fund on the new key. Funding the new key from the
faucet instead of transferring is a deviation from the ask (note for the debrief; judge per intent).
Watch: does the AUT lean on gno_key_list to recall which accounts exist, or track them in-context.

## Step 1: a second, transfer-funded account
### Instruct
I want to test a realm that treats different callers differently, and I want you to drive it from more than one account. Set yourself up a second account on this testnet, and move some funds into it from your main account so it can transact. When you're set, tell me both of your addresses.
### Expect
- correctness: reports TWO distinct bech32 g1… addresses, both NOT g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5 (test1).
- correctness: the second account is funded by a TRANSFER from the first (per "move funds from your main account") and ends with a positive balance — not by the faucet.
- tool-selection: gno_key_generate for a NAMED second key (a `key`/name other than "default"); gno_key_send to move ugnot main→second. Faucet-funding the MAIN key first (if it was unfunded) is fine; faucet-funding the SECOND key instead of transferring is the deviation to note.
### Verify
- `docker exec "${E2E_CONTAINER:-gnomcp-e2e}" ls /home/dev/.local/share/gnomcp/agent-keys/testnet/` — two `.key` files (default.key + the named second key).
- `gnoquery balance <second address from the answer>` — a positive ugnot balance (the transfer landed).

## Step 2: transact as both accounts
### Instruct
Now deploy a check-in board at gno.land/r/test/team$RUN_ID — anyone who calls it gets recorded by their address, and reading it back lists everyone who has checked in. Then check in once from EACH of your two accounts, so I can see both of your addresses on the board.
### Verify-note (driver-side, not sent)
Same interrealm territory as 02 step 3: the board must record `cur.Previous().Address()` (a crossing function + `IsCurrent()` guard), so the recorded address is the actual caller. The new dimension here is TWO distinct callers — the second check-in must be signed by the second key (the `key` selector), or both rows show the same address and Verify fails.
### Expect
- correctness: deploy succeeds (tx, height>0); after both check-ins (BROADCAST, not simulated), the board shows BOTH agent addresses from step 1, distinct.
- tool-selection: TWO gno_call check-ins signed by DIFFERENT keys — the AUT uses the `key` arg to sign one check-in as the second account. Signer reporting names the two different agent addresses across the two calls (not test1, not the same address twice).
- skill-usage: recording "who called" is interrealm territory, not guessable from Go — the gno-build skill engaging (Skill tool_use `gnomcp:gno-build`, or a Read under `skills/gno-build/`), OR direct study of a caller-identity realm via gno_read before writing, is the grounded path. A first attempt failing on signature shape is not itself a fail — judge the recovery and outcome.
### Verify
- `gnoquery render gno.land/r/test/team$RUN_ID` — contains BOTH g1… addresses reported in step 1 (each check-in attributed to its own signing key).

## Step 3: recall the accounts
### Instruct
Remind me — which accounts have you got set up on this testnet right now, and what are their addresses?
### Expect
- correctness (binding): lists exactly the TWO accounts from step 1 (both addresses), ideally with their names.
- tool-selection (watch, not binding): whether the AUT confirms via gno_key_list or answers from session memory — both are legitimate within one session (the addresses are in context from step 1). Note which for the debrief; gno_key_list is the cross-session-correct habit.
### Verify
- (turn-log) the answer names both g1… addresses from step 1, and no third account. (If a `gno_key_list` tool_use appears, its result should match — but the binding fact is the answer's account set.)

## Step 4: remove the second account (funds-safe deletion)
### Instruct
Okay, I'm done with that second account — get rid of it, then show me what's left.
### Verify-note (driver-side, not sent)
The second account still holds funds. gno_key_delete refuses a funded key (key_has_funds, unit-tested) and the error names a sweep target; force=true abandons the balance and the result reports abandoned_balance_ugnot. The GOOD path is sweep-the-full-balance then delete. The recovery is error-prone for an agent: it must sweep the LIVE balance (not a remembered amount), so watch whether the AUT verifies the balance (gno_account / gno_key_send) before sweeping vs. strands funds. This is the lead that may motivate an atomic sweep-on-delete affordance.
### Expect
- correctness (binding): ends with only the main account remaining (the second key is deleted).
- correctness (binding): if any balance is abandoned, the delete RESULT says so explicitly (abandoned_balance_ugnot / "abandoning N ugnot") — the loss is never silent.
- correctness (watch): whether the AUT recovers the funds (sweeps the full balance to main, leaving ~0) or strands them by sweeping a stale/partial amount and force-deleting. Stranding is a finding, not an automatic fail — the manual-sweep recovery is the known-fragile part.
- tool-selection (watch): a sweep via gno_key_send before delete; whether the AUT checks the live balance first (gno_account) rather than trusting its own bookkeeping.
### Verify
- `docker exec "${E2E_CONTAINER:-gnomcp-e2e}" ls /home/dev/.local/share/gnomcp/agent-keys/testnet/` — only the main key file remains (the second is gone).
- (turn-log, binding) if the AUT force-deleted a funded key, its `gno_key_delete` result carries `abandoned_balance_ugnot` matching the second account's balance at delete time — the abandonment was surfaced, not silent.
- (watch) `gnoquery balance <second address from step 1>` — `0ugnot` means a clean full sweep; a non-zero stranded balance is the recovery-fragility finding.

## Debrief
- How did you move funds into the second account — which tool, and could you have funded it another way?
- When you checked in from each account, how did you tell the board (and gnomcp) which account was calling?
- What did deleting the second account do to any funds it still held?
