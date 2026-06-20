---
id: faucet-cold-recovery
tier: local
category: fund
timeout-minutes: 20
image: e2e-faucetcap
covers: [write.faucet-per-address-recovery]
---
# Cold per-address cap: the agent hits the faucet limit WITHOUT prior discovery and must recover, not stall

Driver-only context: the harder twin of scenario 10. Runs the `e2e-faucetcap` image (per-address cap = 1).
Unlike 10, this scenario NEVER mentions limits — it is a plain funding task, so the agent comes into the
re-fund with no policy in context and must interpret the per-address rejection cold. This reproduces the
real test13 regression condition: an agent that hits the cap without having checked the faucet's limits
first. The binding question is the recovery: does the agent read the rejection as a per-ADDRESS cap (this
one account is spent; a fresh key has its own allowance) and recover, or misread it as a GLOBAL outage
("the faucet is down / rate-limited, wait and retry / fund it manually yourself") and stall. This is the
A/B vehicle for proving the structured-429 fix: baseline (the opaque "faucet busy — retry later" message)
vs treatment (the structured per-address message + gno_status discovery). Per-run isolation: each session
uses a distinct key, so the per-address cooldown never collides across runs in one container.

## Step 1: fund-for-a-task
### Instruct
I'm going to start deploying some realms on this testnet today. Set up an agent account named `run-$RUN_ID` and fund it from the faucet so I'm ready to go, then tell me its address.
### Expect
- correctness: reports a fresh `g1…` address (its own agent key, not `test1`) and that it's funded — an automatic faucet grant, not a manual fallback.
- tool-selection: gno_key_generate for `run-$RUN_ID` + gno_faucet_fund on the testnet profile.
### Verify
- Turn log: a `gno_key_generate{key:run-$RUN_ID}` and a `gno_faucet_fund{key:run-$RUN_ID}` on `testnet`; the fund result reports the address funded.
- `gnoquery balance <address from the answer>` — a positive ugnot balance.

## Step 2: cold-re-fund-and-recover
### Instruct
Actually, before I start — grab another faucet grant for that same `run-$RUN_ID` account so I've got extra headroom to work with.
### Expect
- correctness (binding): when the re-fund is refused, the AUT reads it as a PER-ADDRESS / per-account limit (this specific account has already drawn its grant) — NOT as a global faucet outage, "the faucet is rate-limited / down, wait and retry later", or a manual-funding-only fallback.
- correctness (binding): the AUT surfaces or performs a correct recovery — a different/new key has its own allowance (generate + fund a fresh key, optionally gno_key_send onto the original). It does not stall on "wait ~24h" as the only option, and does not present manual funding as the sole path.
- correctness (watch): whether the AUT actually attempts the re-fund (the cold path) vs declines up front by recalling a limit it discovered on its own — both are acceptable if the per-address reasoning and recovery are correct; note which for the debrief.
### Verify
- Turn log: a `gno_faucet_fund{key:run-$RUN_ID}` re-attempt whose result is the per-address rejection; the AUT's answer attributes it to that account being capped (not a global outage) and names the fresh-key recovery.
- (watch) if the AUT recovers by funding, a `gno_key_generate` + `gno_faucet_fund` for a NEW key (a different name) that succeeds.

## Debrief
- When the faucet refused the second grant, how did you read it — that one account was capped, or the whole faucet was unavailable? What in the response told you which?
- If you needed that account to have more to work with, what was your path forward?
