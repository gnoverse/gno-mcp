---
id: deploy-gates
tier: external
category: writes
image: l2-gnomcp
timeout-minutes: 25
covers: [write.key-generate, write.faucet-fund, write.addpkg, write.deploy-gates, external.cla-sign, write.cla-sign-tool]
---
# Deploying a realm on LIVE sapphire, against whatever the chain's sys gates currently require

Driver context: the AUT runs the `l2-gnomcp` image (gnomcp + gno skill, no `profiles.toml`
override), so the built-in `testnet` profile is the real network — chain-id `sapphire-1`, RPC
`https://rpc.sapphire.testnets.gno.land:443`, faucet
`https://faucet-agent.sapphire.testnets.gno.land`. This scenario exists because the simnet has the
deploy gates OFF, so only the live testnet exercises the real keeper path: namespace authorization,
plus the CLA signature **when that gate is enforced**. The agent's own-address namespace
(`r/<its-g1addr>/*`) is always authorized, so the namespace gate is the one that always applies.
The point under test is that the agent discovers what the chain requires and clears it ITSELF — the
Instruct deliberately never mentions the CLA, the namespace, or a preflight.

**The CLA gate is a chain setting, not a constant.** `vm/qrender` on `gno.land/r/sys/cla` reports
"CLA enforcement is currently DISABLED" on sapphire and topaz alike as of 2026-08-09, so a deploy
today needs no `Sign` and an agent that skips it is CORRECT, not lazy. Read the gate state before
judging Step 2 (`gno_cla_info`, or the driver's own render query) and apply the matching branch
below. Do not fail an agent for omitting a signature the chain does not ask for; equally, do not
credit `external.cla-sign` coverage on a run where the gate was off — mark that dimension
`blocked`.

External tier: needs egress to sapphire and polls the live faucet (~10–60s). Mark a step `blocked`
(never `fail`) if the live faucet/chain/`r/sys/cla` is unreachable or rate-limits — that is an
outage outside the AUT, not an agent error. No `gnoquery` for external chains: verify on-chain
results through the AUT's own `gno_render` / `gno_account` reads (turn-log: the tool was called and
reported the expected state). If the gate is enforced, this scenario signs the real sapphire CLA
with a throwaway agent key — intended behavior, the key is disposable.

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

If the AUT presents a CLA/agreement URL and asks whether to proceed, reply exactly once:
"yes, I've read it — go ahead and sign".

### Instruct
Now deploy a small realm of your own on the testnet — a tally board: anyone can bump a counter, and reading it back shows the current count. Put it under your own namespace. Get it live, bump it once yourself, and show me the count went up.
### Expect
- correctness: the realm is actually deployed and live on sapphire, and a real bump-then-read by the agent shows the count increased (e.g. reads `1`) — confirmed by the AUT re-reading the realm, not asserted from memory.
- correctness: it deployed into a namespace it is authorized for — its OWN-address namespace (`gno.land/r/<the step-1 agent address>/…`, the zero-friction path), OR a name it registered in this same run. It did not try to squat an unrelated namespace (e.g. `r/test/…`).
- tool-selection: `gno_addpkg` performed the deploy, and the agent cleared whatever the chain actually required ITSELF — it did not give up, tell the user to run the signing themselves, or fall back to raw `gnokey`/curl.
- tool-selection, CLA gate ENFORCED only: the signature came from `gno_cla_info` + `gno_cla_sign` (fetch, present the agreement URL to the user, confirm, sign; pausing for that confirmation is the GOOD path, not a deviation), with a `gno_call` to `gno.land/r/sys/cla` func `Sign` from the agent key as the accepted fallback.
- tool-selection, CLA gate DISABLED only: no signature is required. Deploying straight through is correct. Checking the gate first (a `gno_cla_info` call) is a bonus, not a requirement — and signing anyway is not a failure, just unnecessary.
- skill-usage: the gno skill family engaged for this realm-authoring + deploy task (a `Skill` tool_use for `gno-build`, or a `Read` under `skills/gno/` references).
### Verify
- Read the live gate state first (`gno.land/r/sys/cla` render: "enforcement is currently DISABLED" or a Required Hash row), then apply exactly one branch:
  - ENFORCED → Turn log: a `gno_cla_sign` tool_use (preferred path), OR a `gno_call` targeting `gno.land/r/sys/cla` func `Sign` (fallback). Absence is a `fail`.
  - DISABLED → no CLA tool_use is required; its absence must NOT be held against the AUT. Record the `external.cla-sign` and `write.cla-sign-tool` dimensions as `blocked` for this run — the chain gave nothing to exercise.
- Turn log: a `gno_addpkg` tool_use whose `.input.deploy_path` is a namespace the agent is authorized for — either `gno.land/r/<the Step-1 agent address>/…` (own-address, the expected path), or `gno.land/r/<name>/…` for a `<name>` the agent registered earlier in this run (a `gno_call` to `r/sys/namereg/v1` func `Register` is in the log). A deploy under `r/test/…` or a name it never registered is a fail.
- The AUT's own `gno_render` (or `gno_read`) of the deployed path, in a turn AFTER the deploy, shows the tally at the bumped value. External: trust the AUT's read of its own deployment; do not reach for `gnoquery`.
- Universal hard-fail still applies: if the AUT itself invokes `gnokey` (a `Bash` tool_use whose command contains `gnokey`), the step is `fail`.
- blocked (not fail) if sapphire or `r/sys/cla` is unreachable mid-flow.

## Debrief
- On sapphire, what (if anything) stopped your first deploy from going through, and how did you get past it?
- Did the chain require a CLA signature for this deploy? How did you find out, and where did any hash you used come from?
- Did you check what the deploy required before attempting it, or did you try the deploy and react to the failure? Either is fine — I want to know which.
- Anything about clearing those deploy requirements you'd make smoother for someone deploying their first realm on a real gno.land network?
