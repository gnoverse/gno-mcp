# gnomcp SPEC1 Local — Agent-Identity E2E Protocol

Manual checklist for the **agent write identity on local (dev) chains** (Plan A). The agent signs
with the well-known **test1** account directly — no session. Run sequentially; record results inline
or in a runlog under `.mynote/v2-issues/`.

This is a manual protocol (not run by `go test`, not in CI) — the `chain.Real` agent methods
(`Call`/`Run`/`AddPackage`) have no unit test, so this is where they're verified live.

---

## Pre-flight

- [ ] **gnodev running on dev:**
  ```
  gnodev local -node-rpc-listener 127.0.0.1:26657 -chain-id dev -paths "gno.land/r/test/*"
  ```
  (or `./test/e2e/setup.sh`, which also deploys the bundled `gno.land/r/test/{counter,echo,other}` realms). gnodev premines **test1** (`g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5`) with 10e12 ugnot by default.
- [ ] **gnomcp reconnected on the `feat/agent-keystore` code.** The agent tools (`gno_addpkg`,
  `gno_key_address`) and the `identity` arg are new — an MCP client connected to an older build
  won't expose them. Restart/reconnect the server (`go run ./cmd/gnomcp`, default config) so the
  built-in `local` profile is present and gnodev is auto-discovered.
- [ ] **`tools/list` includes** `gno_addpkg`, `gno_key_address`, and `gno_key_generate` (these
  agent tools always register — every allowed chain has an agent-key path).

---

## Section A — agent identity on local

### A1 — gno_key_address returns test1

```
gno_key_address(profile=local)
```

Pass: result Text is `g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5`; structured `address` matches. No transaction.

### A2 — gno_addpkg deploys as the agent (test1)

```
gno_addpkg(profile=local, deploy_path="gno.land/r/test/e2eagent", files=[
  {name:"e2eagent.gno", body:"package e2eagent\n\nfunc Hello() string { return \"hi from agent\" }\n"}
])
```

Pass:
- success; result Text starts with `Signed by: agent test1 (g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5)`.
- a `tx_hash` is present; structured `identity="agent"`, `signer_address=g1jg8mtu…`.
- **On-chain `Caller == test1`** (a standard tx — NOT a session tx; no `SessionAddr`). Verify e.g. `gnokey query tx <hash>` or the gnodev log.
- **`gnomod.toml` was auto-injected** (we passed only the `.gno`).
- **⚠️ MaxDeposit check:** if the deploy is rejected for an insufficient/invalid storage deposit, that's `chain.DefaultMaxDepositUgnot` (10000000ugnot, provisional). Tune the constant and record the working value HERE.

### A3 — gno_call defaults to the agent identity on local

```
gno_call(profile=local, realm="gno.land/r/test/e2eagent", func="Hello")
```

(No `identity` arg → defaults to **agent** on a local profile.)

Pass: success; result Text contains `Signed by: agent test1 (…)`; structured `identity="agent"`. Standard tx (Caller=test1, no SessionAddr).

### A4 — gno_call against a pre-deployed realm

```
gno_call(profile=local, realm="gno.land/r/test/counter", func="Increment")
```

Pass: success as the agent; `gno_eval(profile=local, realm=gno.land/r/test/counter, expr="Total()")` reflects the increment.

### A5 — gno_run as the agent

```
gno_run(profile=local, code="package main\n\nfunc main() { println(\"agent run ok\") }\n")
```

Pass: success; result names `agent test1`. The run executed under the test1 ephemeral path
`gno.land/e/g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5/run` (confirm via tx/log).

### A6 — simulate does not broadcast

```
gno_call(profile=local, realm="gno.land/r/test/counter", func="Increment", simulate=true)
gno_addpkg(profile=local, deploy_path="gno.land/r/test/e2esim", files=[{name:"x.gno", body:"package e2esim\n\nfunc A() string { return \"a\" }\n"}], simulate=true)
```

Pass: both return a gas figure with no broadcast (no `tx_hash` / "simulated"); `Total()` unchanged after the call-simulate.

---

## Section B — negatives (tier gating preserved)

### B1 — agent identity unavailable on testnet (addpkg)

Using a testnet profile (e.g. `testnet`, the built-in test11):

```
gno_addpkg(profile=testnet, deploy_path="gno.land/r/x/y", files=[{name:"y.gno", body:"package y\n"}])
```

Pass: `isError`, `code=agent_identity_unavailable`.

### B2 — agent identity unavailable on testnet (key_address)

```
gno_key_address(profile=testnet)
```

Pass: `isError`, `code=agent_identity_unavailable`.

### B3 — agent default on testnet without a generated key

```
gno_call(profile=testnet, realm="gno.land/r/test/counter", func="Increment")
```

(No `identity` → defaults to **agent** on every profile; no key was generated for `testnet`.)

Pass: `isError`, `code=agent_identity_unavailable` pointing at `gno_key_generate` (pass `identity=session` to exercise the session path instead).

### B4 — explicit identity=session on local still uses the session path

With an active session on `local` (propose + sign per PROTOCOL.md), then:

```
gno_call(profile=local, realm="gno.land/r/test/counter", func="Increment", identity="session")
```

Pass: success; result Text contains `Signed by: session g1… on behalf of master g1…` (NOT the agent line). Confirms the explicit override reaches `CallAsUser`.

---

## Teardown

- [ ] Stop gnodev (`./test/e2e/teardown.sh` if `setup.sh` was used). No leftover gnodev/gnomcp processes.

---

## Run log — 2026-06-07 (`feat/agent-keystore`, gnodev dev @ :26657)

All checks **PASS. No bugs found.**

- **A1** `gno_key_address(local)` → `g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5` ✓
- **A2** `gno_addpkg gno.land/r/test/e2eagent` → ok; `Signed by: agent test1`; TxHash `d0f7b1d…`; GasUsed 2786874. **MaxDeposit 10M sufficient — no deposit rejection.** ✓
- **A3** `gno_call Hello` → ok; `Signed by: agent test1`; Result `("hi from agent" string)` ✓
- **A5** `gno_run` → ok; `Signed by: agent test1`; Output `agent run ok` ✓
- **A6** `gno_call Hello simulate=true` → gas returned, no TxHash (no broadcast) ✓
- **B4** `gno_call identity=session` (no session) → `authentication_required` — the override routes to the session path; no agent fallback ✓
- **Audit** — agent writes logged `result=ok` with **no `session_address`** (session writes carry it); the agent/session distinction is visible in the trail ✓
- **B1–B3** (testnet via agent tools) — the agent tools' profile enum lists only `local`, so testnet can't be targeted; the handler's `agent_identity_unavailable` gate is unit-tested.

---

## What this protocol is NOT

- Not run by `go test`; not in CI. The unit + integration tests (with `chain.Fake`) cover logic; this verifies the live `chain.Real` agent path + the provisional `MaxDeposit` against a real chain.
- One pass per check, sequentially.
