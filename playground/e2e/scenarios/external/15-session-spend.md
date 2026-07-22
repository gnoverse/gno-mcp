---
id: session-spend
tier: external
category: sessions
image: l2-gnomcp
timeout-minutes: 20
covers: [external.session-spend, session.propose, session.authorize, write.signer-reporting]
---
# Session spend on the LIVE topaz testnet — a modest limit funds real writes

Driver context: the AUT runs `l2-gnomcp` (built-in `testnet` profile → live topaz-1,
chain gas price 10ugnot/1000gas at last authoring). This scenario pins the
fee/spend decoupling on a real network: a **1000000ugnot** spend limit — far too
small for a fee priced off a flat 200M gas ceiling, ample for fees priced off the
gas a light write actually reserves — must fund several session-signed writes.

Preflight (driver, before turn 1):
- Create a throwaway master key in a scratch gnokey home (`gnokey add`), fund it via
  the live agent faucet (`POST https://faucet-agent.topaz.testnets.gno.land/fund`,
  body `{"address": "<addr>", "chain_id": "topaz-1"}` — grants 10 GNOT), and confirm
  the balance via RPC before sending turn 1.
- Substitute `$MASTER_ADDR` (the funded address) in Instruct text exactly like
  `$RUN_ID`. This scenario has no fixed premined master — external chains have no
  test1.
- Record `FEE` = the live per-write fee: `ceil(10000000 × price) × 2` from
  `auth/gasprice` (equivalently what `gno_account`-era `GasFeeUgnot` reports;
  200000ugnot at 10/1000). All spend arithmetic below is in units of `FEE`.
- External tier: `blocked` (never `fail`) if topaz RPC or the faucet is down.
  Chain ground truth comes from the driver's own RPC queries
  (`auth/accounts/$MASTER_ADDR/session/<session>`), not gnoquery.

## Step 1: propose a modest session
### Instruct
My address on this testnet is $MASTER_ADDR. Set up a delegated session so you can run small gno scripts as me — spend limit 1000000ugnot, expiring in 24 hours. Give me the exact command I need to run to approve it, and tell me how many writes that budget buys at current prices.
### Expect
- correctness: the proposal is ACCEPTED (1000000ugnot is above the live per-write fee) — no rejection, no request to raise the limit.
- correctness: the answer states the per-write cost (= `FEE`) and a writes count consistent with `1000000 / FEE` (5 at 200000ugnot), and relays a `gnokey maketx session create` command whose `--gas-fee` is `FEE` and `--gas-wanted` is 10000000.
- tool-selection: gno_session_propose with `master_address` = $MASTER_ADDR and `allow_run` = true (scripts → MsgRun scope); the AUT never runs gnokey itself.
### Verify
- Turn log: a `gno_session_propose` tool_use with `.input.master_address` = $MASTER_ADDR, `.input.allow_run` = true, `.input.spend_limit` = "1000000ugnot".
- Turn log: no Bash tool_use invoking `gnokey`.

## Driver action (between Step 1 and Step 2): authorize as the user
Run the relayed `gnokey maketx session create` command from the scratch gnokey home
(append `--insecure-password-stdin --home <scratch>`; keep the AUT's `--gas-fee` /
`--gas-wanted`). Confirm tx success AND that the grant exists on chain
(`auth/accounts/$MASTER_ADDR/session/<session_address>` returns the record with
`spend_limit: 1000000ugnot`) before sending Step 2.

## Step 2: first session-signed write
### Instruct
Approved and confirmed on-chain. Now, acting as me through the session, run a tiny gno script that prints exactly hello-$RUN_ID — for real, not a dry run. Tell me what it printed, who signed it, and how much of my session budget is left.
### Expect
- correctness: the broadcast SUCCEEDS — no `session not allowed`, no spend-limit rejection.
- correctness: the reported output contains `hello-$RUN_ID`; the signer is honestly attributed as the session acting on behalf of $MASTER_ADDR (not the agent key).
- correctness: the reported remaining budget equals 1000000ugnot minus one `FEE` (800000ugnot at 200000).
- tool-selection: gno_run with `identity` = "session".
### Verify
- Turn log: a `gno_run` tool_use with `.input.identity` = "session".
- Chain (driver RPC): the session record's `spend_used` equals exactly one `FEE`, and its `sequence` is "1".

## Step 3: the budget keeps funding writes
### Instruct
Do one more — print again-$RUN_ID — and confirm what's left of the budget after that.
### Expect
- correctness: succeeds again; output contains `again-$RUN_ID`; remaining = 1000000ugnot minus two `FEE` (600000ugnot at 200000).
### Verify
- Chain (driver RPC): the session record's `spend_used` equals exactly two `FEE`; `sequence` is "2".

## Debrief
- How did you decide the 1000000ugnot limit was enough before broadcasting?
- Suppose I had asked for a 100000ugnot spend limit instead — what would have happened, and when would I have found out?
- Was anything in the propose output confusing or missing for judging the session's budget?
