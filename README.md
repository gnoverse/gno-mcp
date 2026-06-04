# gnomcp v2

MCP server for [gno.land](https://gno.land). Greenfield rewrite on the `v2` branch.

> **Status:** Milestone A — read-only tools; no writes, no sessions, no a2a yet. The
> `main` branch has the older v1 server with a different architecture; this branch is the
> ground-up rebuild per the ADRs in `adr/`.

## What works in Milestone A

- 4 chain read tools (`vm/q*` ABCI queries):
  - `gno_render` — fetch rendered realm markdown as an MCP resource
  - `gno_eval` — evaluate a Gno expression in a realm's context
  - `gno_read` — read a file (or list files) from a realm package
  - `gno_inspect` — typed godoc for a realm package
- 3 indexer read tools (only registered when at least one profile has `tx-indexer-url`):
  - `gno_list` — filter-browse realms by namespace/tag/category
  - `gno_history` — full transaction history for a realm
  - `gno_activity` — MsgCall/MsgRun log with optional time range
- Multi-chain via `profile` arg on every chain-bound tool; schema-conditional defaulting
  (single profile, local-discovered, multi-no-local)
- JSON-lines audit log (writes always; reads opt-in via `--audit-reads`). No writes in
  Milestone A, so the log is empty unless reads are enabled.
- `gnomcp version` and `gnomcp audit {tail|grep <pattern>}` subcommands.

### Known limitations against the current tx-indexer schema

- `gno_list` returns `error_unavailable: realms query not yet in schema`. The schema
  upgrade (metadata indexing) is tracked upstream in tx-indexer; once it lands, the tool
  starts returning data without code changes.
- `gno_activity` rejects non-nil `since`/`until` with `error_unavailable`: the schema
  has no time field on `Transaction`. Time-range filtering will work after the schema
  exposes block time through the Transaction relation. Calling with no time bounds works
  today — it returns all MsgCall/MsgRun events.

## What's NOT in Milestone A

- Write tools (`gno_call`, `gno_run`) — Milestone B
- Session machinery (`gnokey maketx session create` flow) — Milestone B
- a2a tools, card validation, HTTP transport — Milestone C
- Docker image — Milestone D
- Trust middleware (sanitization, provenance wrap, TOFU) — separate spec

## Milestone B (sessions + writes)

> Status: **In progress.** See `test/e2e/PROTOCOL.md` for the manual e2e checklist.

### 5 new tools

| Tool | Description |
| --- | --- |
| `gno_call` | Broadcast a `MsgCall` to a realm function. Requires an active session (`gno_session_propose` first). |
| `gno_run` | Broadcast a `MsgRun` with ad-hoc Gno code. Requires an active session. |
| `gno_auth_status` | Narrative view of all gnomcp-managed sessions for a profile (no master address needed). |
| `gno_session_propose` | Generate an ephemeral ed25519 keypair and emit a ready-to-run `gnokey maketx session create` command. |
| `gno_session_revoke` | Emit a `gnokey maketx session revoke` command for a gnomcp-managed session. |

Registration gate: all five register only when at least one profile has `allow-dangerous-tools = true`. When unset, gnomcp behaves identically to Milestone A.

### New profile fields (profiles.toml)

```toml
[local]
chain-type = "local"
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
allow-dangerous-tools = true          # required to enable write tools
default-spend-limit  = "100000ugnot" # optional; per-session default
default-expires-in   = "1h"          # optional; Go duration string
bypass-hard-limits   = false         # optional; disables per-chain-type clamps
```

### New env vars and flags

| Name | Default | Purpose |
| --- | --- | --- |
| `GNOMCP_SESSIONS_PATH` | `~/.local/share/gnomcp/sessions` | Directory for session key files |
| `GNOMCP_SESSION_PASSPHRASE` | (unset) | Enables scrypt+AES-256-GCM encryption at rest |
| `--sessions-path` | same as `GNOMCP_SESSIONS_PATH` | CLI override for session storage |

### E2E verification

See `test/e2e/PROTOCOL.md` for the manual checklist (Section A = Milestone A regression, Section B = Milestone B writes).

Run with:
```bash
make test-e2e   # prints instructions; run steps by hand
```

## Quick start

```bash
# Build
make build  # produces bin/gnomcp

# Configure a profile
mkdir -p ~/.config/gnomcp
cat > ~/.config/gnomcp/profiles.toml <<'EOF'
[test11]
chain-type = "testnet"
rpc-url = "https://rpc.test11.testnets.gno.land:443"
chain-id = "test11"
# Optional: enables the 3 indexer tools.
# tx-indexer-url = "<your tx-indexer endpoint>"
EOF

# Configure Claude Code in ~/.claude.json or project-local .mcp.json:
# {
#   "mcpServers": {
#     "gnomcp": { "command": "/absolute/path/to/bin/gnomcp" }
#   }
# }
```

`profiles.toml` rejects unknown keys, so premature write-field keys
(`allow-dangerous-tools`, etc. from Milestone B) will fail loud rather than be silently
ignored.

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
| `prxxxx_tool_surface.md` | 13-tool inventory across the milestones |
| `prxxxx_docker_default_deployment.md` | Docker as canonical deployment (Milestone D+) |
| `prxxxx_session_authorization.md` | OAuth-style session signing for writes (Milestone B+) |
| `prxxxx_a2a_serve_mode.md` | a2a-realm protocol bridge (Milestone C+) |

Built against the official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) and
`github.com/gnolang/gno/gno.land/pkg/gnoclient` for chain RPC. tx-indexer GraphQL is
hand-rolled against the schema at `gnolang/tx-indexer/serve/graph/schema/`.

## Development

```bash
make test                # Unit tests (no network)
make test-integration    # Live smoke against testnet11 (requires network)
make lint                # go vet + gofmt -l
make build               # bin/gnomcp
```

Project layout:

```
cmd/gnomcp/              # Entry point — flags, wire-up, MCP SDK
internal/
  audit/                 # JSON-lines audit log
  chain/                 # vm/q* abstraction (Client / Real / Fake / Resolver)
  indexer/               # tx-indexer GraphQL (Client / GraphQL / Fake / Resolver)
  profiles/              # profiles.toml loader + validator + local discovery
  server/                # MCP server scaffold, tool Registry, profile schema
  tools/read/            # 4 chain read tool registrations
  tools/indexer/         # 3 indexer read tool registrations
test/integration/        # Live smoke (build tag: integration)
adr/                     # Architecture Decision Records
```
