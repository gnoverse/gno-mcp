---
id: address-first-deploy
tier: local
category: writes
timeout-minutes: 15
covers: [write.addpkg, write.short-name-expansion, write.address-namespace]
---
# Deploy with a short package name — auto-expansion to address-based namespace

Tests that the agent can deploy a Gno realm using only a short package name
(e.g. "ping") rather than a fully-qualified path. The tool should expand it
automatically to gno.land/r/<agent_address>/<name>, which is always authorized
and gnoweb-compatible.

## Step 1: deploy with a short name
### Instruct
Deploy a minimal realm for me. Call the package "ping" — it should expose a Render
function that just returns the string "pong". Use the shortest path you can.
### Expect
- correctness: deploy succeeds with a tx hash and height > 0.
- correctness: the deployed path contains the agent's own g1… address (not a registered name, not r/test/).
- tool-selection: gno_addpkg was called with deploy_path="ping" (the short form) OR an address-based path the tool built automatically.
### Verify
- Turn log: `gno_addpkg` tool_use present.
- Turn log: the `.output` of that call contains a path matching `gno.land/r/g1[a-z0-9]+/ping` — the address-expanded form.

## Step 2: read it back
### Instruct
Read the realm back to confirm it's live.
### Expect
- correctness: returns "pong" or a render containing "pong".
- tool-selection: gno_render or gno_eval called on the deployed path.
### Verify
- Turn log: `gno_render` (or `gno_eval`) tool_use whose path matches the deployed realm.
- Turn log: result text contains "pong".

## Debrief
- Did you need to know your own address before deploying, or did the tool handle it?
- What would the path have been if you had typed the full path yourself?
