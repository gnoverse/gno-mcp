---
id: faucet
tier: external
category: fund
image: l2-gnomcp
timeout-minutes: 20
covers: [write.key-generate, write.faucet-fund, external.faucet-live, external.testnet-key-cycle]
---
# Funding an agent on the LIVE test13 testnet via the built-in faucet service

Driver context: the AUT runs the `l2-gnomcp` image — gnomcp + the gno skill installed, but NO
`profiles.toml` override (unlike the simnet `e2e` image), so the built-in `testnet` profile resolves
to the real network: chain-id `test-13`, RPC `https://rpc.test13.testnets.gno.land:443`,
faucet-service-url `https://faucet-agent.test13.testnets.gno.land`. This is the only scenario that
drives the deployed agent-faucet end to end, and it validates the built-in `faucet-service-url`
default. External tier: it needs egress to test13 and polls the live faucet (~10–60s). Mark `blocked`
(never `fail`) if the live faucet/chain is unreachable or rate-limits the request — that is an
outage outside the AUT, not an agent error. No `gnoquery` for external chains: verify funding through
the AUT's own `gno_account` reading (turn-log: the tool was called and reported a non-zero balance).
No gnokey or master account is involved — this is agent-key funding only.

## Step 1: generate-agent-key
### Instruct
I want to start transacting on the testnet. Set up an agent key named `run-$RUN_ID` for me and tell me its address.
### Expect
- correctness: reports a fresh bech32 `g1…` address (its own agent key, not `test1`).
- tool-selection: gno_key_generate created the key on the testnet profile.
### Verify
- Turn log: a `gno_key_generate` tool_use whose `.input.key` is `run-$RUN_ID` and `.input.profile` is `testnet` (or absent → server default is testnet).

## Step 2: fund-and-confirm
### Instruct
Now fund `run-$RUN_ID` from the faucet so it has some GNOT to spend, then check its balance and tell me what it holds.
### Expect
- correctness: the faucet request succeeds — the answer reports the address is funded, not a manual "open this page and do it yourself" fallback.
- correctness: a follow-up balance check shows a non-zero GNOT balance for that address.
- tool-selection: gno_faucet_fund performed the funding (not a hand-rolled curl/RPC); gno_account read the balance back.
### Verify
- Turn log: a `gno_faucet_fund` tool_use for key `run-$RUN_ID` on the testnet profile; its result text reports the address funded — an automatic service grant, not merely a faucet link.
- Turn log: a `gno_account` tool_use for the agent's address; the result text shows a non-zero `ugnot` balance.

## Debrief
- Which faucet did you use, and how did you know to use it?
- Did the funding happen automatically, or did you fall back to a manual faucet page or instructions?
- Anything about the fund-then-verify flow you'd improve for someone funding an agent on a real testnet?
