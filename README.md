# gnomcp

[![Release](https://img.shields.io/github/v/release/gnoverse/gno-mcp?sort=semver)](https://github.com/gnoverse/gno-mcp/releases/latest)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)
[![Container](https://img.shields.io/badge/ghcr.io-gnoverse%2Fgnomcp-2496ED?logo=docker&logoColor=white)](https://github.com/gnoverse/gno-mcp/pkgs/container/gnomcp)

> MCP server + agent skill for [gno.land](https://gno.land).

`gnomcp` connects gno.land to any MCP client (Claude Code, Claude Desktop, Cursor, Gemini CLI, OpenCode, …): read realms, evaluate expressions, inspect accounts, manage testnet keys, and simulate or broadcast transactions.

- **MCP server** — the tools to read and write gno.land from your agent, with safety built in.
- **`gno` skill** — the knowledge layer for coding agents: interrealm semantics, security taxonomy, idiomatic patterns, `Render()` conventions, stdlib surface.

> [!WARNING]
> **Work in progress — unaudited and pre-release.**
>
> - The tool API can still change, and the session write path will be reworked.
> - Writes are confined to dev/testnet — no code path signs on mainnet or betanet.
> - No guaranteed upgrade path yet.
>
> Read [docs/security.md](docs/security.md) and file issues when something looks off.

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

Reads work right away. Writing needs a funded agent key — gnomcp can generate one and request testnet funds. On the `local` profile, writes use gnodev's pre-funded `test1` key, so they need no setup.

## Tools

20 tools, grouped by what they touch:

- **Chain reads** — render realms, evaluate expressions, read packages, inspect accounts and status. Work immediately.
- **Indexer reads** — list realms, deploy and transaction history, on-chain activity. Need a profile with an indexer URL.
- **Writes** — call functions, run code, deploy packages. Need a funded agent key or an active session.
- **Sessions & keys** — propose and revoke user sessions, generate and fund agent keys.

Full catalog → [docs/tools.md](docs/tools.md).

## Configuration

gnomcp can reach any gno.land chain. Dev and testnet chains are read/write; mainnet and betanet are read-only — inspect and audit deployed code, but no signing on real-funds chains.

Beyond the built-in `testnet` and `local` defaults, save the chains you use as named **profiles** with `gnomcp profile add` (written to `profiles.toml`), so gnomcp remembers them between runs. A profile can also carry an indexer URL, or a master address for user-session writes (dev/testnet only).

Profile fields and the signing model → [Configuration](docs/gnomcp.md#configuration) · [Write authorization](docs/gnomcp.md#write-authorization).

## Security

- **Keys stay in `gnokey`** — gnomcp never sees a mnemonic; the user signs sessions on their own machine.
- **No signing on real-funds chains** — writes are gated to dev/testnet; mainnet and betanet are read-only.
- **Chain output can't hijack the agent** — every chain-derived byte is wrapped in an untrusted-content envelope.
- **Bounded reads** — output is budgeted and summarized, never silently truncated.
- **Every write is logged** — an append-only audit trail of the tool, profile, result, and signer.
- **Structured errors** — machine-routable codes and recovery hints, so the agent fails forward.

Full posture and threat model → [docs/security.md](docs/security.md).

## Skills

The plugin teaches your agent gno.land — one deep skill plus focused workflows it routes to:

- **gno** — write and modify realm code: interrealm calls, payments, `Render()`, project setup. The reference library the others build on.
- **gno-onboard** — bring a newcomer up to speed, adapting to what they already know.
- **gno-audit** — review a realm before you trust it ("is this safe?", pre-funding checks).
- **gno-debug** — trace a failed transaction back to its cause.

Authoring conventions → [docs/skills.md](docs/skills.md).

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
