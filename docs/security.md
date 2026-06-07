# Security Posture

`gnomcp` is the bridge between an LLM and a live blockchain. Every design decision below exists to limit blast radius.

## 1. Chain-id allowlist — betanet/mainnet/staging forbidden

Config validation rejects any profile whose `chain-id` does not match `^(dev|test-?\d+)$`. Betanet (`gnoland1`), `staging`, and mainnet ids cannot enter the config at all. The check runs at startup and at `gnomcp profile add` time. A config that would admit mainnet fails loud; there is no override flag.

## 2. Write authorization: agent identity (dev/test) or chain-bound session

Writes sign with one of two identities — never with the user's key.

**Agent identity — local/dev only.** On `local`/dev profiles the agent signs with its own built-in **test1** account (the well-known *public* test mnemonic). This key is structurally confined to dev/test by three independent gates, all of which must pass:

1. the chain-id allowlist (`^(dev|test-?\d+)$`),
2. a tier check — the keystore returns a signer **only** when `chain-type == local` (`ErrNoAgentKey` otherwise), and
3. tool gating — `gno_addpkg`/`gno_key_address` register only when a local profile exists.

No path derives the agent key for testnet or mainnet. (Testnet agent keys + a faucet are a later phase.)

**Session — testnet.** On non-local profiles the agent acts *as the user* via a chain-bound session:

1. **Profile has a `master-address` (g1...).** Without it the session write path is unavailable for that profile.
2. **User-authorized session.** `gno_session_propose` generates an ephemeral ed25519 keypair and emits a paste-ready `gnokey maketx session create` command. The user runs it; their `gnokey` signs the authorization. gnomcp never sees the user's key or mnemonic.

The session carries an explicit scope: `allow_paths`, `allow_run`, `spend_limit`, and `expires_in`, enforced on every call before broadcast.

The gate is structural (profile + tier + session); there is no opt-in dangerous-tools flag. Every write result names the acting identity so the human always knows which account signed.

## 3. The user's keys never leave gnokey

gnomcp never generates, reads, or stores the **user's** keys — acting as the user is always mediated by a session the user authorizes with their own `gnokey`. The ephemeral session keypair is generated per `gno_session_propose` call and stored only in `~/.local/share/gnomcp/sessions` (mode `0600`); with `GNOMCP_SESSION_PASSPHRASE` set, session files are encrypted at rest with scrypt+AES-256-GCM.

gnomcp does hold its **own** agent key, but only the dev/test **test1** account — derived from the *public* test mnemonic, valid only on dev/test (see §2), and entirely separate from the user's keystore.

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
| `agent_identity_unavailable` | Agent identity requested on a non-local profile (no agent key there) |
