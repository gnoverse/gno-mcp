# Tools

17 v0.2 tools, in stable order. Every tool is registered through `internal/tools/tools.go` with explicit annotations (`readOnlyHint`, `destructiveHint`, etc.) the MCP client can use to gate confirmations.

## Auth

### `gno_auth_status`
- **Args:** `ensure_pending?` (bool, default `true`)
- **Returns:** `{state, network, session_address, balance_ugnot, threshold_ugnot, created_at, last_check, fund_url?, web_fund_url?, qr_ascii?, human_guidance?}`
- Returns the MCP session's current authorization state. On a fresh process, the default `ensure_pending=true` lazily generates the session keypair and returns the fund payload. Use `ensure_pending=false` to read state without creating a keypair. See [docs/auth.md](auth.md) for the model.

## Read-only

### `gno_network_info`
- **Args:** `domain?` (default `gno.land`)
- **Returns:** `{chain, domain, rpc, height}`
- Resolve a network domain to its current chain metadata.

### `gno_get`
- **Args:** `path` (required), `network?`
- **Returns:** `{kind, path, network, gnoweb_url, truncated, size, content?, summary?}`
- If `path` contains a function-call expression (`(`), evaluates read-only via `Eval`. Otherwise returns the realm's `Render()` output. Body is wrapped in `<untrusted_content>`. Truncates at 4 KB and returns a `summary` pointing at gnoweb when over-budget.

### `gno_eval`
- **Args:** `expr` (required), `network?`
- **Returns:** `{expr, network, size, content}`
- Thin wrapper over `Eval`. Result is wrapped in `<untrusted_content kind="eval">`.

### `gno_read`
- **Args:** `path` (required), `network?`, `symbol?`, `file?`, `lines?` (e.g. `"10-40"`)
- **Returns:** `{path, network, gnoweb_url, slice_requested, size, content?, summary?}`
- Without any slice param, returns a summary + gnoweb URL — **no source dump by default**. With one of `symbol`/`file`/`lines`, returns the full slice wrapped in `<untrusted_content kind="source">`.

### `gno_inspect`
- **Args:** `target` (required), `network?`
- **Returns:** `{kind: address|realm|network, target, data}`
- Dispatches on `target` shape: `g1…` → address info, anything containing `/` → realm inspection, otherwise → network info.

### `gno_address_info`
- **Args:** `address` (required), `network?`
- **Returns:** `{address, balance, sequence, account_number, recent_txs}`
- Recent transactions capped at 20.

## Onboarding

### `gno_keygen`
- **Args:** `name` (required)
- **Returns:** `{name, address, pubkey}` — **never a mnemonic**
- Tool description explicitly tells the LLM not to ask the user for a mnemonic.

### `gno_faucet_request`
- **Args:** `address` (required), `network` (required)
- **Returns:** `{network, address, status: "requested"}`
- Refuses mainnet (`gno.land`) with a `mainnet_write_blocked` structured error.

## Writes

### `gno_call`
- **Args:** `network?` (default `gno.land`), `path` (required), `func` (required), `signer?`, `args?[]`, `send?`, `confirm?`
- **Returns:** `{security: SecurityBlock, simulation: CallResult, broadcast: CallResult|null}`
- Always simulates first. **Signer resolution:** explicit `signer` arg → MCP session (if authenticated) → return `authentication_required` (or `authentication_expired`) with a fund payload. On `gno.land` mainnet, a missing `confirm: true` keeps `broadcast: null` and sets `security.confirmation_required: true` — no transaction is sent. Session-signed broadcasts show `signer: "mcp-session"` in the audit log.

### `gno_run`
- **Args:** `network` (required), `code` (required), `signer?`, `send?`, `confirm?`
- **Returns:** same shape as `gno_call`
- Same simulate→confirm→broadcast flow as `gno_call`, but with raw gno source instead of a path+func. The code is treated as a transaction payload; tool description warns the LLM that the code is not instructions.

## Sessions (v0.1 stubs)

### `gno_session_create`, `gno_session_revoke`, `gno_session_list`
- All three return a `not_implemented` structured error with hint:
  `blocked on upstream session-key PR; track gno-mcp release notes for availability`

## Configuration

### `gno_config_get`
- **Args:** none
- **Returns:** `{default_key, default_network, gas_buffer}`

### `gno_config_set`
- **Args:** `key` (required), `value` (required)
- **Returns:** the updated config map
- Whitelists `default_key` / `default_network` / `gas_buffer`. Unknown keys → `invalid_argument`. `gas_buffer` must parse as a non-negative integer. Persisted to `$XDG_CONFIG_HOME/gno-mcp/config.json` (or `$GNO_MCP_CONFIG`).

## Audit

### `gno_audit_tail`
- **Args:** `limit?` (default 50, capped at 1000)
- **Returns:** `{limit, count, entries: [{time, tool, network, signer, tx_hash, result, args}]}`
- Reads from `~/.gno-mcp/audit.jsonl`. `args` is post-redaction.
