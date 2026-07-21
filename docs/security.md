# Security Posture

`gnomcp` is the bridge between an LLM and a live blockchain. Every design decision below exists to limit blast radius.

## 1. Chain-id capability gate — writes confined to dev/testnet

A chain's `chain-id` determines what gnomcp may do with it:

- **Write-capable** — `chain-id` matching `^(dev|test-?\d+)$` (local dev, numbered testnets). These get an agent key path and appear in the write tools' profile enums.
- **Read-only** — any other format-safe `chain-id` (betanet `gnoland1`, `staging`, mainnet). Admitted so deployed source can be audited, but excluded from every write tool: no agent key, no faucet, no session, and `master-address` is refused at config time. Reads only.

The classification is enforced at startup config validation, at `gnomcp profile add`, and at `gno_profile_add` (which additionally dials the node and refuses the add unless it reports the declared chain-id). No override turns a read-only chain writable — the write path for mainnet/betanet does not exist in code. A `chain-id` carrying shell metacharacters or whitespace is rejected outright, since it is interpolated into the commands the user pastes into a terminal.

## 2. Write authorization: agent identity (local/testnet) or chain-bound session

Writes sign with one of two identities — never with the user's key.

**Agent identity — local and testnet.** The agent signs with its own key directly — no session required:

- **Local profiles** use the built-in **test1** account (the well-known *public* test mnemonic). Structurally confined to dev chains by the chain-id capability gate.
- **Testnet profiles** use keys generated and persisted by `gno_key_generate`. Each profile may hold up to `GNOMCP_AGENT_MAX_KEYS` (default 5) named keys, stored one file per key at `~/.local/share/gnomcp/agent-keys/<profile>/<name>.key` (mode `0600`); when `GNOMCP_SESSION_PASSPHRASE` is set they are encrypted at rest with scrypt+AES-256-GCM. `gno_key_send` moves ugnot only between a profile's own keys (the destination is a key name, never an arbitrary address).

Both tiers are confined to dev/test by the chain-id capability gate (`^(dev|test-?\d+)$`); no path creates an agent key for a read-only chain (mainnet/betanet).

**Session — opt-in (`identity=session`), WIP.** The session path is functional end-to-end but young and will be reworked — use with caution: keep `allow_paths` tight, `spend_limit` low, and `expires_in` short. On any writable chain the agent can act *as the user* via a chain-bound session:

1. **Master account.** The session binds to the user's account. It comes from the profile's `master-address`, or — for a writable profile without one — from a `master_address` the user supplies at `gno_session_propose` time. That value is a **public** bech32 address (no key material); `gno_session_propose` validates it and **rejects seed-phrase-shaped input without echoing it**, so a mnemonic cannot be pasted by mistake. The master is stored on the session record, not persisted to `profiles.toml`. A wrong address cannot move funds — the user's gnokey is still the only authorization (step 2).
2. **User-authorized session.** `gno_session_propose` generates an ephemeral ed25519 keypair and emits a paste-ready `gnokey maketx session create` command. The user runs it; their `gnokey` signs the authorization. gnomcp never sees the user's key or mnemonic.

The session carries an explicit scope: `allow_paths`, `allow_run`, `spend_limit`, and `expires_in`, enforced on every call before broadcast. The chain counts each write's full offered gas fee against `spend_limit`, so gnomcp is fee-aware at both ends: `gno_session_propose` queries the live gas price and rejects a limit no write could fit under (defaulting an omitted limit to ~10 writes' worth), and every session-signed write pre-checks its outflow (gas fee + send) against the remaining limit client-side, mirroring the chain's ante — an over-limit write fails with the numbers and recovery path instead of the chain's terse `session not allowed error`. The chain remains authoritative.

The gate is structural (profile + tier + session); there is no opt-in dangerous-tools flag. Every write result names the acting identity so the human always knows which account signed.

## 3. The user's keys never leave gnokey

gnomcp never generates, reads, or stores the **user's** keys — acting as the user is always mediated by a session the user authorizes with their own `gnokey`. The ephemeral session keypair is generated per `gno_session_propose` call and stored only in `~/.local/share/gnomcp/sessions` (mode `0600`); with `GNOMCP_SESSION_PASSPHRASE` set, session files are encrypted at rest with scrypt+AES-256-GCM.

gnomcp does hold its **own** agent key: the dev/test **test1** account on local profiles, and a generated key on testnet profiles (see §2). Both are entirely separate from the user's keystore and valid only on dev/test chains.

## 4. Untrusted-content envelope

The gno chain is open — any realm's content is attacker-influenceable — so all
chain-returned bytes are untrusted. The control differs by delivery channel:

- **Inline text tools** (`gno_render`, `gno_eval`, `gno_packages`, `gno_account`, `gno_status`, `gno_activity`, `gno_history`, `gno_list`) wrap their chain-derived output in an envelope:

  ```
  <untrusted_content kind="eval" source="gno.land/r/demo/foo">
  …
  </untrusted_content>
  ```

  An envelope tag (opening or closing) embedded in chain content is neutralized first, so content cannot escape or forge the envelope. The write tools envelope the realm-controlled portions of their success text the same way: `gno_call`'s `Result` (kind `call_result`), `gno_run`'s `Output` (kind `run_output`), and `gno_cla_info`'s fetched document URL (kind `cla_url`; the required hash is regex-constrained hex and stays inline).

- **The resource tool** (`gno_read`) returns content as an MCP `EmbeddedResource`, a distinct trust posture clients treat as resource data rather than inline instructions. Verbatim source (`full=true`, `symbols`) is not textually wrapped because that would corrupt the txtar archive and break byte fidelity (the body is audit evidence). The default **outline** is server-rendered rather than verbatim, so it additionally neutralizes embedded envelope tags — realm-authored doc comments cannot forge an envelope there. Both paths still rely on the client honoring the resource boundary.

- **Error text** is mixed-trust: gnomcp's own framing can embed chain or network bytes (a realm's panic string in an ABCI log, a faucet's error body). All tool-error text is neutralized at the SDK boundary — embedded envelope tags are escaped — so error text cannot forge or close an envelope; it is not itself enveloped. The faucet error body is additionally labeled `[untrusted faucet response]` at the source.

- **Structured content** (`structuredContent` fields such as `gno_call`'s `result` and `gno_run`'s `output`) carries raw values — it is the machine-readable channel, and wrapping would corrupt consumers. Clients that surface structured fields to a model must apply their own marking.

Anything from any channel must be treated as data, never instructions.

## 5. Output budgeting

Every read and indexer tool applies a ~4 KB budget to chain-returned content. `gno_read`'s bounded and explicit modes — the outline (bodies elided by construction), a named file with `full=true`, a `symbols` fetch — get a ~64 KB tier instead: a higher ceiling sized for real reads, not a bypass. Whole-package raw keeps the tight tier. Over-budget responses are replaced by a summary with a hint to fetch a narrower slice (a specific file or symbol) or view at gnoweb — they are never silently chopped.

## 6. Audit log

- Path: `~/.local/share/gnomcp/audit.jsonl`, mode `0600`.
- One JSONL entry per tool invocation: `{time, tool, profile, args_summary, result, duration_ms, session_address}`.
- `args_summary` keeps only an allowlist of non-sensitive keys; every other arg is redacted. The write-tx tools build their own value-free summary (e.g. `nargs=N`, `code_len=N`) so addresses, amounts, code, and file bodies are never logged.
- Every write attempt is audited — including denials (insufficient_funds, scope_mismatch, validation). Reads opt-in via `--audit-reads`.

## 7. Structured errors

Errors are JSON-encoded payloads with `code`, `message`, and (where useful) extra fields. Notable codes:

| Code | Trigger |
|---|---|
| `chain_id_malformed` | A chain-id carrying shell metacharacters/whitespace was rejected (gno_connect or gno_profile_add) |
| `authentication_required` | A session-signed write was attempted with no active session |
| `scope_mismatch` | The call's realm is not covered by any active session's `allow_paths` |
| `insufficient_funds` | The agent's testnet account is unfunded (run `gno_faucet_fund`) |
| `simulate_unsupported` | `simulate=true` against a client that can't dry-run |
| `agent_identity_unavailable` | Agent identity requested on a profile with no agent key (run `gno_key_generate` for testnet) |
| `key_cap_reached` | The profile already holds `GNOMCP_AGENT_MAX_KEYS` agent keys; delete one (`gno_key_delete`) to free a slot |
| `key_ignored_for_session` | A `key` arg was supplied with `identity=session`, where it does not apply (the session signer is used) |
| `key_has_funds` | `gno_key_delete` on a key that still holds ugnot without `force=true` — sweep with `gno_key_send` first, or force to abandon the funds |
| `cla_unsigned` | A deploy was rejected by the chain's CLA gate; clear it with `gno_cla_info` + `gno_cla_sign` (the hint carries the steps) |
| `hash_required` | `gno_cla_sign` was called without a `hash` — fetch it first with `gno_cla_info` |
| `cla_hash_not_found` | `gno_cla_info` could not extract the required hash from the `r/sys/cla` render (realm format changed?) |
