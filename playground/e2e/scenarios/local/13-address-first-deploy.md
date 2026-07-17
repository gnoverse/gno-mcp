---
id: address-first-deploy
tier: local
category: writes
timeout-minutes: 15
covers: [write.addpkg, write.short-name-expansion, write.address-namespace]
---
# Deploy with a short package name — auto-expansion to address-based namespace

Tests the TOOL-side expansion: `gno_addpkg` given a bare package name (no `/`) must expand it
to `gno.land/r/<agent_address>/<name>` — always authorized, gnoweb-safe — and regenerate the
auto-injected `gnomod.toml` for the expanded path. The Instruct pins the short form on purpose
(direct-invocation style): an AUT that hand-builds the full path itself deploys fine but leaves
the expansion unexercised, which is what this scenario exists to cover.

## Step 1: deploy with a short name
### Instruct
Deploy a minimal realm for me. Call the package "ping$RUN_ID" — it should expose a Render function that just returns the string "pong". Pass just the bare package name to the deploy tool as the path — no need to work out the full path yourself.
### Expect
- correctness: deploy succeeds with a tx hash and height > 0, and the reported deployed path is under the agent's own g1… address (not a registered name, not r/test/).
- tool-selection: `gno_addpkg` called with the SHORT form — `deploy_path` is "ping$RUN_ID", not a hand-built full path (the Instruct pins this; expanding manually defies it and leaves the feature unexercised).
### Verify
- Turn log: a `gno_addpkg` tool_use whose `.input.deploy_path` == `ping$RUN_ID`.
- Chain ground truth: `gnoquery render gno.land/r/<agent-address>/ping$RUN_ID` returns "pong" (take the agent address from the AUT's answer or its key-tool output in the turn log).

## Step 2: read it back
### Instruct
Read the realm back to confirm it's live.
### Expect
- correctness: confirms the realm is live on-chain with content "pong" — by rendering it or reading its source.
- tool-selection: used a gnomcp read tool on the deployed path (gno_render, gno_eval, or gno_read), not curl/raw RPC.
### Verify
- Chain ground truth: `gnoquery render <deployed path>` returns "pong".
- Turn log: a gnomcp read tool_use (gno_render / gno_eval / gno_read) whose path matches the deployed realm.

## Debrief
- Did you need to know your own address before deploying, or did the tool handle it?
- What would the path have been if you had typed the full path yourself?
