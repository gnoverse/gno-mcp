# Session-as-OAuth: how `gno-mcp` authorizes itself

> Status: v0.2 demo. The session keypair backend is a stub (`crypto/ed25519` + `gmcp1…` bech32 address). Once gnopie is reachable as a Go library, the same flow will swap to `tm2/pkg/crypto/keys` and emit real `g1…` addresses without touching the auth flow above the keypair.

## Why this exists

Most "give the agent a key" flows are wrong in one of two ways:

1. **The user pastes a primary key into the agent.** Now the agent can sign anything the user can sign, forever. There is no scope, no expiry, no clean revocation, and the agent has to be trusted with the seed material.
2. **The user creates a separate key by hand**, then has to track which one belongs to which agent, fund each one, audit each one, etc. High friction; people give up and reuse keys.

`gno-mcp` does neither. The MCP server **owns its own session key**, generates it on demand, and asks the user to *authorize* the session — analogous to an OAuth handoff. The user's primary wallet keeps the high-value money; the session key only ever has whatever the user explicitly funded it with.

## The model

```
┌──────────────────┐                       ┌────────────────────┐
│  Claude / agent  │  invokes write tool   │     gno-mcp        │
│                  ├──────────────────────▶│  session: pending  │
└──────────────────┘                       │  no funds yet      │
        ▲                                  └─────────┬──────────┘
        │                                            │
        │ authentication_required                    │
        │ { fund_url, qr_ascii, session_address }    │
        │ ◀──────────────────────────────────────────┘
        │
        │ user opens fund_url / scans QR in their wallet
        │ wallet signs a transfer to session_address
        ▼
┌──────────────────┐                       ┌────────────────────┐
│   user wallet    │── ugnot transfer ────▶│  session_address   │
│   (primary key)  │                       │  on-chain          │
└──────────────────┘                       └─────────┬──────────┘
                                                     │
                                       balance ≥ threshold
                                                     │
                                                     ▼
                                           ┌────────────────────┐
                                           │     gno-mcp        │
                                           │ session: authentic-│
                                           │ ated, writes work  │
                                           └────────────────────┘
```

## States

| State | Meaning | What the next write does |
|---|---|---|
| `unauthenticated` | No keypair yet — fresh process. | `gno_call` → return auth payload (also generates the keypair) |
| `pending` | Keypair generated, address not yet funded. | `gno_call` → return auth payload |
| `authenticated` | Balance ≥ `threshold_ugnot`. | `gno_call` signs with the session and proceeds |
| `expired` | Authenticated session dropped below `threshold/2`. | `gno_call` → return auth payload with `code: authentication_expired` |

Hysteresis is deliberate. Without it, a single broadcast that crosses the threshold flips state and re-prompts the user mid-flow — bad UX. The half-threshold floor gives the session room to use what's been authorized.

## The auth payload

When a write tool returns `authentication_required`, the structured error carries:

```json
{
  "code": "authentication_required",
  "message": "MCP session is pending; user must authorize the session…",
  "hint": "open the fund_url (or scan qr_ascii) in your gno wallet…",
  "data": {
    "state": "pending",
    "network": "staging.gno.land",
    "session_address": "gmcp1…",
    "threshold_ugnot": 1000000,
    "current_balance_ugnot": 0,
    "fund_url": "gnoland://send?to=gmcp1…&amount=1000000ugnot&memo=gno-mcp-auth",
    "qr_ascii": "▄▄▄▄▄▄▄ ▄▄  …",
    "web_fund_url": "https://staging.gno.land/r/sys/wallet?send_to=gmcp1…",
    "human_guidance": "Send at least 1000000 ugnot to gmcp1… …"
  }
}
```

Clients should render one of `qr_ascii`, `fund_url`, or `web_fund_url` based on what surface the user is on (terminal, mobile wallet, web wallet). The `gno-session-auth` skill picks the right one and walks the user through.

## Read-only tools are not gated

`gno_get`, `gno_eval`, `gno_read`, `gno_inspect`, `gno_address_info`, `gno_network_info`, `gno_audit_tail`, `gno_config_get`, `gno_auth_status` all work without authorization. The session is checked only when a tool would *sign* something.

This is deliberate: the demo runnable in Docker with **no flags, no key, no setup** should still be useful from minute zero. Reads work; writes prompt for authorization the first time they're needed.

## Persistence

| Mode | Default | How |
|---|---|---|
| In-memory | yes | Session keypair lives in the process; container restart → new session, user re-authorizes. |
| Encrypted file | v0.3 | `GNO_MCP_SESSION_FILE=/path/session.enc` + `GNO_MCP_SESSION_PASSPHRASE=…`. AES-GCM, env-supplied key. **Not implemented in v0.2** — the env variables are reserved. |

In-memory is the right default for the demo: it makes the threat model explicit (key dies with the process), and Docker-friendly without volumes.

## What's deliberately not in v0.2

- **Real keypair backend.** Stubbed at `crypto/ed25519` + `gmcp1…` HRP so we can demo the flow before the gnopie-as-library question lands. Swap is a one-file change in `internal/session/address.go`.
- **Encrypted file persistence.** Env variables are reserved; implementation is v0.3.
- **Time-bound expiry.** Today, "expired" means balance dropped. Time-bound expiry needs an upstream session-key registration realm (see `gno_session_*` stubs from v0.1).
- **Per-tool / per-realm scope.** All writes today share one session. Scope-limited sessions land alongside the upstream session-key contract.

## See also

- [README.md](../README.md) for the broader project frame.
- [docs/security.md](security.md) for the full security posture; auth model is section 9.
- [skills/gno-session-auth/SKILL.md](../skills/gno-session-auth/SKILL.md) — the Claude skill that drives this flow.
