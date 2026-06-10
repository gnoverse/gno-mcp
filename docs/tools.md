# Tools

19 tools across read, discovery, admin, indexer, and write categories. All tools except `gno_connect` and `gno_profile_add` accept a `profile` parameter that selects which profile (chain) to target; when omitted, the server applies the default profile (discovered local node, else `testnet`).

Chain-returned bytes are untrusted: the inline-text read/indexer tools (including `gno_render`) wrap their output in an `<untrusted_content>` envelope, and `gno_read` delivers content as an MCP resource (see `docs/security.md` §4).

## Read-only (chain)

These tools require no config — the built-in `local` and `testnet` profiles are available by default.

### `gno_render`

- **Args:** `realm` (required), `path?` (subpath), `profile?`
- **Returns:** rendered markdown from the realm's `Render()` function, wrapped in an `<untrusted_content>` envelope (realm markdown is the highest-injection-risk content in the system).
- Output is truncated at ~4 KB. When truncated, a hint points at the canonical gnoweb URL or suggests fetching a narrower subpath.

### `gno_read`

- **Args:** `path` (required), `file?`, `profile?`
- **Returns:** the whole package as a txtar archive (all files) or a single file's source, as an MCP resource (EmbeddedResource trust posture; not textually wrapped).
- Output is truncated at ~4 KB with a hint to narrow the request.

### `gno_eval`

- **Args:** `path` (required), `expr` (required), `profile?`
- **Returns:** the typed result of evaluating a Gno expression within a package, wrapped in an `<untrusted_content>` envelope.

### `gno_inspect`

- **Args:** `path` (required), `profile?`
- **Returns:** godoc summary of a package (description, exported types, functions, variables), wrapped in an `<untrusted_content>` envelope.

### `gno_packages`

- **Args:** `path` (required — a prefix like `gno.land/r/demo/`, or `@namespace`), `limit?`, `profile?`
- **Returns:** newline-separated package paths deployed under the path (`vm/qpaths`, chain-native, no indexer required), wrapped in an `<untrusted_content>` envelope.

## Read-only (discovery)

### `gno_connect`

- **Args:** `gnoweb_url` (required), `name?` (suggested profile name, default derived from chain-id)
- **Returns:** both follow-up paths — `gno_profile_add` arguments for in-session use, and the exact `gnomcp profile add` command the user runs to persist the chain.
- Reads gnoconnect meta-tags from the gnoweb page at `gnoweb_url` and derives the `--rpc` and `--chain-id` arguments. **Never mutates config itself.**

## Admin

### `gno_profile_add`

- **Args:** `name` (required), then exactly one form: `rpc_url` + `chain_id` (explicit), or `gnoweb_url` (discovery). Optional: `tx_indexer_url`, `faucet_service_url`, `faucet_url`.
- **Returns:** confirmation plus the `gnomcp profile add` command to persist the profile.
- Adds a profile **in-memory only** — it disappears on restart and never touches `profiles.toml`. Init-time profiles cannot be overridden; re-adding a dynamically added name replaces it. Only `dev`/`testNN` chain-ids are accepted, and the node is dialed to confirm it reports the declared chain-id (gnoweb meta-tags are a hint, not truth; a non-loopback gnoweb advertising a loopback RPC is rejected). No `master-address` field: dynamic profiles support reads and agent-key writes only — sessions require a persisted profile. After a successful add the tool set is re-published (`tools/list_changed`), which can summon gated tools (faucet, indexer) mid-session.

## Read-only (indexer)

These three tools are only registered when at least one profile has a `tx-indexer-url` set.

### `gno_list`

- **Args:** `namespace?`, `tag?`, `category?`, `profile?`
- **Returns:** filtered list of realms from the tx-indexer catalog, wrapped in an `<untrusted_content>` envelope (entries echo realm-supplied paths and descriptions).

### `gno_history`

- **Args:** `realm` (required), `profile?`
- **Returns:** full deploy and transaction log for a realm, wrapped in an `<untrusted_content>` envelope.

### `gno_activity`

- **Args:** `realm` (required), `since?`, `until?`, `profile?`
- **Returns:** MsgCall and MsgRun events for a realm, with optional RFC3339 time bounds, wrapped in an `<untrusted_content>` envelope.

## Write tools — agent identity or session

gnomcp signs writes with one of two identities, chosen per call via the `identity` arg (`"agent"` | `"session"`):

- **Agent identity (default on local and testnet).** Local profiles sign with the built-in **test1** key directly — no session needed. Testnet profiles sign with a key generated and persisted by `gno_key_generate`; run that once and fund the address before making writes.
- **Session (opt-in).** The agent acts *as the user* through a chain-bound session; authorize one with `gno_session_propose` first. Pass `identity=session` to choose this path on any profile with a `master-address`.

Every write result names the signer (`Signed by: agent test1 (g1…)` or `Signed by: session g1… on behalf of master g1…`) and the structured output carries `identity` + `signer_address`.

Registration: every allowed chain (local dev or testnet) has an agent key path, so `gno_call`, `gno_run`, `gno_addpkg`, `gno_key_address`, and `gno_key_generate` always register. `gno_faucet_fund` appears when a testnet profile exists (including one added mid-session via `gno_profile_add`).

### `gno_session_propose`

- **Args:** `profile` (required), `allow_paths?[]`, `allow_run?`, `spend_limit?`, `expires_in?`
- **Returns:** a paste-ready `gnokey maketx session create` command the user runs to authorize a chain-bound session.
- Generates an ephemeral ed25519 keypair locally. The user's `gnokey` signs the session; gnomcp never sees the user's key. At least one of `allow_paths` (non-empty) or `allow_run=true` must be requested.

### `gno_session_revoke`

- **Args:** `profile` (required), `session_address` (required)
- **Returns:** a paste-ready `gnokey maketx session revoke` command the user runs to revoke a managed session. Use `gno_auth_status` to list session addresses.

### `gno_auth_status`

- **Args:** `profile?`
- **Returns:** narrative view of all gnomcp-managed sessions for a profile — address, scope, expiry, and live on-chain confirmation status.

### `gno_call`

- **Args:** `profile` (required), `realm` (required), `func` (required), `args?[]`, `simulate?`, `identity?`
- **Returns:** broadcast (or `simulate`) result, prefixed with the signing identity.
- Default identity: **agent** (test1 on local, generated key on testnet). Pass `identity=session` to act as the user instead.

### `gno_run`

- **Args:** `profile` (required), `code` (required), `simulate?`, `identity?`
- **Returns:** broadcast (or `simulate`) result, prefixed with the signing identity.
- Default identity: **agent**; pass `identity=session` to act as the user.

### `gno_addpkg`

- **Args:** `profile` (required), `deploy_path` (required), `files[]` (required — each `{name, body}`), `simulate?`
- **Returns:** deploy (or `simulate`) result, prefixed with the signing identity.
- Deploys a package/realm via `vm/MsgAddPackage`, signed by the agent key (local: test1, testnet: generated key). A `gnomod.toml` is generated automatically if omitted.

### `gno_key_address`

- **Args:** `profile?`
- **Returns:** the agent's own account address for a local or testnet profile — read-only, no transaction. Use it to fund or inspect the agent account. Local returns test1; testnet returns the address of the generated key.

### `gno_key_generate`

- **Args:** `profile` (required — testnet profiles only)
- **Returns:** the generated bech32 g1… address for the agent's testnet account. Persists the key. Refuses to overwrite an existing key.

### `gno_faucet_fund`

- **Args:** `profile?` (testnet only)
- **Returns:** the outcome of requesting a testnet grant for the agent's generated key — an automatic service grant (tx hash), a faucet link, or manual instructions, depending on the profile's faucet config. Use after `gno_key_generate` when the agent account is unfunded.
