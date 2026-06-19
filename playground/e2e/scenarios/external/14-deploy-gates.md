---
id: deploy-gates
tier: external
category: writes
image: l2-gnomcp
timeout-minutes: 25
covers: [write.key-generate, write.faucet-fund, write.addpkg, write.deploy-gates, external.cla-sign]
---
# Deploying a realm on LIVE test13, where the chain's sys gates actually bite

Driver context: the AUT runs the `l2-gnomcp` image (gnomcp + gno skill, no `profiles.toml`
override), so the built-in `testnet` profile is the real network — chain-id `test-13`, RPC
`https://rpc.test13.testnets.gno.land:443`, faucet `https://faucet-agent.test13.testnets.gno.land`.
This scenario exists because the simnet has the deploy gates OFF, so only the live testnet exercises
them: on test13 a package deploy must clear two genesis-activated keeper gates — namespace
authorization, then the CLA signature. The agent's own-address namespace (`r/<its-g1addr>/*`) is
always authorized, so the namespace gate passes and the **unsigned CLA** is the real blocker; the
agent must sign `r/sys/cla` from its own key (an ordinary `gno_call`, no human) before the deploy
lands. The point under test is that the agent discovers and clears these gates ITSELF — the Instruct
deliberately never mentions the CLA, the namespace, or a preflight.

External tier: needs egress to test13 and polls the live faucet (~10–60s). Mark a step `blocked`
(never `fail`) if the live faucet/chain/`r/sys/cla` is unreachable or rate-limits — that is an
outage outside the AUT, not an agent error. No `gnoquery` for external chains: verify on-chain
results through the AUT's own `gno_render` / `gno_account` reads (turn-log: the tool was called and
reported the expected state). This scenario signs the real test13 CLA with a throwaway agent key —
intended behavior, the key is disposable.

## Step 1: own identity, funded
### Instruct
I want to deploy something on the testnet myself. Set up your own agent key named `run-$RUN_ID` and make sure it has some GNOT to spend, then tell me its address and what it holds.
### Expect
- correctness: reports a fresh bech32 `g1…` address that is its own agent key, NOT `test1` (`g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5`).
- correctness: the key ends up funded — a follow-up balance read shows a non-zero GNOT balance; the faucet grant succeeded, not a "go fund it yourself" fallback.
- tool-selection: `gno_key_generate` created the key and `gno_faucet_fund` funded it (not a hand-rolled curl/RPC), `gno_account` read the balance back.
### Verify
- Turn log: a `gno_key_generate` tool_use with `.input.key` == `run-$RUN_ID` on the testnet profile (or profile absent → server default is testnet).
- Turn log: a `gno_faucet_fund` for that key reporting an automatic grant, and a `gno_account` whose result text shows a non-zero `ugnot` balance.
- blocked (not fail) if the live faucet is unreachable or rate-limits the request.

## Step 2: deploy under your own namespace
### Instruct
Now deploy a small realm of your own on the testnet — a tally board: anyone can bump a counter, and reading it back shows the current count. Put it under your own namespace. Get it live, bump it once yourself, and show me the count went up.
### Expect
- correctness: the realm is actually deployed and live on test13, and a real bump-then-read by the agent shows the count increased (e.g. reads `1`) — confirmed by the AUT re-reading the realm, not asserted from memory.
- correctness: it deployed into a namespace it is authorized for — its OWN-address namespace (`gno.land/r/<the step-1 agent address>/…`, the zero-friction path), OR a name it registered in this same run. It did not try to squat an unrelated namespace (e.g. `r/test/…`).
- tool-selection: `gno_addpkg` performed the deploy and the agent cleared the chain's deploy requirement ITSELF by signing `gno.land/r/sys/cla` from the same key — it did not give up, ask the user to sign, or fall back to raw `gnokey`/curl. (Signing `r/sys/cla` from the agent key via `gno_call` is the GOOD path here, not a deviation.)
- skill-usage: the gno skill family engaged for this realm-authoring + deploy task (a `Skill` tool_use for `gno-build`, or a `Read` under `skills/gno/` references).
### Verify
- Turn log: a `gno_call` tool_use targeting `gno.land/r/sys/cla` func `Sign` (the agent signing the CLA from its own key) — this is what unblocks the deploy on test13.
- Turn log: a `gno_addpkg` tool_use whose `.input.deploy_path` is a namespace the agent is authorized for — either `gno.land/r/<the Step-1 agent address>/…` (own-address, the expected path), or `gno.land/r/<name>/…` for a `<name>` the agent registered earlier in this run (a `gno_call` to `r/sys/namereg/v1` func `Register` is in the log). A deploy under `r/test/…` or a name it never registered is a fail.
- The AUT's own `gno_render` (or `gno_read`) of the deployed path, in a turn AFTER the deploy, shows the tally at the bumped value. External: trust the AUT's read of its own deployment; do not reach for `gnoquery`.
- Universal hard-fail still applies: if the AUT itself invokes `gnokey` (a `Bash` tool_use whose command contains `gnokey`), the step is `fail`.
- blocked (not fail) if test13 or `r/sys/cla` is unreachable mid-flow.

## Debrief
- On test13, what (if anything) stopped your first deploy from going through, and how did you get past it?
- Where did the CLA hash you signed come from — did you read it off-chain, from memory, or from the realm itself?
- Did you check what the deploy required before attempting it, or did you try the deploy and react to the failure? Either is fine — I want to know which.
- Anything about clearing those deploy requirements you'd make smoother for someone deploying their first realm on a real gno.land network?
