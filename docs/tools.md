# Tools

13 tools across three categories. All tools accept a `profile` parameter that selects which profile (chain) to target; when only one profile is active it is omitted from the schema.

## Read-only (chain)

These four tools require no config — the built-in `local` and `testnet` profiles are available by default.

### `gno_render`

- **Args:** `path` (required), `subpath?`, `profile?`
- **Returns:** rendered markdown from the realm's `Render()` function.
- Output is truncated at ~4 KB. When truncated, a hint points at the canonical gnoweb URL or suggests fetching a narrower subpath.

### `gno_read`

- **Args:** `path` (required), `file?`, `profile?`
- **Returns:** source listing (all files) or a single file's source, wrapped in an `<untrusted_content>` envelope.
- Output is truncated at ~4 KB with a hint to narrow the request.

### `gno_eval`

- **Args:** `path` (required), `expr` (required), `profile?`
- **Returns:** the typed result of evaluating a Gno expression within a realm's context.

### `gno_inspect`

- **Args:** `path` (required), `profile?`
- **Returns:** godoc summary of a realm: package description, exported types, functions, and variables.

## Read-only (discovery)

### `gno_connect`

- **Args:** `url` (required), `name?`, `profile?`
- **Returns:** the exact `gnomcp profile add` command the user must run to register this chain.
- Reads gnoconnect meta-tags from the gnoweb page at `url` and derives the `--rpc` and `--chain-id` arguments. **Never mutates config.** Read-only; the user must run the printed command.

## Read-only (indexer)

These three tools are only registered when at least one profile has a `tx-indexer-url` set.

### `gno_list`

- **Args:** `namespace?`, `tag?`, `category?`, `profile?`
- **Returns:** filtered list of realms from the tx-indexer catalog.

### `gno_history`

- **Args:** `path` (required), `profile?`
- **Returns:** full deploy and transaction log for a realm.

### `gno_activity`

- **Args:** `path` (required), `since?`, `until?`, `profile?`
- **Returns:** MsgCall and MsgRun events for a realm, with optional RFC3339 time bounds.

## Write / session tools

All five are registered only when at least one profile has a `master-address` set. Writes additionally require an active chain-bound session (authorize one with `gno_session_propose` first).

### `gno_session_propose`

- **Args:** `profile` (required), `allow_paths?[]`, `allow_run?`, `spend_limit?`, `expires_in?`
- **Returns:** a paste-ready `gnokey maketx session create` command the user runs to authorize a chain-bound session.
- Generates an ephemeral ed25519 keypair locally. The user's `gnokey` signs the session; gnomcp never sees the user's key. At least one of `allow_paths` (non-empty) or `allow_run=true` must be requested.

### `gno_session_revoke`

- **Args:** `profile` (required), `address` (required)
- **Returns:** a paste-ready `gnokey maketx session revoke` command the user runs to revoke a managed session. Use `gno_auth_status` to list session addresses.

### `gno_auth_status`

- **Args:** `profile?`
- **Returns:** narrative view of all gnomcp-managed sessions for a profile — address, scope, expiry, and live on-chain confirmation status.

### `gno_call`

- **Args:** `profile` (required), `path` (required), `func` (required), `args?[]`, `send?`, `simulate?`
- **Returns:** simulation result and (when `simulate` is not set) broadcast result.
- Requires an active session that covers `path`. The session's `allow_paths` list is the authorization gate.

### `gno_run`

- **Args:** `profile` (required), `code` (required), `send?`, `simulate?`
- **Returns:** simulation result and (when `simulate` is not set) broadcast result.
- Requires an active session with `allow_run=true`.
