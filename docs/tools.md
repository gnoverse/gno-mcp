# Tools

15 tools across three categories. All tools accept a `profile` parameter that selects which profile (chain) to target; when only one profile is active it is omitted from the schema.

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

## Write tools — agent identity or session

gnomcp signs writes with one of two identities, chosen per call via the `identity` arg (`"agent"` | `"session"`):

- **Agent identity (default on local).** On `local`/dev profiles the agent signs with its own built-in **test1** account directly — no session, no `master-address` needed.
- **Session (default off-local).** On testnet profiles the agent acts *as the user* through a chain-bound session; authorize one with `gno_session_propose` first.

Every write result names the signer (`Signed by: agent test1 (g1…)` or `Signed by: session g1… on behalf of master g1…`) and the structured output carries `identity` + `signer_address`.

Registration: `gno_call`/`gno_run` appear when a profile is writable — local (agent key) **or** has a `master-address` (session). `gno_addpkg` and `gno_key_address` appear only when a local profile exists.

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

- **Args:** `profile` (required), `realm` (required), `func` (required), `args?[]`, `simulate?`, `identity?`
- **Returns:** broadcast (or `simulate`) result, prefixed with the signing identity.
- On local the agent key signs directly; on testnet an active session covering `realm` is required (its `allow_paths` is the gate).

### `gno_run`

- **Args:** `profile` (required), `code` (required), `simulate?`, `identity?`
- **Returns:** broadcast (or `simulate`) result, prefixed with the signing identity.
- On local the agent key signs directly; on testnet an active session with `allow_run=true` is required.

### `gno_addpkg`

- **Args:** `profile` (required), `deploy_path` (required), `files[]` (required — each `{name, body}`), `simulate?`
- **Returns:** deploy (or `simulate`) result, prefixed with the signing identity.
- Deploys a package/realm via `vm/MsgAddPackage`, signed by the agent key (local/dev only). A `gnomod.toml` is generated automatically if omitted.

### `gno_key_address`

- **Args:** `profile?`
- **Returns:** the agent's own account address for a local profile — read-only, no transaction. Use it to fund or inspect the agent account.
