# E2E Protocol — Agent Faucet (SPEC2 / B2)

Manual protocol verifying the tiered faucet (`gno_faucet_fund` + the `agentfaucet` service) against a live gnodev.

The **tier-2 dispense path** is also covered automatically by `TestIntegration_faucetDispense`
(`go test -tags=integration ./test/integration/`) — service → `bank.MsgSend` → real in-process node →
balance rises → cooldown 429. This manual protocol covers the **human-facing** tiers (1 / fallback)
and the end-to-end agent flow the automated test can't drive (key generation, the MCP tool surface,
the funded write).

## Prerequisites

- `gnodev` running with a **test-tier chain-id** (e.g. `test5`) so the agent uses a *generated* testnet
  key, not test1. Note its RPC URL + chain-id.
- A gnomcp profile pointing at it. Example `profiles.toml`:
  ```toml
  [testdev]
  chain-type = "testnet"
  rpc-url    = "http://127.0.0.1:26657"
  chain-id   = "test5"
  ```
- gnomcp built against this branch and connected to the MCP client.
- The faucet binary: `go build -o /tmp/agentfaucet ./cmd/agentfaucet`.
- A funding seed for the faucet (funded on the gnodev chain — test1's seed is pre-funded on gnodev).

## Checks

### 0. Generate the agent key
- `gno_key_generate` (profile = `testdev`) → records a `g1…` address. Re-running → `key_already_exists`.
- `gno_key_address` returns the same address.

### 1. Fallback — no faucet configured
- Profile has **no** `faucet-url` / `faucet-service-url`.
- `gno_faucet_fund` (profile = `testdev`) → reports the agent address + "send ugnot here, then retry"
  (backend: manual). **PASS** if the address matches `gno_key_address` and no URL is shown.

### 2. Tier 1 — existing faucet page
- Set `faucet-url = "https://faucet.example"` on the profile (no service-url). Restart gnomcp.
- `gno_faucet_fund` → surfaces the `faucet-url` + the address, then does a bounded balance poll.
- Fund the address out-of-band (e.g. `gnokey maketx send -send 1000000ugnot <addr>` from a funded
  account on gnodev). Re-run `gno_faucet_fund` → reports **funded**. **PASS** if the URL was shown and
  the status flips to funded after the manual send.

### 3. Tier 2 — automatic service
- Run the faucet:
  `/tmp/agentfaucet -rpc-url http://127.0.0.1:26657 -chain-id test5 -mnemonic "<funded seed>" -listen 127.0.0.1:8590 -grant 1000000000`
- Set `faucet-service-url = "http://127.0.0.1:8590"` on the profile (takes precedence over
  `faucet-url`). Restart gnomcp.
- On a **fresh** profile/address, `gno_faucet_fund` → returns a tx hash + **funded**, with no human
  step. Verify the balance rose on-chain (`gno_eval` a balance query, or gnokey). **PASS** if funded
  automatically.

### 4. Tier 2 — rate limit
- Immediately call `gno_faucet_fund` again for the same address → a clear transient "faucet busy /
  rate-limited" message (the 429 / per-address cooldown). **PASS** if it's a clean error, not a hang
  or crash.

### 5. Funded write — end to end
- With the agent funded, `gno_call` (or `gno_addpkg`) on `testdev` → succeeds, signed by the agent
  key; `insufficient_funds` no longer fires.
- On a fresh, unfunded testnet profile, the same write → `insufficient_funds` whose message points at
  `gno_faucet_fund`. **PASS** if the pointer is present.

### 6. Safety
- `agentfaucet` refuses to start with a non-testnet `-chain-id` (e.g. `-chain-id mainnet`) →
  `log.Fatal` before binding. **PASS** if it exits with the chain-id error.
- A `POST /fund` with a non-test `chain_id` → HTTP 403.

## Run log

(fill on a live run against gnodev)

- [ ] 0 — key generate / address
- [ ] 1 — fallback (manual)
- [ ] 2 — tier 1 (faucet page + balance poll)
- [ ] 3 — tier 2 (automatic dispense)
- [ ] 4 — tier 2 rate limit (429)
- [ ] 5 — funded write end-to-end
- [ ] 6 — safety (startup chain-id guard, 403)

**Bugs found / fixed:**

_(none yet — fill on first live run)_
