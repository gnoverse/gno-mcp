# Security Posture

`gnomcp` is the bridge between an LLM and a live blockchain. Every design decision below exists to limit blast radius.

## 1. Chain-id allowlist — betanet/mainnet/staging forbidden

Config validation rejects any profile whose `chain-id` does not match `^(dev|test-?\d+)$`. Betanet (`gnoland1`), `staging`, and mainnet ids cannot enter the config at all. The check runs at startup and at `gnomcp profile add` time. A config that would admit mainnet fails loud; there is no override flag.

## 2. Write authorization: master-address + chain-bound session

Write tools (`gno_call`, `gno_run`) require two conditions:

1. **Profile has a `master-address` (g1...).** Without it the write tools are not even registered — they do not appear in the MCP tool list.
2. **User-authorized chain-bound session.** `gno_session_propose` generates an ephemeral ed25519 keypair and emits a paste-ready `gnokey maketx session create` command. The user runs it; their `gnokey` signs the session authorization. gnomcp never sees the user's key or mnemonic.

The session carries an explicit scope: `allow_paths` (realm paths the session may call), `allow_run` (whether MsgRun is permitted), `spend_limit` (maximum ugnot), and `expires_in`. gnomcp enforces the scope on every write call before broadcasting.

The gate is structural: profile configuration + session authorization. There is no opt-in dangerous-tools flag.

## 3. Keys never leave gnokey

gnomcp does not generate or store user keys. The ephemeral session keypair is generated per `gno_session_propose` call and stored only in `~/.local/share/gnomcp/sessions` (mode `0600`). If `GNOMCP_SESSION_PASSPHRASE` is set, session key files are encrypted at rest with scrypt+AES-256-GCM.

## 4. Untrusted-content envelope

Every tool that returns external bytes wraps the body in:

```
<untrusted_content kind="render" source="gno.land/r/demo/foo">
…
</untrusted_content>
```

Anything inside must be treated as data, never instructions.

## 5. Output budgeting

`gno_read` and `gno_render` apply a ~4 KB budget. Over-budget responses are truncated with a hint to fetch a narrower slice or view at gnoweb — they are never silently chopped.

## 6. Audit log

- Path: `~/.local/share/gnomcp/audit.jsonl`, mode `0600`.
- One JSONL entry per tool invocation: `{time, tool, profile, result, args}`.
- `args` is redacted of any sensitive fields before logging.
- Writes are always audited; reads opt-in via `--audit-reads`.

## 7. Structured errors

All errors are JSON-encoded payloads with `code`, `message`, and an actionable `hint`. Notable codes:

| Code | Trigger |
|---|---|
| `session_required` | Write tool called without an active session |
| `session_expired` | Session exists but has expired |
| `session_scope_denied` | Call target not in the session's `allow_paths` |
| `chain_id_forbidden` | Config validation rejected a mainnet/betanet/staging chain-id |
| `invalid_argument` | Schema-level validation in tools |
