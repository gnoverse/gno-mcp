---
id: faucet-limits
tier: local
category: fund
timeout-minutes: 20
image: e2e-faucetcap
covers: [read.faucet-limits, write.faucet-per-address-recovery]
---
# The agent learns the faucet's per-address policy, and recovers a per-address cap with a fresh key — not a global-outage stall

Driver-only context: this runs the `e2e-faucetcap` image — identical to `e2e` but the simnet faucet's
per-address cap is 1 (a second fund of the SAME address trips it). The point is the regression from a
real test13 session: an agent hit the per-address faucet cap, read the opaque rejection as a GLOBAL
outage ("the faucet is rate-limited, wait / fund manually"), stalled, and punted to the human — when
the fix was trivial (per-address → fund a fresh key). Two halves: (1) gno_status now surfaces the
faucet's policy so an oriented agent knows the limit up front; (2) the faucet 429 + gno_faucet_fund
error now name the per-address limit and the fresh-key recovery. Across-turn note: the agent KEY
persists (keystore file) and the faucet process persists (per-address cooldown survives the turn
boundary), so a re-fund in a later turn trips the same-address cap. The faucet IP is in-container
loopback (per-IP cap is high), so only the per-address cap fires.

## Step 1: discover-the-policy
### Instruct
Before I start spending, what does this testnet's faucet actually hand out per request, and is there any limit on how often I can hit it for the same account? I want to know what I'm working with.
### Expect
- correctness: reports the grant size (a concrete GNOT amount per request) AND a PER-ADDRESS limit (one grant per address per ~24h) — framed as a per-account cap, not a vague "there's some rate limit".
- tool-selection: the policy comes from a gno_status reading of the testnet profile, not a guess or a recalled-from-training number.
### Verify
- Turn log: a `gno_status` tool_use on the `testnet` profile whose result carries a `faucet` block with `per_address.max` = 1 and a non-zero `grant_ugnot`. The answer's stated per-address limit matches that block (not a fabricated figure).

## Step 2: set-up-a-funded-account
### Instruct
Okay, set me up an agent account named `run-$RUN_ID` and fund it from the faucet so it's ready to transact. Tell me its address and that it's funded.
### Expect
- correctness: reports a fresh `g1…` address (its own agent key, not `test1`) and that the faucet grant succeeded — an automatic service grant, not a manual "go to this page" fallback.
- tool-selection: gno_key_generate created `run-$RUN_ID`; gno_faucet_fund funded it on the testnet profile.
### Verify
- Turn log: a `gno_key_generate` for key `run-$RUN_ID` and a `gno_faucet_fund` for that key on `testnet`; the fund result reports the address funded.
- `gnoquery balance <address from the answer>` — a positive ugnot balance (the first grant landed).

## Step 3: hit-the-cap-and-recover
### Instruct
Can you grab another faucet grant for that same `run-$RUN_ID` account? I'd like a bigger balance on it to work with.
### Expect
- correctness (binding): the AUT attributes the refusal to a PER-ADDRESS limit — that this specific account has already drawn its grant for the window — NOT to a global faucet outage / "the faucet is down" / "rate-limited, try again later for everyone".
- correctness (binding): the AUT surfaces a correct recovery — a different/new key has its own allowance (it does not strand on "wait ~24h" as the only option, and it does NOT punt to "fund it manually yourself" as the sole path). Actually generating + funding a fresh key to demonstrate recovery is a strong pass; clearly explaining the per-address-vs-new-key model is an acceptable pass.
- tool-selection: a second gno_faucet_fund on `run-$RUN_ID` (the same key) is what surfaces the limit; any recovery funding targets a DIFFERENT key.
### Verify
- Turn log: a `gno_faucet_fund` tool_use for key `run-$RUN_ID` whose result is an error naming the per-address limit (mentions the address already drew its grant / a fresh key via gno_key_generate / retry after ~24h) — i.e. the structured per-address message, not a generic "faucet busy".
- (watch) Turn log: if the AUT recovers by funding, a `gno_key_generate` + `gno_faucet_fund` for a NEW key (a different name than `run-$RUN_ID`) that succeeds — a fresh address has its own allowance.

## Debrief
- When the second faucet request was refused, how did you know whether it was that one account that was capped, or the whole faucet being down?
- What told you the limit was per-address — the faucet's response, something you checked beforehand, or a guess?
- If you needed another funded account right then, what was the fastest way to get one?
