# Security Posture

`gno-mcp` is the bridge between an LLM and a live blockchain. The blast radius of a careless tool call is real money on mainnet, leaked credentials, or unsigned transactions broadcast at the wrong moment. Every design decision below exists to keep the LLM on rails.

## 1. The MCP never holds the user's primary key

This is the most important property. The agent is not trusted with the user's seed. There is no path — by config, by tool, by prompt — that asks the user for a mnemonic, private key, or wallet export.

Instead, the MCP **owns its own session keypair**:

- Generated in-process on demand (lazy: state stays `unauthenticated` until the first write attempt).
- Stored in memory by default; encrypted-file persistence is reserved (`GNO_MCP_SESSION_FILE` + `GNO_MCP_SESSION_PASSPHRASE`) for v0.3.
- The session address (`gmcp1…` in v0.2 stub, `g1…` once tm2 keys is wired) is what the user funds from their primary wallet.

Blast radius is bounded to whatever the user explicitly funded the session with. Stopping the agent terminates the session (memory mode). The user's primary wallet is never touched by the MCP.

See [docs/auth.md](auth.md) for the full session model.

The legacy `gno_keygen` tool from v0.1 — which generates a *named* gnokey-stored key — still exists and still returns only `{name, address, pubkey}` (never a mnemonic), but it is no longer the path the agent takes to sign writes; the session is.

## 2. Mainnet writes are gated by an explicit confirm

`gno_call` and `gno_run`:

1. Always run `CallSimulate` / `RunSimulate` first — gas is estimated against the live chain, no broadcast.
2. Build a `SecurityBlock` that summarises network, signer, simulated gas, estimated cost, and a `confirmation_required` flag.
3. If the network resolves to `gno.land` (mainnet) **and** `confirm=true` is not set, the response is returned with `confirmation_required: true` and `broadcast: null` — no broadcast happens.
4. The caller (skill or human) is expected to echo the security block, get the user's go-ahead, and re-call with `confirm=true`.

Successful simulations and broadcasts are logged with explicit `sim_err:` / `broadcast_err:` / `ok` result prefixes so each step is distinguishable in the audit tail.

## 3. Faucet refuses mainnet

`gno_faucet_request` returns a structured `mainnet_write_blocked` error when network resolves to `gno.land`. Hint points the caller at a testnet domain like `staging.gno.land`.

## 4. Untrusted-content envelope

Every tool that returns external bytes — Render output (`gno_get`), eval results (`gno_eval`), source slices (`gno_read`) — wraps the body in:

```
<untrusted_content kind="render" source="gno.land/r/demo/foo">
…
</untrusted_content>
```

The envelope is the contract: anything inside MUST be treated as data, never instructions. Skills repeat this rule for the LLM in their `Judgment` sections.

## 5. Output budgeting

`gno_get` and `gno_read` apply a 4 KB budget (`budget.DefaultBudget`). When over-budget and no slice was requested, the response carries a `summary` pointing at the canonical gnoweb URL — never a chopped half-source. When a slice **is** requested (`symbol`/`file`/`lines`), the full slice is returned regardless of size.

## 6. Audit log

- Path: `~/.gno-mcp/audit.jsonl`, mode `0600`.
- One JSONL entry per tool invocation: `{time, tool, network, signer, tx_hash, result, args}`.
- `args` runs through `audit.RedactArgs` which strips `password`, `mnemonic`, `private_key`.
- Tail via the `gno_audit_tail` MCP tool.
- Session-signed writes show `signer: "mcp-session"` so authorised vs. user-provided signers are distinguishable.

## 7. Structured errors

All errors are JSON-encoded `StructuredError` payloads with `code`, `message`, an actionable `hint`, and (for auth) a `data` block the LLM can route on:

| Code | Trigger | Notes |
|---|---|---|
| `authentication_required` | Write tool called with empty signer and the MCP session is `pending` / `unauthenticated`. | `data` carries `session_address`, `fund_url`, `qr_ascii`, `threshold_ugnot`. The `gno-session-auth` skill consumes this. |
| `authentication_expired` | Authorised session balance dropped below half the threshold. | Same payload shape as `authentication_required`. |
| `confirmation_required` | Mainnet write submitted without `confirm=true`. | Soft block — response still carries the security block; caller re-submits with `confirm=true`. |
| `mainnet_write_blocked` | Faucet hit on `gno.land`. | Hard block. Switch to a testnet domain. |
| `onboarding_required` | Legacy: explicit signer passed but not configured. | Kept for the `gno_keygen` / external-key path; the session path uses `authentication_required` instead. |
| `not_implemented` | Session-key stubs (`gno_session_create/revoke/list`). | Blocked on upstream session-key contract. |
| `invalid_argument` | Schema-level validation in tools. | Always recoverable by fixing args. |

Skills switch on `code` to route the user to the right next step.

## 8. Session lifecycle and hysteresis

The session has four states: `unauthenticated`, `pending`, `authenticated`, `expired`. Transitions:

- `unauthenticated → pending` on first `EnsurePending` (lazy keypair generation).
- `pending → authenticated` when the balance fetcher reports `balance ≥ threshold`.
- `authenticated → expired` when balance drops below `threshold / 2` (hysteresis — a single broadcast that crosses the threshold does *not* re-prompt the user mid-flow).
- `expired → authenticated` on re-funding through the same flow.

Transient RPC failures during `Refresh` do **not** change state. The session stays authorised through brief outages.

## 9. What gno-mcp deliberately doesn't do (yet)

- **No real keypair backend.** v0.2 stubs `crypto/ed25519` + `gmcp1` HRP so the auth flow is demonstrable. Swap to `tm2/pkg/crypto/keys` is a one-file change in `internal/session/address.go` once gnopie is library-friendly.
- **No encrypted-file session persistence.** Env variables `GNO_MCP_SESSION_FILE` / `GNO_MCP_SESSION_PASSPHRASE` are reserved; implementation is v0.3.
- **No time-bound or scoped session expiry.** Today the only expiry is balance-based. Time-bound + per-realm scope land alongside the upstream session-key contract.
- **No real gnopie wiring.** v0.2 ships against the in-memory fake. The interface (`internal/client/GnopieClient`) is the seam; the swap is a `replace` directive + one file.
- **No Windows audit-log testing.** Linux/darwin only for v0.2.
- **No external security audit.** Pre-1.0.
