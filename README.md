# gnomcp

[![Release](https://img.shields.io/github/v/release/gnoverse/gno-mcp?sort=semver)](https://github.com/gnoverse/gno-mcp/releases/latest)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)
[![Container](https://img.shields.io/badge/ghcr.io-gnoverse%2Fgnomcp-2496ED?logo=docker&logoColor=white)](https://github.com/gnoverse/gno-mcp/pkgs/container/gnomcp)

> MCP server + agent skill for [gno.land](https://gno.land).

`gnomcp` connects gno.land to any MCP client (Claude Code, Claude Desktop, Cursor, Gemini CLI, OpenCode, …): read realms, evaluate expressions, inspect accounts, manage testnet keys, and simulate or broadcast transactions.

- **MCP server** — 20 tools for reading and writing gno.land, with safety built in: mainnet is locked out, chain output can't hijack the agent, and user keys never leave `gnokey`.
- **`gno` skill** — the knowledge layer for coding agents: interrealm semantics, security taxonomy, idiomatic patterns, `Render()` conventions, stdlib surface.

> [!WARNING]
> **Work in progress — unaudited, pre-release, testnet only.** The tool API can still change, the session write path is young and will be reworked, and there's no guaranteed upgrade path. Only `dev`/`testNN` chain-ids pass validation — mainnet is rejected by design. Read [docs/security.md](docs/security.md) and file issues when something looks off.

## Install

One command installs everything — the server **binary** and the **plugin** (skills + agents). It downloads the binary into `~/.local/bin` (verifying the checksum) and wires up the clients it can: Claude Code and Gemini CLI automatically, Codex and OpenCode with printed steps.

```bash
curl -fsSL https://raw.githubusercontent.com/gnoverse/gno-mcp/main/scripts/install.sh | sh
```

This runs a script from the internet on your machine — read [the script](scripts/install.sh) first.

When it finishes, restart your editor or agent so it loads gnomcp.

Other clients (Cursor, Claude Desktop, …), manual install, building from source, and Docker → **[docs/gnomcp.md](docs/gnomcp.md#install)**.

## Try it

gnomcp ships pointed at the public testnet and a local gnodev node — nothing to configure:

| Profile | Chain-id | RPC |
|---------|----------|-----|
| `testnet` | `test11` | `https://rpc.test11.testnets.gno.land:443` |
| `local` | `dev` | `http://127.0.0.1:26657` (local [gnodev](https://docs.gno.land/builders/local-dev-with-gnodev) node) |

Once it's installed, just talk to your agent in plain language:

- *"render gno.land/r/gnoland/home"* — fetches and returns the realm's page
- *"what's the balance of g1…?"* — inspects an account
- *"call AddPost on gno.land/r/myorg/blog with …"* — builds and broadcasts a transaction

Reads work right away. Writing needs a funded agent key — gnomcp can generate one and request testnet funds for it. On the built-in `local` profile, writes are signed by gnodev's pre-funded `test1` key, so they need no setup at all.

## Tools

The tools cover chain reads, indexer reads, writes, sessions, and agent-key management. Reads work right away against the built-in profiles; the write, session, and indexer tools become available once their prerequisites exist (an agent key or an active session, a profile with an indexer URL). Full catalog → [docs/tools.md](docs/tools.md).

## Configuration

gnomcp can connect to any gno.land testnet or local dev chain — mainnet isn't supported. Beyond the built-in testnet and local defaults, save the chains you use as named **profiles** so gnomcp remembers them between runs: `gnomcp profile add` writes them to `profiles.toml`. A profile can also hold per-chain settings — an indexer URL, or the master address that lets the agent act as you for writes (user sessions).

Profile fields and the agent-key-vs-session signing model → [Configuration](docs/gnomcp.md#configuration) and [Write authorization](docs/gnomcp.md#write-authorization).

## Security posture

- **User keys never leave `gnokey`.** Sessions are authorized by the user running a printed `gnokey` command on their own machine; gnomcp never sees a mnemonic and never asks for one.
- **Mainnet cannot enter the config.** The chain-id allowlist (`dev`, `testNN`) is enforced at validation time — there is no confirm flag to get past it, because there is nothing to confirm against.
- **Untrusted-content envelope on every chain byte.** Tool output derived from the chain is wrapped in `<untrusted_content kind="…" source="…">` with embedded-tag neutralization, so the LLM treats it as data, not instructions.
- **Output budgeting.** Read tools cap inline output and summarize overflows instead of returning chopped halves.
- **Audit log on every write.** JSON-lines, append-only, mode `0600`; records the tool, profile, result, and signing identity (agent vs session address). Operator-facing — not queryable through MCP.
- **Structured errors.** Failures carry a machine-routable code (`insufficient_funds`, `authentication_required`, `scope_mismatch`, …) plus a recovery hint, so agents fail forward instead of guessing.

See [docs/security.md](docs/security.md) for the full posture.

## Skills

The plugin bundles the `gno` skill (knowledge + routing, at `skills/gno/`) plus thin side skills — `gno-audit`, `gno-debug`, `gno-onboard` — that turn its reference library into step-by-step workflows. Authoring conventions → [docs/skills.md](docs/skills.md).

## Development

```bash
make test                # Unit tests (no network)
make test-integration    # In-process node + live tests (build tag: integration)
make lint                # go vet + gofmt -l
make build               # bin/gnomcp
make dev                 # go run ./cmd/gnomcp (starts MCP server)
```

Built against the official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) and `github.com/gnolang/gno/gno.land/pkg/gnoclient` for chain RPC. Testing: see [`test/README.md`](test/README.md) for the four test layers (unit / integration / agent e2e / manual).

## Roadmap

- More side skills driving the tools through common workflows: onboarding, explicit audits, debugging a failed tx
- Docker as the canonical deployment
- a2a serve mode (agent-to-agent realm bridge)
- External security audit before any "stable" claim

## License

Apache-2.0. See [LICENSE](LICENSE).
