# Tools

23 tools across read, discovery, admin, indexer, and write categories. All tools except `gno_connect` and `gno_profile_add` accept a `profile` parameter that selects which profile (chain) to target; when omitted, the server applies the default profile (discovered local node, else `testnet`).

Chain-returned bytes are untrusted: the inline-text read/indexer tools (including `gno_render`) wrap their output in an `<untrusted_content>` envelope, and `gno_read` delivers content as an MCP resource (see `docs/security.md` §4).

## Read-only (chain)

These tools require no config — the built-in `local` and `testnet` profiles are available by default.

### `gno_render`

- **Args:** `realm` (required), `path?` (subpath), `profile?`
- **Returns:** rendered markdown from the realm's `Render()` function, wrapped in an `<untrusted_content>` envelope (realm markdown is the highest-injection-risk content in the system).
- Output is truncated at ~4 KB. When truncated, a hint points at the canonical gnoweb URL or suggests fetching a narrower subpath.

### `gno_read`

- **Args:** `path` (required), `file?`, `symbols?[]`, `full?`, `profile?`
- **Returns:** package source at three depths, as an MCP resource (EmbeddedResource trust posture; not textually wrapped):
  - **Default (no `symbols`/`full`):** a structural **outline** — per-file txtar with exported signatures + docs, unexported signatures, imports, and byte counts; bodies elided. Server-rendered from the AST, with embedded envelope tags neutralized.
  - **`symbols` (e.g. `["Transfer", "Counter.Inc"]`):** the verbatim source of those declarations with docs, plus a best-effort `// deps:` header (same-package references and imports used; unresolved method calls are counted and flagged inline). Misses are reported with the available names.
  - **`full=true`:** raw source — one whole file (with `file`) or the whole package as txtar.
- Budget: whole-package raw (`full` without `file`) truncates at ~4 KB; everything else — the outline (bounded by construction: bodies elided), `symbols`, `full` + `file` — gets ~64 KB, a higher ceiling sized for real reads, not a bypass.
- The outline and dep headers are **navigation, not evidence**: names and docs are realm-authored claims. Audit-grade review reads whole files.

### `gno_eval`

- **Args:** `path` (required), `expr` (required), `profile?`
- **Returns:** the typed result of evaluating a Gno expression within a package, wrapped in an `<untrusted_content>` envelope.

### `gno_packages`

- **Args:** `path` (required — a prefix like `gno.land/r/demo/`, or `@namespace`), `limit?`, `profile?`
- **Returns:** newline-separated package paths deployed under the path (`vm/qpaths`, chain-native, no indexer required), wrapped in an `<untrusted_content>` envelope.

### `gno_account`

- **Args:** `address` (required — bech32 `g1…`), `profile?`
- **Returns:** balance, sequence (nonce), and account number for any address (`auth/accounts`), wrapped in an `<untrusted_content>` envelope plus structured fields. An address with no on-chain record reports `exists:false` — a normal answer (never funded or used), not an error.

### `gno_status`

- **Args:** `profile?`
- **Returns:** the profile's declared chain-id and RPC URL plus the node's live chain-id, latest block height, and block time (RPC `/status`). Flags a mismatch when the node reports a different chain-id than the profile declares. If the node is unreachable, config info is still returned with a `height_error` instead of a tool failure.

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
- **Session (opt-in, WIP — use with caution).** The agent acts *as the user* through a chain-bound session; authorize one with `gno_session_propose` first. Pass `identity=session` to choose this path on any profile with a `master-address`. The session path is young and will be reworked — keep `allow_paths` tight and `spend_limit` low.

Every write result names the signer (`Signed by: agent test1 (g1…)` or `Signed by: session g1… on behalf of master g1…`) and the structured output carries `identity` + `signer_address`.

Registration: every allowed chain (local dev or testnet) has an agent key path, so `gno_call`, `gno_run`, `gno_addpkg`, `gno_key_send`, `gno_key_address`, `gno_key_list`, `gno_key_generate`, and `gno_key_delete` always register. `gno_faucet_fund` appears when a testnet profile exists (including one added mid-session via `gno_profile_add`).

A profile can hold several named agent keys (up to `GNOMCP_AGENT_MAX_KEYS`, default 5), so the agent can fund secondary accounts and exercise realms involving multiple addresses. The write and key tools take an optional `key` arg (default `"default"`) selecting which key to act with; `gno_key_list` enumerates them. `key` applies to `identity=agent` only and is rejected with `identity=session`.

### `gno_session_propose`

- **Args:** `profile` (required), `allow_paths?[]`, `allow_run?`, `spend_limit?`, `expires_in?`, `master_address?`
- **Returns:** a paste-ready `gnokey maketx session create` command the user runs to authorize a chain-bound session.
- Generates an ephemeral ed25519 keypair locally. The user's `gnokey` signs the session; gnomcp never sees the user's key. At least one of `allow_paths` (non-empty) or `allow_run=true` must be requested.
- On a writable profile with no configured `master-address`, pass `master_address` — the user's PUBLIC g1… address — so the session can act as them, with no `profiles.toml` edit. It is public data, never a private key or seed phrase; seed-phrase-shaped input is rejected without being echoed.

### `gno_session_revoke`

- **Args:** `profile` (required), `session_address` (required)
- **Returns:** a paste-ready `gnokey maketx session revoke` command the user runs to revoke a managed session. Use `gno_auth_status` to list session addresses.

### `gno_auth_status`

- **Args:** `profile?`
- **Returns:** narrative view of all gnomcp-managed sessions for a profile — address, scope, expiry, and live on-chain confirmation status.

### `gno_call`

- **Args:** `profile` (required), `realm` (required), `func` (required), `args?[]`, `send?`, `simulate?`, `identity?`, `key?`
- **Returns:** broadcast (or `simulate`) result, prefixed with the signing identity.
- Default identity: **agent** (test1 on local, generated key on testnet). Pass `identity=session` to act as the user instead.
- `send` attaches coins to the call (e.g. `"1000000ugnot"`) for payable functions that read `std.OriginSend()`; under a session, the chain enforces the session spend limit against it.

### `gno_run`

- **Args:** `profile` (required), `code` (required), `simulate?`, `identity?`, `key?`
- **Returns:** broadcast (or `simulate`) result, prefixed with the signing identity.
- Default identity: **agent**; pass `identity=session` to act as the user.

### `gno_addpkg`

- **Args:** `profile` (required), `deploy_path` (required), `files[]` (required — each `{name, body}`), `simulate?`, `key?`
- **Returns:** deploy (or `simulate`) result, prefixed with the signing identity.
- Deploys a package/realm via `vm/MsgAddPackage`, signed by the agent key (local: test1, testnet: generated key). A `gnomod.toml` is generated automatically if omitted.

### `gno_key_address`

- **Args:** `profile?`, `key?`
- **Returns:** the agent's own account address for a local or testnet profile — read-only, no transaction. Use it to fund or inspect the agent account. Local returns test1; testnet returns the address of the named key (default `"default"`). Does NOT enumerate keys — use `gno_key_list`.

### `gno_key_list`

- **Args:** `profile?`
- **Returns:** the profile's agent keys as an array of `{name, address}` (read-only). Use it to rediscover keys generated in earlier sessions before selecting one with the `key` arg. Local profiles report a single `"default"` (test1); an empty list means none generated yet. A key whose file is unreadable is flagged rather than hidden.

### `gno_key_generate`

- **Args:** `profile` (required — testnet profiles only), `key?`
- **Returns:** the generated bech32 g1… address for a testnet agent key. Persists the key under `key` (default `"default"`). Purely additive: refuses to overwrite an existing name, and refuses past the per-profile cap (`GNOMCP_AGENT_MAX_KEYS`). To replace a key, `gno_key_delete` it first, then generate again.

### `gno_key_delete`

- **Args:** `profile` (required — testnet only), `key` (required)
- **Returns:** confirmation naming the deleted address. **Irreversible** — any ugnot the address held becomes unreachable. The `key` arg is required (no default), so a key can't be deleted by omission. Use to free a slot at the cap, or to replace a key (delete, then `gno_key_generate`).

### `gno_key_send`

- **Args:** `profile` (required — testnet only), `to` (required — destination key name), `amount` (required — ugnot, whole number), `from?` (source key name, default `"default"`)
- **Returns:** the from/to addresses, the amount, and the tx hash. Transfers ugnot **between two of the agent's own keys** in the same profile (`bank.MsgSend`). `to`/`from` are key NAMES, not addresses — there is no path to an arbitrary external address. Use to seed a secondary key from the main funded key. Does NOT call realm functions (use `gno_call` with `send=`).

### `gno_faucet_fund`

- **Args:** `profile?` (testnet only), `key?`
- **Returns:** the outcome of requesting a testnet grant for the named agent key — an automatic service grant (tx hash), a faucet link, or manual instructions, depending on the profile's faucet config. Use after `gno_key_generate` when the agent account is unfunded.
