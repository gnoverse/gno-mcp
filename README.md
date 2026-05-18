# gno-mcp

> MCP server + Claude skills for [gno.land](https://gno.land).

`gno-mcp` exposes gno.land as a [Model Context Protocol](https://modelcontextprotocol.io) server: read realms, evaluate expressions, manage testnet keys, simulate and broadcast transactions — all from any MCP-aware client (Claude Code, Claude Desktop, Cursor, etc.). The MCP holds its own session key and authorizes itself OAuth-style by asking the user to fund it from their primary wallet — it never touches the user's seed. The repo also ships five Claude skills that drive the tools through the common workflows: session auth, onboarding, exploring a realm, reading a contract, debugging a failed tx.

---

> [!WARNING]
> **Work in progress. Unaudited. Pre-release.**
>
> - **No security review.** The threat model exists on paper (see [docs/security.md](docs/security.md)) but no third party has audited the code, the prompts, or the skill flows.
> - **API surface is unstable.** Tool names, argument shapes, and error codes may change before v0.2 is tagged.
> - **Mainnet is gated for a reason.** Writes against `gno.land` require explicit `confirm=true`. Do not disable that gate. Do not point this at a wallet holding real funds you cannot afford to lose.
> - **Session keypair is a v0.2 stub.** The session backend is `crypto/ed25519` + a `gmcp1…` bech32 address, not real `g1…` gno addresses. The auth flow shape is real and final; the keypair swap to `tm2/pkg/crypto/keys` is a one-file change once gnopie is reachable as a Go library.
> - **The real gnopie client is not wired yet.** Until [gnolang/gno#5444](https://github.com/gnolang/gno/pull/5444) tags a library-friendly release, `gno-mcp` runs against an in-memory fake — tools work end-to-end over MCP, but they are not actually talking to a chain.
> - **No upgrade path guaranteed.** Audit log shape, config schema, and the `GnopieClient` interface will move; expect breakage between pre-1.0 tags.
>
> Use it on testnet. Read the security doc. File issues when something looks off.

---

## Why this exists

LLMs are starting to drive real on-chain actions, but the bridge between an LLM and a live chain is where most things go wrong: leaked seeds, prompt-injected memos, transactions broadcast on the wrong network, errors that read like noise. The MCP ecosystem solves the "how does the model call a tool" half, but the "what does the tool look like, how is it bounded, and how do we keep the model on rails" half is per-domain.

`gno-mcp` is that bridge for gno.land. It ships **two coupled artifacts** so the bridge is not just a tool surface but a usable one:

1. **An MCP server** with read tools (work without a key) and write tools (gate on a self-owned session key) under one security spine — untrusted-content envelopes on every external byte, mainnet write-gate on every state-changer, audit log on every call, mnemonics never observed.
2. **A skills pack** — five Claude skills that drive the tools through the workflows users actually have (session-auth, onboard, explore, read, debug-tx). Skills are first-class shipped artifacts, not afterthoughts; they encode the security posture for the LLM in language the LLM can route on.

## The auth model (v0.2)

Most "give an agent a key" flows end up either pasting the user's primary key into the agent (no scope, no expiry, full blast radius) or asking the user to manage a second key by hand (high friction → people give up and reuse keys).

`gno-mcp` does neither. The MCP **owns its own session key**, generates it on demand, and asks the user to *authorize* the session — analogous to OAuth.

```
agent ──→ MCP (write)                  
            └─ session: pending        
agent ←── { code: authentication_required,
            session_address: gmcp1…,
            fund_url, qr_ascii, threshold_ugnot }
user wallet ──→ session_address (Send ugnot)
            └─ balance ≥ threshold
            └─ session: authenticated
agent ──→ MCP (write) ──→ signs with session key
```

Read-only tools (`gno_get`, `gno_eval`, `gno_read`, `gno_inspect`, `gno_address_info`, `gno_network_info`, `gno_audit_tail`, `gno_auth_status`) work without any authorization. The MCP runs in Docker with **no flags, no key, no setup** — reads work from minute zero, the first write prompts the user to authorize. Full details in [docs/auth.md](docs/auth.md).

## How it is shaped

A few decisions worth knowing if you are reading the code:

- **The MCP holds its own session key.** No user seed material ever enters the agent. The session is OAuth-shaped: fresh process → no key → user funds the session → session signs writes. State machine + payload shape are in [`internal/session`](internal/session); the keypair backend is currently `crypto/ed25519` + `gmcp1` HRP, with a one-file swap path to `tm2/pkg/crypto/keys` once gnopie ships as a library.
- **Wrap `gnopie`'s Go internals as a library, not a subprocess.** Typed errors, no shell-injection surface, faster cold path. The seam is `internal/client.GnopieClient`, so the real client and an in-memory fake are interchangeable. The fake still backs v0.2; the real wiring lands once PR 5444 is library-friendly.
- **Security is a property of the package, not a checklist.** Every tool inherits the envelope (`mcp.UntrustedEnvelope`), the structured-error vocabulary (`authentication_required`, `authentication_expired`, `confirmation_required`, `mainnet_write_blocked`, `onboarding_required`, `not_implemented`, `invalid_argument`), the audit log (`audit.Log`), and the mainnet-confirm gate by construction. New tools can't accidentally skip them.
- **Output budgeting is mandatory.** Read-shaped tools (`gno_get`, `gno_read`) apply a 4 KB budget by default. Over-budget responses return a `summary` pointing at gnoweb instead of a chopped half-source. Explicit slice requests (symbol/file/lines) bypass the budget.
- **Skills are markdown, not code.** Claude's skill router reads frontmatter; no build step, no DSL. Authoring conventions live in [docs/skills.md](docs/skills.md).
- **Tests use an in-process MCP client, not stdio.** `internal/mcp/testmcp` wires `mcp-go`'s `NewInProcessClient` against a fake `GnopieClient`. Tests exercise the full JSON-RPC layer (initialize → list → call) with no subprocess and no port allocation.
- **No mnemonic, ever.** No user-supplied keys at all. The MCP doesn't accept, ask for, store, or display mnemonics; the session key is generated in-process and bounded to whatever the user explicitly funded. The audit log redacts `mnemonic`, `password`, `private_key` even if a tool accidentally accepts them.

## Status

v0.2 implementation: session-as-OAuth auth model + 17 MCP tools shipping, `-race` clean across 8 packages. Read-only tools work without any setup. First write triggers the auth flow. See [docs/tools.md](docs/tools.md) for the tool surface and [docs/auth.md](docs/auth.md) for the auth model.

## Install

### Local binary

    go install github.com/gnolang/gno-mcp/cmd/gno-mcp@latest

### Docker (no setup)

    docker run --rm -i ghcr.io/gnolang/gno-mcp:latest

The container is stdio-only — no port exposed. Pipe an MCP client to it. Read tools work immediately; the first write returns an `authentication_required` payload with a fund link and QR.

Release binaries (linux/darwin × amd64/arm64) are published via [goreleaser](.goreleaser.yaml) once tagged.

## Configure

Copy `.mcp.json.example` into your MCP client config:

```json
{
  "mcpServers": {
    "gno": {
      "command": "gno-mcp",
      "args": []
    }
  }
}
```

Per-user state lives at:

- Audit log: `~/.gno-mcp/audit.jsonl` (mode `0600`)
- Config: `$XDG_CONFIG_HOME/gno-mcp/config.json` (`GNO_MCP_CONFIG` overrides)

Environment variables:

- `GNO_MCP_NETWORK` — default network for the session (default: `staging.gno.land`)
- `GNO_MCP_SESSION_FILE` / `GNO_MCP_SESSION_PASSPHRASE` — reserved for encrypted-at-rest session persistence (v0.3)

## Skills

The `skills/` directory is a Claude plugin package containing five skills:

- **gno-session-auth** — walks the user through authorizing the MCP session (`authentication_required` → fund link/QR)
- **gno-onboarding** — first-time testnet setup, no mnemonic shown
- **gno-explore-realm** — point-paste a realm path, get a structured tour
- **gno-read-contract** — read source in slices, surface invariants and risk
- **gno-debug-tx** — classify a failed tx and propose a concrete fix

See [docs/skills.md](docs/skills.md).

## Tools

17 MCP tools: `gno_auth_status`, `gno_network_info`, `gno_get`, `gno_eval`, `gno_read`, `gno_inspect`, `gno_address_info`, `gno_keygen`, `gno_faucet_request`, `gno_call`, `gno_run`, `gno_session_{create,revoke,list}` (stubs), `gno_config_{get,set}`, `gno_audit_tail`. See [docs/tools.md](docs/tools.md).

## Security posture

- **Never asks for, displays, or stores the user's primary key.** The MCP holds its own session keypair, generated in-process; the user authorizes the session by funding its address. Blast radius = whatever the user funded the session with.
- **Read-only tools work without authorization.** First write triggers the auth flow with a `gno_auth_status`-shaped payload (`session_address`, `fund_url`, `qr_ascii`, `threshold_ugnot`). Re-prompts only when the session expires.
- **Mainnet writes gated.** `gno_call` and `gno_run` always simulate first. On `gno.land` the broadcast is suppressed unless `confirm=true` is set; the response carries `confirmation_required: true` so the caller can echo the security block to the user.
- **Faucet refuses mainnet.** `gno_faucet_request` returns a `mainnet_write_blocked` structured error if pointed at `gno.land`.
- **Untrusted-content envelope.** Every tool that returns external bytes (Render output, source, eval results) wraps them in `<untrusted_content kind="…" source="…">…</untrusted_content>` so the LLM treats them as data, not instructions.
- **Audit log on every write.** JSONL append-only, args redacted (`mnemonic`, `password`, `private_key` stripped). Tail via `gno_audit_tail`.

See [docs/security.md](docs/security.md) for the full posture and [docs/auth.md](docs/auth.md) for the session-auth model.

## Development

```sh
make test    # go test -race ./...
make e2e     # build + stdio smoke (asserts every v0.1 tool is registered)
make lint    # vet + gofmt -l -d
make install # go install ./cmd/gno-mcp
```

The repo currently uses an in-memory fake `GnopieClient`. Wiring the real gnopie client (Task 22) requires a local gno checkout and a `replace` directive — instructions will land alongside the upstream release.

## Roadmap

- **v0.1**: read tools + write flow + skills *(shipped)*
- **v0.2**: session-as-OAuth auth model, `gno_auth_status`, Dockerfile, session-auth skill *(this milestone)*
- **v0.3**: real gnopie/tm2 keys backend, encrypted session-file persistence, tx-indexer integration (history/search), upstream session-key contract (time-bound + scoped)
- **post-v1.0**: external security audit before any "stable" claim

## License

Apache-2.0. See [LICENSE](LICENSE).
