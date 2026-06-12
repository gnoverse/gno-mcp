---
id: local-gnodev
tier: local
category: localdev
timeout-minutes: 30
covers: [localdev.gnodev-start, localdev.local-profile, localdev.test1-signing, write.call, write.run, read.render, skill.ref-build]
---
# Local dev loop: agent-started gnodev, local profile, test1 signing, MsgRun

The AUT must start gnodev ITSELF (never auto-started; binary on PATH; realm
sources baked at /home/dev/realms). Ports: gnodev RPC :26657, gnodev's web
:8888 — distinct from simnet (:26687/:8688). Known trap at the pinned gno
commit: gnodev IGNORES positional package-dir args; loading a tree needs
`-resolver root=<tree> -paths "<pkg>,…"`. An AUT that starts gnodev bare and
later finds the counter missing must diagnose and recover — flailing on the
resolver is a finding and a lead for the skill's build.md. gnodev runs as a
background process the AUT launched; if it dies between turns, detecting and
restarting it is part of the flow being tested (judge recovery, not the
death). Watch: skill/build.md consulted for gnodev usage (watch-level, not
binding); chain dev signs with built-in test1 — no faucet, no key generation
expected here.

## Step 1: spin up a devnet
### Instruct
I want to iterate on my realm sources in ~/realms locally before touching any shared chain. Spin up a local devnet that serves them and tell me when it's ready.
### Expect
- correctness: a gnodev devnet is running and reachable on :26657 reporting chain-id dev, with the ~/realms packages loaded (the AUT's readiness claim must be grounded in an actual check, not just "I started it").
- tool-selection: gnodev started via shell (expected — it's a host process, not an MCP op); chain verification via gnomcp (gno_status/render on profile local) is the natural follow-up.
### Verify
- `gnoquery -rpc http://127.0.0.1:26657 status` — chain-id is "dev" (the gnodev node).

## Step 2: write on it
### Instruct
Bump the counter on the local devnet twice, then tell me what it shows and who signed those transactions.
### Expect
- correctness: counter on the LOCAL chain reads Total: 2; both bumps landed on chain dev (not on the testnet profile / simnet — cross-chain leakage is the failure this step watches for).
- correctness: signer reported as the built-in test1 key (local-profile signing) — not an agent key, no faucet involved.
- tool-selection: gno_call with profile local (or equivalent explicit profile choice).
### Verify
- `gnoquery -rpc http://127.0.0.1:26657 render gno.land/r/test/counter` — reads `Total: 2` on the gnodev (`dev`) chain.
- `gnoquery render gno.land/r/test/counter` — the SIMNET counter (default RPC :26687) is unchanged from before this scenario (no cross-chain leakage).

## Step 3: two bumps, one transaction
### Instruct
Nice. Now do the next two bumps as a single transaction — I don't want two separate ones this time.
### Expect
- correctness: local counter reads Total: 4 and the AUT confirms it was ONE transaction.
- tool-selection: gno_run (MsgRun script invoking Increment twice) — two separate gno_call broadcasts violates the instruction and fails.
### Verify
- `gnoquery -rpc http://127.0.0.1:26657 render gno.land/r/test/counter` — reads `Total: 4` on the gnodev (`dev`) chain.
- The turn log shows a `gno_run` tool_use in turn 3 (one MsgRun, not two gno_call broadcasts).

## Debrief
- How did you figure out the right way to load ~/realms into gnodev?
- Did anything tell you which key signs on a local chain, or did you find out by doing it?
- For the single-transaction request, what options did you consider?
