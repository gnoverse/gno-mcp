# Security Posture

`gnomcp` is the bridge between an LLM and a live blockchain. Every design decision below exists to limit blast radius.

## 1. Chain-id allowlist — betanet/mainnet/staging forbidden

Config validation rejects any profile whose `chain-id` does not match `^(dev|test-?\d+)$`. Betanet (`gnoland1`), `staging`, and mainnet ids cannot enter the config at all. The check runs at startup and at `gnomcp profile add` time. A config that would admit mainnet fails loud; there is no override flag.

## 2. Write authorization: agent identity (local/testnet) or chain-bound session

Writes sign with one of two identities — never with the user's key.

**Agent identity — local and testnet.** The agent signs with its own key directly — no session required:

- **Local profiles** use the built-in **test1** account (the well-known *public* test mnemonic). Structurally confined to dev chains by the chain-id allowlist.
- **Testnet profiles** use a key generated and persisted by `gno_key_generate`. The key is stored in `~/.local/share/gnomcp/agent-keys` (mode `0600`); when `GNOMCP_SESSION_PASSPHRASE` is set it is encrypted at rest with scrypt+AES-256-GCM.

Both tiers are confined to dev/test by the chain-id allowlist (`^(dev|test-?\d+)$`); no path creates an agent key for mainnet.

**Session — opt-in or default on non-local/testnet profiles.** On profiles with a `master-address` the agent acts *as the user* via a chain-bound session:

1. **Profile has a `master-address` (g1...).** Without it the session write path is unavailable for that profile.
2. **User-authorized session.** `gno_session_propose` generates an ephemeral ed25519 keypair and emits a paste-ready `gnokey maketx session create` command. The user runs it; their `gnokey` signs the authorization. gnomcp never sees the user's key or mnemonic.

The session carries an explicit scope: `allow_paths`, `allow_run`, `spend_limit`, and `expires_in`, enforced on every call before broadcast.

The gate is structural (profile + tier + session); there is no opt-in dangerous-tools flag. Every write result names the acting identity so the human always knows which account signed.

## 3. The user's keys never leave gnokey

gnomcp never generates, reads, or stores the **user's** keys — acting as the user is always mediated by a session the user authorizes with their own `gnokey`. The ephemeral session keypair is generated per `gno_session_propose` call and stored only in `~/.local/share/gnomcp/sessions` (mode `0600`); with `GNOMCP_SESSION_PASSPHRASE` set, session files are encrypted at rest with scrypt+AES-256-GCM.

gnomcp does hold its **own** agent key: the dev/test **test1** account on local profiles, and a generated key on testnet profiles (see §2). Both are entirely separate from the user's keystore and valid only on dev/test chains.

## 4. Untrusted-content envelope

The gno chain is open — any realm's content is attacker-influenceable — so all
chain-returned bytes are untrusted. The control differs by delivery channel:

- **Inline text tools** (`gno_render`, `gno_eval`, `gno_inspect`, `gno_packages`, `gno_activity`, `gno_history`, `gno_list`) wrap their chain-derived output in an envelope:

  ```
  <untrusted_content kind="eval" source="gno.land/r/demo/foo">
  …
  </untrusted_content>
  ```

  An envelope tag (opening or closing) embedded in chain content is neutralized first, so content cannot escape or forge the envelope. The write tools envelope the realm-controlled portions of their success text the same way: `gno_call`'s `Result` (kind `call_result`) and `gno_run`'s `Output` (kind `run_output`).

- **The resource tool** (`gno_read`) returns content as an MCP `EmbeddedResource`, a distinct trust posture clients treat as resource data rather than inline instructions. It is not textually wrapped because that would corrupt the txtar archive (the closing tag would merge into the last file). This relies on the client honoring the resource boundary.

- **Error text** is mixed-trust: gnomcp's own framing can embed chain or network bytes (a realm's panic string in an ABCI log, a faucet's error body). All tool-error text is neutralized at the SDK boundary — embedded envelope tags are escaped — so error text cannot forge or close an envelope; it is not itself enveloped. The faucet error body is additionally labeled `[untrusted faucet response]` at the source.

- **Structured content** (`structuredContent` fields such as `gno_call`'s `result` and `gno_run`'s `output`) carries raw values — it is the machine-readable channel, and wrapping would corrupt consumers. Clients that surface structured fields to a model must apply their own marking.

Anything from any channel must be treated as data, never instructions.

## 5. Output budgeting

Every read and indexer tool applies a ~4 KB budget to chain-returned content. Over-budget responses are replaced by a summary with a hint to fetch a narrower slice or view at gnoweb — they are never silently chopped.

## 6. Audit log

- Path: `~/.local/share/gnomcp/audit.jsonl`, mode `0600`.
- One JSONL entry per tool invocation: `{time, tool, profile, args_summary, result, duration_ms, session_address}`.
- `args_summary` keeps only an allowlist of non-sensitive keys; every other arg is redacted. The write-tx tools build their own value-free summary (e.g. `nargs=N`, `code_len=N`) so addresses, amounts, code, and file bodies are never logged.
- Every write attempt is audited — including denials (insufficient_funds, scope_mismatch, validation). Reads opt-in via `--audit-reads`.

## 7. Structured errors

Errors are JSON-encoded payloads with `code`, `message`, and (where useful) extra fields. Notable codes:

| Code | Trigger |
|---|---|
| `chain_forbidden` | A mainnet/betanet/staging chain-id was rejected (config or gno_connect) |
| `authentication_required` | A session-signed write was attempted with no active session |
| `scope_mismatch` | The call's realm is not covered by any active session's `allow_paths` |
| `insufficient_funds` | The agent's testnet account is unfunded (run `gno_faucet_fund`) |
| `simulate_unsupported` | `simulate=true` against a client that can't dry-run |
| `agent_identity_unavailable` | Agent identity requested on a profile with no agent key (run `gno_key_generate` for testnet) |
