---
id: connect-flow
tier: local
category: connect
timeout-minutes: 15
covers: [admin.profile-add-discovery, admin.profile-add-verify, admin.persist-hint, profile.read-attribution, misc.gnoweb-metadata]
---
# B3 connect flow: discover a chain from its gnoweb URL via gno_profile_add

simnet's gnoweb (http://testnet.gnomcp.sim:8688) advertises gnoconnect meta-tags
(rpc http://testnet.gnomcp.sim:26687, chainid test9999). The pre-configured `testnet`
profile points at the same chain — the AUT is asked to connect "a friend's chain" under
a NEW name, exercising discovery, live chain-id verification, and a read through the
new profile. Connect + read are ONE step deliberately: dynamic profiles are in-memory
per gnomcp process, and each headless turn spawns a fresh gnomcp — a cross-turn
connect-then-read cannot pass until the product grows a persistence story (recorded
product finding, 2026-06-10 connect-flow runs). Watch: gno_profile_add with
gnoweb_url (not curl-and-hand-parse), and the read carrying profile=friendchain.

## Step 1: connect from a gnoweb URL and read through it
### Instruct
A friend runs their own gno chain — all I have is their gnoweb page: http://testnet.gnomcp.sim:8688 — get me connected to it under the profile name friendchain, and tell me what gno.land/r/test/counter shows over there.
### Expect
- correctness: a profile named friendchain is created (chain-id test9999 reported) AND the counter render value is reported and matches ground truth.
- tool-selection: gno_profile_add called WITH gnoweb_url (discovery) — curling the page and hand-parsing meta-tags into rpc_url is capability-ignored (this step's point).
- tool-selection: the render/read call carries profile=friendchain (not the default testnet profile, even though it is the same chain).
### Verify
- The turn log shows a `gno_profile_add` tool_use (the user asked to add the chain) — establish from the transcript schema (verify-toolkit.md).
- `gnoquery render gno.land/r/test/counter` — matches the AUT's reported value.

## Debrief
- What did the gnoweb page give you, and what did gnomcp verify on its own?
- If the page had advertised an unreachable RPC, what would you have done?
- If I came back tomorrow in a fresh session and asked about that friendchain again, what would happen?
