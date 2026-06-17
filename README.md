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

## What you can do

gnomcp ships pointed at the public testnet and a local gnodev node — nothing to configure:

| Profile | Chain-id | RPC |
|---------|----------|-----|
| `testnet` | `test-13` | `https://rpc.test13.testnets.gno.land:443` |
| `local` | `dev` | `http://127.0.0.1:26657` (local [gnodev](https://docs.gno.land/builders/local-dev-with-gnodev) node) |

Then talk to your agent in plain language. New to gno.land? Just ask it to teach you — it gauges your background and gives you a hands-on tour (the `gno-onboard` skill). Otherwise:

### Explore a chain

> *"Which chain is this, is the node live, and what realms exist under gno.land/r/test? Show me what the counter realm renders."*

Reads live state — node status, the realm catalog, account balances, a realm's source and rendered page — grounded in real queries, never guessed. Works immediately, on any chain.

### Build and deploy a realm — `gno-build`

> *"Deploy a check-in board at gno.land/r/test/checkin: anyone can check in, it records their address, and reading it back lists everyone so far. Then check in yourself."*

Your agent writes the realm, tests it locally, runs a security pass, then — once you pick where it runs — funds a key from the faucet, deploys, and makes a real call to prove it works. You get working on-chain code and the transaction that proves it.

### Audit a realm before you trust it — `gno-audit`

> *"Give me a formal security audit of gno.land/r/test/vault before I route user data through it."*

Fetches the on-chain source (read-only — works on mainnet too) and returns an evidence-backed report: findings with quoted lines and severity, plus an honest note on what it did and didn't check. Nothing is mutated.

### Debug a failed transaction — `gno-debug`

> *"My transaction failed with insufficient_funds — what happened?"*

Classifies the error, reproduces it cheaply without broadcasting, applies the fix, and re-runs to prove it works — telling you which identity signed each attempt.

### Act as yourself, safely — sessions

> *"From now on write as my account g1… , not your own key. Bump the counter at gno.land/r/test/counter as me."*

Your agent proposes a scoped session and hands you a `gnokey` command to authorize on your own machine — it never touches your keys. Once you approve, it writes as you, within the limits you set, and tells you exactly how to revoke.

Under all of these, the `gno` skill gives your agent the language, idioms, and security model; the workflows above build on it. Skill authoring → [docs/skills.md](docs/skills.md).

## Tools

23 tools, grouped by what they touch:

- **Chain reads** — render realms, evaluate expressions, read packages, inspect accounts and status. Work immediately.
- **Indexer reads** — list realms, deploy and transaction history, on-chain activity. Need a profile with an indexer URL.
- **Writes** — call functions, run code, deploy packages. Need a funded agent key or an active session.
- **Sessions & keys** — propose and revoke user sessions; generate, list, fund, delete agent keys, and transfer ugnot between a profile's own keys.

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

- Thin the skills toward the [gnolang/gno](https://github.com/gnolang/gno) monorepo as the single source of truth, less hand-distilled content
- Docker as the canonical deployment
- a2a serve mode (agent-to-agent realm bridge)
- External security audit before any "stable" claim

## License

Apache-2.0. See [LICENSE](LICENSE).
