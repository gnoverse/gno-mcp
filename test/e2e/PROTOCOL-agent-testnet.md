# gnomcp SPEC1 Testnet — Agent-Identity E2E Protocol

Manual checklist for the **agent write identity on testnet chains** (SPEC1, testnet tier). The
agent signs with a freshly generated per-profile key — not the test1 account — after being funded
out-of-band. Run sequentially; record results inline or in a runlog under `.mynote/v2-issues/`.

This is a manual protocol (not run by `go test`, not in CI) — the testnet key-generation,
persistence, balance pre-check, and funded write paths have no unit test against a live chain, so
this is where they're verified.

---

## Pre-flight

- [ ] **gnodev running with a testnet-tier chain-id.** The chain-id must match the allowlist regex
  `test-?\d+` (e.g. `test9999`). Example launch:
  ```
  gnodev start --chain-id test9999 --node-rpc-listener 127.0.0.1:26657
  ```
  (Verify the exact flag name with `gnodev --help`; the flag shown above may differ across gnodev
  versions.) gnodev premines **test1**
  (`g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5`) with 10 000 000 000 000 ugnot by default. The
  `test1` account is used only for funding steps — the agent operates under its own separate key.

- [ ] **A `local-tnet` profile is configured** in `./profiles.toml` (project-local) or
  `~/.config/gnomcp/profiles.toml` (global). The profile must carry `chain-type = "testnet"`,
  the matching chain-id, and a `master-address` so write tools are enabled:
  ```toml
  [local-tnet]
  chain-type     = "testnet"
  rpc-url        = "http://127.0.0.1:26657"
  chain-id       = "test9999"
  master-address = "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"
  ```
  Alternatively, use `gnomcp profile add`:
  ```
  gnomcp profile add local-tnet --rpc http://127.0.0.1:26657 --chain-id test9999 --master g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5
  ```
  Confirm: `gnomcp profile list` shows `local-tnet  test9999  http://127.0.0.1:26657  [writable]`.

- [ ] **No pre-existing agent key for `local-tnet`.** Check that
  `$GNOMCP_AGENT_KEYS_PATH/local-tnet.key` (default:
  `~/.local/share/gnomcp/agent-keys/local-tnet.key`) does not exist, so section A starts fresh.
  Remove it if a leftover from a previous run is present.

- [ ] **gnomcp running on the `feat/agent-keystore-testnet` code:**
  ```
  go run ./cmd/gnomcp
  ```
  Optional env vars (set before launching):
  - `GNOMCP_AGENT_KEYS_PATH` — override the agent-keys directory (default:
    `~/.local/share/gnomcp/agent-keys`).
  - `GNOMCP_SESSION_PASSPHRASE` — when set, the on-disk key file is encrypted at rest (AES-GCM);
    leave unset to store plaintext (acceptable for a dev/test hot key).

- [ ] **`tools/list` includes** `gno_addpkg`, `gno_key_address`, and `gno_key_generate`
  (confirms `AnyProfileAgentCapable()` sees at least one local or testnet profile).

---

## Section A — key generation and identity

### A1 — gno_key_generate creates a fresh agent key

```
gno_key_generate(profile=local-tnet)
```

Pass:
- [ ] success; result Text is a `g1…` bech32 address.
- [ ] the address is **NOT** `g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5` (test1) — it is a newly
  derived address unique to this profile.
- [ ] structured `address` matches the Text value.
- [ ] the key file `$GNOMCP_AGENT_KEYS_PATH/local-tnet.key` now exists on disk (mode 0600).
- [ ] **Record the generated address here for use in subsequent steps:**
  `AGENT_ADDR = ___________________________`

### A2 — gno_key_address echoes the same address

```
gno_key_address(profile=local-tnet)
```

Pass:
- [ ] success; result Text is identical to `AGENT_ADDR` from A1.
- [ ] structured `address` matches.
- [ ] no transaction was submitted (read-only).

### A3 — duplicate key generation refused

```
gno_key_generate(profile=local-tnet)
```

(A key now exists for this profile — call should be rejected.)

Pass:
- [ ] `isError`, `code=key_already_exists`, message names `local-tnet` and suggests
  `gno_key_address`.

---

## Section B — unfunded write blocked

### B1 — gno_call blocked on unfunded agent (call)

The agent address `AGENT_ADDR` has zero balance at this point.

```
gno_call(profile=local-tnet, realm="gno.land/r/test/counter", func="Increment")
```

Pass:
- [ ] `isError`, `code=insufficient_funds`.
- [ ] message names `AGENT_ADDR` as the unfunded account.
- [ ] structured `Extra` contains `"address": "<AGENT_ADDR>"` and `"profile": "local-tnet"`.
- [ ] **no transaction was broadcast** (no `tx_hash` in the error; gnodev log shows no incoming tx
  from `AGENT_ADDR`).

### B2 — gno_addpkg blocked on unfunded agent (addpkg)

```
gno_addpkg(profile=local-tnet, deploy_path="gno.land/r/test/e2atnet",
  files=[{name:"e2atnet.gno", body:"package e2atnet\n\nfunc Hello() string { return \"hi testnet\" }\n"}])
```

Pass:
- [ ] `isError`, `code=insufficient_funds`.
- [ ] message names `AGENT_ADDR`.
- [ ] structured `Extra` contains `"address": "<AGENT_ADDR>"`.

---

## Section C — fund the agent and verify writes succeed

### C1 — fund the agent address

Send ugnot from test1 to `AGENT_ADDR`. Using `gnokey`:
```
gnokey maketx send \
  --gas-fee 1000000ugnot \
  --gas-wanted 100000 \
  --send 10000000ugnot \
  --to <AGENT_ADDR> \
  --remote http://127.0.0.1:26657 \
  --chainid test9999 \
  test1
```
(Adjust `--gas-fee`, `--gas-wanted`, and `--send` amounts to match your gnodev's state. The
`test1` key must be in your local gnokey ring — it is auto-imported by gnodev on most setups.
Verify the exact flag names with `gnokey maketx send --help`.)

Alternatively, use gnodev's built-in premine UI or fund via the gnoweb faucet if available.

Pass:
- [ ] the send transaction is confirmed (non-zero block height returned by gnokey or visible in the
  gnodev log).
- [ ] `AGENT_ADDR` now has a non-zero balance (verify with
  `gnokey query bank/balances/<AGENT_ADDR> --remote http://127.0.0.1:26657` or gnodev's balance
  API — exact query path may vary; verify with `gnokey query --help`).

### C2 — gno_call succeeds as the agent

```
gno_call(profile=local-tnet, realm="gno.land/r/test/counter", func="Increment")
```

(No `identity` arg → defaults to `agent` on a testnet profile.)

Pass:
- [ ] success; result Text starts with `Signed by: agent <AGENT_ADDR>` (NOT test1).
- [ ] `TxHash` is present; `Height > 0`.
- [ ] structured `identity="agent"`, `signer_address=<AGENT_ADDR>`.
- [ ] **on-chain Caller == `AGENT_ADDR`**: verify via gnodev log or
  `gnokey query tx <TxHash> --remote http://127.0.0.1:26657`.

### C3 — gno_addpkg deploys as the agent

```
gno_addpkg(profile=local-tnet, deploy_path="gno.land/r/test/e2atnet",
  files=[{name:"e2atnet.gno", body:"package e2atnet\n\nfunc Hello() string { return \"hi testnet\" }\n"}])
```

Pass:
- [ ] success; result Text starts with `Signed by: agent <AGENT_ADDR>`.
- [ ] `TxHash` present; `Height > 0`.
- [ ] structured `identity="agent"`, `signer_address=<AGENT_ADDR>`.
- [ ] **`gnomod.toml` was auto-injected** (only `.gno` file was supplied).
- [ ] **on-chain Caller == `AGENT_ADDR`** (not test1).
- [ ] **⚠️ MaxDeposit check:** if the deploy is rejected for an insufficient storage deposit, that
  is `chain.DefaultMaxDepositUgnot` (10 000 000 ugnot, provisional). Tune the constant and record
  the working value HERE: `MaxDeposit = ___________`

### C4 — gno_call against the deployed realm

```
gno_call(profile=local-tnet, realm="gno.land/r/test/e2atnet", func="Hello")
```

Pass:
- [ ] success; `Signed by: agent <AGENT_ADDR>`; result contains `"hi testnet"`.

### C5 — simulate does not broadcast

```
gno_call(profile=local-tnet, realm="gno.land/r/test/counter", func="Increment", simulate=true)
gno_addpkg(profile=local-tnet, deploy_path="gno.land/r/test/e2atnet2",
  files=[{name:"e2atnet2.gno", body:"package e2atnet2\n\nfunc A() string { return \"a\" }\n"}], simulate=true)
```

Pass:
- [ ] both return a gas figure; neither has a `tx_hash` (no broadcast, `simulated=true`).
- [ ] no balance change on `AGENT_ADDR` after the simulate calls.
- [ ] the balance pre-check is **skipped** for simulate: even with a zeroed-out balance the
  simulate would not raise `insufficient_funds` (confirm by topping up to exactly 0 ugnot is not
  required — the code path skips the check when `simulate=true`).

---

## Section D — persistence and at-rest encryption

### D1 — restart gnomcp; key survives

Stop gnomcp (Ctrl-C or SIGTERM) and restart it:
```
go run ./cmd/gnomcp
```
(Same env vars as the initial launch.)

```
gno_key_address(profile=local-tnet)
```

Pass:
- [ ] returns the **same** `AGENT_ADDR` as A1 (key was loaded from the persisted file).
- [ ] `gno_call` with `profile=local-tnet` still signs as `AGENT_ADDR` (funded write succeeds).

### D2 — at-rest encryption (when passphrase is set)

Perform this sub-check only if `GNOMCP_SESSION_PASSPHRASE` was set during key generation.

- [ ] Inspect the raw bytes of `$GNOMCP_AGENT_KEYS_PATH/local-tnet.key`:
  ```
  file $GNOMCP_AGENT_KEYS_PATH/local-tnet.key
  ```
  Pass: reported as binary / data (not UTF-8 text); it is **not** a readable 24-word mnemonic
  phrase.

If `GNOMCP_SESSION_PASSPHRASE` was **not** set during key generation, the file is stored as
plaintext (acceptable for a dev/test hot key) — skip D2 and note it in the run log.

---

## Section E — negatives (tier gating preserved)

### E1 — gno_key_generate rejected on a local profile

```
gno_key_generate(profile=local)
```

(The built-in `local` profile has `chain-type = "local"` — key generation is not permitted.)

Pass:
- [ ] `isError`, `code=key_generation_unsupported`.
- [ ] message says the profile is not a testnet profile and names `local`.

### E2 — session default still applies when identity=session

With **no** active session on `local-tnet`:

```
gno_call(profile=local-tnet, realm="gno.land/r/test/counter", func="Increment", identity=session)
```

Pass:
- [ ] `isError`, `code=authentication_required` (the session path is unchanged — no agent fallback
  when `identity=session` is explicit).

---

## Teardown

- [ ] Stop gnodev and gnomcp. No leftover processes.
- [ ] Optionally remove the generated key file if you want a clean state for future runs:
  ```
  rm $GNOMCP_AGENT_KEYS_PATH/local-tnet.key
  ```
  (Or keep it — the next run of A1 will return `key_already_exists`, which is expected.)

---

## Run log — 2026-06-08

Date: 2026-06-08  Branch: `feat/agent-keystore-testnet`  gnodev chain-id: test9999

Driven via the MCP stdio protocol against a freshly built `cmd/gnomcp` (the editor's connected MCP
was a stale build on the dev chain). Agent address generated:
`g1ewz0sf8nanm7fry8g86q0gjel7k2egzjxelqya`. test1 = `g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5`.

- **A1** `gno_key_generate(local-tnet)` → `g1ewz0sf8nanm7fry8g86q0gjel7k2egzjxelqya` (≠ test1) ☑ PASS
- **A2** `gno_key_address(local-tnet)` → same address ☑ PASS
- **A3** duplicate generate → `key_already_exists` ☑ PASS
- **B1** unfunded `gno_call` → `insufficient_funds` (names agent addr in `extra`) ☑ PASS
- **B2** unfunded `gno_addpkg` → `insufficient_funds` ☑ PASS
- **C1** fund agent from test1 (`gnokey maketx send` 10000000000ugnot) ☑ PASS
- **C2/C3** funded `gno_addpkg` (deploy `r/test/e2atnet`) + `gno_call` → signed by agent `g1ewz0…`, broadcast, Height > 0 ☑ PASS (used an agent-deployed realm rather than a pre-deployed counter; MaxDeposit 10000000ugnot sufficient)
- **C4** `gno_call e2atnet.Hello` → `"hi testnet"` ☑ PASS
- **C5** simulate `gno_addpkg` → `simulated=true`, no `tx_hash` ☑ PASS
- **D1** fresh gnomcp process → `gno_key_address` returns the same `g1ewz0…`; funded writes still sign as it ☑ PASS
- **D2** with `GNOMCP_SESSION_PASSPHRASE` the key file is `data` (binary ciphertext); without it `ASCII text` (plaintext, as documented) ☑ PASS
- **E1** `gno_key_generate(local)` → `key_generation_unsupported` ☑ PASS
- **E2** `gno_call identity=session` (no session) → `authentication_required` ☑ PASS

Overall: ☑ ALL PASS (1 bug found and fixed — see notes).

Notes:

- **Bug found + fixed during this run.** Every agent write rendered `Signed by: agent test1 (<addr>)`
  even on testnet, where the signer is a generated key (the address shown was the generated key's,
  not test1's) — a misleading identity line. Root cause: `internal/tools/write/identity.go`
  hardcoded the `test1` name. Fix: `signedByLine` now takes the profile chain-type and names `test1`
  only on local; testnet renders `Signed by: agent (<addr>)`. Regression test added in
  `identity_test.go`. Re-verified live: testnet → `agent (g1ewz0…)`, local → `agent test1 (g1jg8…)`.
- The unit tests exercise only the local/test1 agent path, so this label bug was invisible to them —
  a textbook case for keeping this live protocol.

---

## What this protocol is NOT

- Not run by `go test`; not in CI. The unit tests (with `chain.Fake`) cover the key-generation and
  balance-check logic; this protocol verifies the live `chain.Real` path — actual key derivation,
  actual balance query, actual broadcast — against a running gnodev.
- One pass per check, sequentially.
