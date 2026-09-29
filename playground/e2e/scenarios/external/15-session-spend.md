---
id: session-spend
tier: external
category: sessions
image: l2-gnomcp
timeout-minutes: 20
covers: [external.session-spend, session.propose, session.authorize, write.signer-reporting]
---
# Session spend on the LIVE onyx testnet — a modest limit funds real writes

Driver context: the AUT runs `l2-gnomcp` (built-in `testnet` profile → live onyx-1; chain gas
price 1ugnot/1000gas as of 2026-09-29 — re-read it, it is the value most likely to have moved).
onyx restricts `MsgRun` to an allowlist (`params/vm:p:run_submitters`), so the session-signed
writes here are calls: `gno.land/r/demo/profile` `SetStringField("Bio", …)`, a genesis realm
whose setter writes the caller's own profile field (here the master's) and touches no one else's.
This scenario pins the fee/spend decoupling on a real network: a **1000000ugnot** spend limit —
far too small for a fee priced off a flat 200M gas ceiling, ample for fees priced off the gas a
light write actually reserves — must fund several session-signed writes. The chain bills the
session each write's full gas fee plus any storage deposit the write locks: the first `Bio` write
creates the field and locks a deposit (about 108,200 ugnot on 2026-09-29), an update of the same
field locks none. The budget the AUT reports must match the chain's record, deposit included.

Preflight (driver, before turn 1):
- Create a throwaway master key in a scratch gnokey home (`gnokey add`), fund it via the live
  agent faucet (`POST https://faucet-agent.onyx.testnets.gno.land/fund`, body
  `{"address": "<addr>", "chain_id": "onyx-1"}`; the grant size is in `GET /limits`), and confirm
  the balance via RPC before sending turn 1. The balance decides: a faucet build older than the
  chain's release can answer 502 after its grant lands.
- Substitute `$MASTER_ADDR` (the funded address) in Instruct text exactly like `$RUN_ID`. This
  scenario has no fixed premined master — external chains have no test1.
- Record `FEE` = the live per-write fee: `ceil(10000000 × price) × 2` from `auth/gasprice`
  (20000ugnot at 1/1000). All spend arithmetic below is in units of `FEE`.
- External tier: `blocked` (never `fail`) if the onyx RPC or the faucet is down. Chain ground truth
  comes from the driver's own RPC queries (`auth/accounts/$MASTER_ADDR/session/<session>`, and
  `vm/qeval` `gno.land/r/demo/profile` `GetStringField("$MASTER_ADDR", "Bio", "")`), not gnoquery.

## Step 1: propose a modest session
### Instruct
My address on the testnet is $MASTER_ADDR. Set up a delegated session so you can update my profile on gno.land/r/demo/profile as me — spend limit 1000000ugnot, expiring in 24 hours. Give me the exact command I need to run to approve it, and tell me how many writes that budget buys at current prices.
### Expect
- correctness: the proposal is ACCEPTED (1000000ugnot is above the live per-write fee) — no rejection, no request to raise the limit.
- correctness: the answer states the per-write fee (= `FEE`) and a writes count consistent with `1000000 / FEE` (50 at 20000ugnot), and relays a `gnokey maketx session create` command whose `--gas-fee` is `FEE` and `--gas-wanted` is 10000000. Saying that the count is a fee-only ceiling, because a write that stores new data also locks a deposit, is a plus.
- tool-selection: gno_session_propose on the `testnet` profile with `master_address` = $MASTER_ADDR and `allow_paths` covering `gno.land/r/demo/profile`; the AUT never runs gnokey itself.
### Verify
- Turn log: a `gno_session_propose` tool_use with `.input.profile` = "testnet" (or absent → server default), `.input.master_address` = $MASTER_ADDR, `.input.allow_paths` containing "gno.land/r/demo/profile", `.input.spend_limit` = "1000000ugnot".
- Turn log: no Bash tool_use invoking `gnokey`.

## Driver action (between Step 1 and Step 2): authorize as the user
Run the relayed `gnokey maketx session create` command from the scratch gnokey home
(append `--insecure-password-stdin --home <scratch>`; keep the AUT's `--gas-fee` /
`--gas-wanted`). Confirm tx success AND that the grant exists on chain
(`auth/accounts/$MASTER_ADDR/session/<session_address>` returns the record with
`spend_limit: 1000000ugnot`) before sending Step 2.

## Step 2: first session-signed write
### Instruct
Approved and confirmed on-chain. Now, acting as me through the session, set my profile's Bio to hello-$RUN_ID — for real, not a dry run. Tell me who signed it and how much of my session budget is left.
### Expect
- correctness: the broadcast SUCCEEDS — no `session not allowed`, no spend-limit rejection.
- correctness: the signer is honestly attributed as the session acting on behalf of $MASTER_ADDR (not the agent key); the reported remaining budget equals 1000000ugnot minus the chain's `spend_used` after the write (one `FEE` plus the deposit the new field locked).
- tool-selection: gno_call with `identity` = "session" on the `testnet` profile, realm `gno.land/r/demo/profile`, func `SetStringField`.
### Verify
- Turn log: a `gno_call` tool_use with `.input.identity` = "session", `.input.realm` = "gno.land/r/demo/profile", `.input.func` = "SetStringField".
- Chain (driver RPC): the session record's `sequence` is "1" and its `spend_used` exceeds one `FEE` by the storage deposit; the AUT's reported remaining equals `1000000 - spend_used`.
- Chain (driver RPC): `GetStringField("$MASTER_ADDR", "Bio", "")` returns `hello-$RUN_ID`.

## Step 3: the budget keeps funding writes
### Instruct
Do one more — set my Bio to again-$RUN_ID — and confirm what's left of the budget after that.
### Expect
- correctness: succeeds again; an update of an existing field locks no new deposit, so the reported remaining drops by exactly one `FEE` and equals 1000000ugnot minus the chain's `spend_used`.
### Verify
- Chain (driver RPC): `sequence` is "2" and `spend_used` grew by exactly one `FEE` since step 2; the AUT's reported remaining equals `1000000 - spend_used`.
- Chain (driver RPC): `GetStringField("$MASTER_ADDR", "Bio", "")` returns `again-$RUN_ID`.

## Debrief
- How did you decide the 1000000ugnot limit was enough before broadcasting?
- Suppose I had asked for a 100000ugnot spend limit instead — what would have happened, and when would I have found out?
- Was anything in the propose output confusing or missing for judging the session's budget?
