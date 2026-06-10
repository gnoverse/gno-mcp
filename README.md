# gnomcp v2

MCP server for [gno.land](https://gno.land). Greenfield rewrite on the `v2` branch.

> **Status:** read + write tools; session-gated writes; multi-chain via profiles. The
> `main` branch has the older v1 server with a different architecture; this branch is the
> ground-up rebuild per the ADRs in `adr/`.

## Zero-config reads

gnomcp ships with two built-in profiles that require no configuration:

| Profile | Chain-id | RPC |
|---------|----------|-----|
| `local` | `dev` | `http://127.0.0.1:26657` (auto-discovered local node) |
| `testnet` | `test11` | `https://rpc.test11.testnets.gno.land:443` |

The five chain read tools (`gno_render`, `gno_eval`, `gno_read`, `gno_inspect`, `gno_packages`) and the `gno_connect` discovery tool work immediately against these profiles — no config file needed. Both built-ins are also agent-capable, so the agent-identity write tools (see below) register out of the box.

## Quick start

### Install

```bash
go install github.com/gnoverse/gno-mcp/cmd/gnomcp@latest
```

### MCP client config (installed binary)

```json
{
  "mcpServers": {
    "gnomcp": { "command": "gnomcp", "args": [] }
  }
}
```

For in-repo development the `.mcp.json` at the repo root uses `go run ./cmd/gnomcp` instead.

### Chain-id allowlist

Only `dev` and `testNN` chain-ids are accepted. Betanet (`gnoland1`), `staging`, and mainnet ids are rejected at config validation — they cannot enter a profile.

## Profiles

Profiles are the source of truth for which chains gnomcp can reach. The built-in `local` and `testnet` are read-only defaults.

### Adding a profile

```bash
# From a gnoweb URL (autofills rpc + chain-id from the page's gnoconnect meta-tags)
gnomcp profile add mychain --from-gnoweb https://mychain.testnets.gno.land

# Manual
gnomcp profile add mychain --rpc https://rpc.mychain.gno.land:443 --chain-id test99

# With master address to enable writes
gnomcp profile add mychain --from-gnoweb https://mychain.testnets.gno.land \
  --master g1youraddresshere
```

Profiles are written to `~/.config/gnomcp/profiles.toml`. A project-local `./profiles.toml` overlays the global file; the `-config` flag overrides both.

**Config precedence:** built-in defaults < `~/.config/gnomcp/profiles.toml` < `./profiles.toml` < `-config` flag.

A profile entry in a config file is a whole-profile replacement — an overlay redefining a built-in must re-supply `rpc-url` and `chain-id`, not just `master-address`.

```bash
gnomcp profile list    # show all active profiles
gnomcp profile remove mychain
```

### Profile fields (profiles.toml)

```toml
[mychain]
rpc-url              = "https://rpc.test99.testnets.gno.land:443"
chain-id             = "test99"
master-address       = "g1..."       # enables session writes — the agent acting as this user (bech32)
tx-indexer-url       = "..."         # optional; enables gno_list/gno_history/gno_activity
default-spend-limit  = "100000ugnot" # optional; per-session default
default-expires-in   = "1h"          # optional; Go duration string
faucet-url           = "..."         # optional; faucet page gno_faucet_fund links the user to
faucet-service-url   = "..."         # optional; automatic faucet service gno_faucet_fund calls
```

## Write authorization

Writes are signed by one of two identities, chosen per call via the `identity` arg:

- **Agent identity (default on local and testnet).** Local profiles sign with the built-in
  `test1` key — no setup. Testnet profiles sign with a per-profile key: run
  `gno_key_generate` once, then fund it (`gno_faucet_fund` or send it ugnot).
- **Session — the agent acts as the user (requires `master-address`).** Call
  `gno_session_propose` to get a paste-ready `gnokey maketx session create` command; run it
  to authorize a chain-bound session with explicit scope (`allow_paths`, `allow_run`,
  `spend_limit`, `expires_in`). Pass `identity=session` to force this path on any profile.

```text
# Typical session flow
gno_session_propose(profile="mychain", allow_paths=["gno.land/r/myorg/blog"])
# → prints gnokey command; user runs it
gno_call(profile="mychain", realm="gno.land/r/myorg/blog", func="AddPost", identity="session", ...)
```

Every write result names the identity that signed it.

Session key files are stored in `~/.local/share/gnomcp/sessions` (mode `0600`). Set `GNOMCP_SESSION_PASSPHRASE` to enable encryption at rest.

## Tools

See [`docs/tools.md`](docs/tools.md) for the full catalog. Summary (18 tools):

| Tool | Category | Registered when |
|------|----------|---------|
| `gno_render` | chain read | always |
| `gno_eval` | chain read | always |
| `gno_read` | chain read | always |
| `gno_inspect` | chain read | always |
| `gno_packages` | chain read | always |
| `gno_connect` | discovery | always |
| `gno_list` | indexer read | a profile has `tx-indexer-url` |
| `gno_history` | indexer read | a profile has `tx-indexer-url` |
| `gno_activity` | indexer read | a profile has `tx-indexer-url` |
| `gno_call` | write | always (agent key or active session) |
| `gno_run` | write | always (agent key or active session) |
| `gno_session_propose` | session | always (needs `master-address` to succeed) |
| `gno_session_revoke` | session | always (needs `master-address` to succeed) |
| `gno_auth_status` | session | always |
| `gno_addpkg` | write | a local or testnet profile exists |
| `gno_key_address` | agent key | a local or testnet profile exists |
| `gno_key_generate` | agent key | a local or testnet profile exists |
| `gno_faucet_fund` | agent key | a testnet profile exists |

## Skill installation (for AI coding agents)

The repo bundles a `gno` skill at `skills/gno/` covering interrealm semantics, the
security taxonomy, idiomatic patterns, Render() conventions, the stdlib surface, the
memory model, and project setup. It installs as a plugin for the major coding-agent
harnesses.

| Agent | Install |
| --- | --- |
| **Claude Code** | `/plugin marketplace add gnoverse/gno-mcp` then `/plugin install gnomcp` |
| **Codex CLI** | Install via Codex's plugin flow pointing at `.codex-plugin/plugin.json` in this repo |
| **Cursor** | Install via Cursor's plugin flow; reads `.cursor-plugin/plugin.json` |
| **Gemini CLI** | `gemini extensions install https://github.com/gnoverse/gno-mcp` |
| **OpenCode** | Add `"gnomcp@git+https://github.com/gnoverse/gno-mcp.git"` to your `opencode.json` `plugin` array. See `.opencode/INSTALL.md`. |

The skill is independent of the MCP server — installing one does not require the other,
but they're complementary (skill = knowledge, MCP server = on-chain tools).

## Architecture

ADRs in `adr/`:

| ADR | What |
| --- | ---- |
| `prxxxx_multichain_via_profiles.md` | Profile-arg model with schema-conditional defaulting |
| `prxxxx_tool_surface.md` | Tool inventory (read, write, session) |
| `prxxxx_docker_default_deployment.md` | Docker as canonical deployment (future) |
| `prxxxx_session_authorization.md` | OAuth-style session signing for writes |
| `prxxxx_a2a_serve_mode.md` | a2a-realm protocol bridge (not yet built) |

Built against the official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) and
`github.com/gnolang/gno/gno.land/pkg/gnoclient` for chain RPC. tx-indexer GraphQL is
hand-rolled against the schema at `gnolang/tx-indexer/serve/graph/schema/`.

## Development

```bash
make test                # Unit tests (no network)
make test-integration    # Live smoke against testnet11 (requires network)
make lint                # go vet + gofmt -l
make build               # bin/gnomcp
make dev                 # go run ./cmd/gnomcp (starts MCP server)
```

Project layout:

```
cmd/gnomcp/              # Entry point — flags, wire-up, MCP SDK, profile subcommand
internal/
  audit/                 # JSON-lines audit log
  chain/                 # vm/q* abstraction (Client / Real / Fake / Resolver)
  indexer/               # tx-indexer GraphQL (Client / GraphQL / Fake / Resolver)
  profiles/              # profiles.toml loader + validator + local discovery
  server/                # MCP server scaffold, tool Registry, profile schema
  session/               # Session key management and scope enforcement
  keystore/              # Per-profile agent keys (local test1, testnet generated)
  tools/read/            # 6 chain/discovery read tool registrations
  tools/indexer/         # 3 indexer read tool registrations
  tools/write/           # 9 write/session/agent-key tool registrations
test/e2e/                # Manual e2e protocol (PROTOCOL.md)
test/integration/        # Live smoke (build tag: integration)
adr/                     # Architecture Decision Records
docs/                    # tools.md, security.md, skills.md
```
