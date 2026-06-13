# gnomcp

> MCP server + agent skill for [gno.land](https://gno.land).

`gnomcp` exposes gno.land as a [Model Context Protocol](https://modelcontextprotocol.io) server: read realms, evaluate expressions, inspect accounts, manage testnet keys, simulate and broadcast transactions — from any MCP-aware client (Claude Code, Claude Desktop, Cursor, Gemini CLI, OpenCode, …). The repo also ships a `gno` skill that teaches coding agents the language, the security taxonomy, and how to drive the tools.

---

> [!WARNING]
> **Work in progress. Unaudited. Pre-release.**
>
> - **No security review.** The threat model is written down ([docs/security.md](docs/security.md)) but no third party has audited the code, the tool descriptions, or the skill.
> - **API surface is unstable.** Tool names, argument shapes, and error codes may change between pre-release tags.
> - **Testnet and local dev only — by construction.** Only `dev` and `testNN` chain-ids pass config validation; mainnet ids are rejected outright and there is no flag to bypass that.
> - **Sessions are WIP.** The session-signed write path (`gno_session_propose` → `identity=session`) works end-to-end but is young and will be reworked — use with caution and tight scopes.
> - **No upgrade path guaranteed.** Config schema, session-file format, and audit-log shape may move.
>
> Use it on testnet. Read the security doc. File issues when something looks off.

---

## Why this exists

LLMs are starting to drive real on-chain actions, and the bridge between an LLM and a live chain is where things go wrong: leaked seeds, prompt-injected realm output, transactions signed on the wrong network. MCP solves the "how does the model call a tool" half; the "how is the tool bounded and how does the model stay on rails" half is per-domain.

`gnomcp` is that bridge for gno.land, shipped as two coupled artifacts:

1. **An MCP server** — 20 tools under a single security spine: untrusted-content envelopes on every chain-derived byte, a chain-id allowlist that keeps mainnet out entirely, output budgeting, an append-only audit log, and user keys that never leave `gnokey`.
2. **A `gno` skill** — the knowledge layer for coding agents: interrealm semantics, security taxonomy, idiomatic patterns, `Render()` conventions, stdlib surface, project setup. Skill and server are independent but complementary (skill = knowledge, server = on-chain tools).

## Zero-config testnet

gnomcp ships pointed at the public testnet — no configuration needed:

| Profile | Chain-id | RPC |
|---------|----------|-----|
| `testnet` | `test11` | `https://rpc.test11.testnets.gno.land:443` |
| `local` | `dev` | `http://127.0.0.1:26657` (local gnodev node — see note) |

The six chain read tools (`gno_render`, `gno_eval`, `gno_read`, `gno_packages`, `gno_account`, `gno_status`) and the `gno_connect` discovery tool work immediately against these profiles — no config file needed. The write tools register out of the box as well: generate an agent key once (`gno_key_generate`), fund it (`gno_faucet_fund`), and the agent can write.

> [!NOTE]
> For local development with [gnodev](https://docs.gno.land/builders/local-dev-with-gnodev), the built-in `local` profile auto-discovers a node on `127.0.0.1:26657` and signs with the pre-funded `test1` key — zero setup, instant writes.

## Install

gnomcp is two artifacts, and every harness needs both: the **binary** (the MCP server — [release archives](https://github.com/gnoverse/gno-mcp/releases/latest) for linux/darwin × amd64/arm64) and the **plugin** (the skills + agents, installed through each harness's own plugin manager). Pick your harness: [Claude Code](#claude-code) · [Codex CLI](#codex-cli) · [OpenCode](#opencode) · [Gemini CLI](#gemini-cli) · [Cursor](#cursor) · [other MCP clients](#other-mcp-clients).

**One-line install** — downloads the binary (checksum-verified, into `~/.local/bin`) and wires every harness it detects (Claude Code and Gemini CLI fully automatic; Codex and OpenCode get their steps printed). The script is short on purpose — read it before piping:

```bash
curl -fsSL https://raw.githubusercontent.com/gnoverse/gno-mcp/main/scripts/install.sh | sh
```

Flags (pass via `sh -s -- <flags>`): `--harness claude|gemini|codex|opencode|none` (repeatable), `--bin-dir DIR`, `--version vX.Y.Z`.

Prefer no script? Download `gno-mcp_<os>_<arch>.tar.gz` from the [releases page](https://github.com/gnoverse/gno-mcp/releases/latest), put `gnomcp` wherever you like, and follow your harness section below.

**When you're done: restart the harness.** Plugins and MCP servers load at session start — a session that was already open will not see gnomcp.

### Claude Code

Run the installer above — or paste this prompt into a session and let the agent do it (short enough to audit first; each step runs under your normal permission prompts):

> Install gnomcp: download the gnomcp binary for this machine from github.com/gnoverse/gno-mcp/releases/latest into ~/.local/bin, install the gno skills plugin with `claude plugin marketplace add gnoverse/gno-mcp` and `claude plugin install gnomcp@gnoverse`, register the MCP server with `claude mcp add gnomcp --scope user -- ~/.local/bin/gnomcp`, verify with `claude mcp list`, and then remind me to restart Claude Code so the plugin loads.

Or by hand, with the binary already at `~/.local/bin/gnomcp`:

```bash
claude plugin marketplace add gnoverse/gno-mcp
claude plugin install gnomcp@gnoverse
claude mcp add gnomcp --scope user -- ~/.local/bin/gnomcp   # absolute path — no PATH assumption
```

(the `plugin` commands also work as `/plugin …` slash commands inside a session)

Restart Claude Code, then check: `claude mcp list` shows `gnomcp … ✔ Connected` and `/gno` is available.

### Codex CLI

Install the plugin via Codex's plugin flow pointing at `.codex-plugin/plugin.json` in this repo, then register the binary (`gnomcp`, no arguments) as a stdio MCP server in your Codex config.

### OpenCode

Add the plugin to your `opencode.json` and restart OpenCode — details in [.opencode/INSTALL.md](.opencode/INSTALL.md):

```json
{ "plugin": ["gnomcp@git+https://github.com/gnoverse/gno-mcp.git"] }
```

### Gemini CLI

```bash
gemini extensions install https://github.com/gnoverse/gno-mcp
```

### Cursor

Install via Cursor's plugin flow; it reads `.cursor-plugin/plugin.json`.

### Other MCP clients

Any MCP host that runs local stdio servers works — install the binary (or build from source: `go install github.com/gnoverse/gno-mcp/cmd/gnomcp@latest`) and point your client at it:

```json
{
  "mcpServers": {
    "gnomcp": { "command": "gnomcp", "args": [] }
  }
}
```

### Docker

gnomcp speaks MCP over stdio, so the image runs attached. Multi-arch images (amd64/arm64) are published to GitHub Container Registry on every release:

```json
{
  "mcpServers": {
    "gnomcp": { "command": "docker", "args": ["run", "-i", "--rm", "ghcr.io/gnoverse/gnomcp:latest"] }
  }
}
```

Pin a version with `ghcr.io/gnoverse/gnomcp:0.2.0` instead of `:latest`. State (audit log, sessions, agent keys) is ephemeral unless you mount a volume at `/home/nonroot/.local/share/gnomcp`. Images carry a signed build-provenance attestation — verify with `gh attestation verify oci://ghcr.io/gnoverse/gnomcp:0.2.0 --repo gnoverse/gno-mcp`.

For in-repo development register a dev server once: `claude mcp add gnomcp -- go run ./cmd/gnomcp` (do **not** add a `.mcp.json` at the repo root — the Claude Code plugin is sourced from the whole repo, so a root `.mcp.json` would ship to every plugin user as a broken server definition).

## Profiles

Profiles are the source of truth for which chains gnomcp can reach. The built-in `local` and `testnet` are read-only defaults. Only `dev` and `testNN` chain-ids are accepted — betanet, staging, and mainnet ids cannot enter a profile.

```bash
# From a gnoweb URL (autofills rpc + chain-id from the page's gnoconnect meta-tags)
gnomcp profile add mychain --from-gnoweb https://mychain.testnets.gno.land

# Manual
gnomcp profile add mychain --rpc https://rpc.mychain.gno.land:443 --chain-id test99

# With master address to enable session writes
gnomcp profile add mychain --from-gnoweb https://mychain.testnets.gno.land \
  --master g1youraddresshere

gnomcp profile list
gnomcp profile remove mychain
```

Profiles are written to `~/.config/gnomcp/profiles.toml`. A project-local `./profiles.toml` overlays the global file; the `-config` flag overrides both.

**Config precedence:** built-in defaults < `~/.config/gnomcp/profiles.toml` < `./profiles.toml` < `-config` flag.

A profile entry in a config file is a whole-profile replacement — an overlay redefining a built-in must re-supply `rpc-url` and `chain-id`, not just `master-address`.

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

- **Agent identity (default).** Testnet profiles sign with a per-profile key: run
  `gno_key_generate` once, then fund it (`gno_faucet_fund` or send it ugnot). Local
  (gnodev) profiles sign with the built-in `test1` key — no setup.
- **Session (WIP) — the agent acts as the user (requires `master-address`).** Call
  `gno_session_propose` to get a paste-ready `gnokey maketx session create` command; run it
  to authorize a chain-bound session with explicit scope (`allow_paths`, `allow_run`,
  `spend_limit`, `expires_in`). Pass `identity=session` to force this path on any profile.

> [!WARNING]
> The session path is functional end-to-end but **WIP** and will be reworked. Use with
> caution: authorize with tight `allow_paths`, a low `spend_limit`, and a short
> `expires_in` — and revoke (`gno_session_revoke`) when you're done.

```text
# Typical session flow
gno_session_propose(profile="mychain", allow_paths=["gno.land/r/myorg/blog"])
# → prints gnokey command; user runs it
gno_call(profile="mychain", realm="gno.land/r/myorg/blog", func="AddPost", identity="session", ...)
```

Every write result names the identity that signed it.

Session key files are stored in `~/.local/share/gnomcp/sessions` (mode `0600`). Set `GNOMCP_SESSION_PASSPHRASE` to enable encryption at rest.

## Tools

See [`docs/tools.md`](docs/tools.md) for the full catalog. Summary (20 tools):

| Tool | Category | Registered when |
|------|----------|---------|
| `gno_render` | chain read | always |
| `gno_eval` | chain read | always |
| `gno_read` | chain read | always |
| `gno_packages` | chain read | always |
| `gno_account` | chain read | always |
| `gno_status` | chain read | always |
| `gno_connect` | discovery | always |
| `gno_profile_add` | admin | always (in-memory profile, gone on restart) |
| `gno_list` | indexer read | a profile has `tx-indexer-url` |
| `gno_history` | indexer read | a profile has `tx-indexer-url` |
| `gno_activity` | indexer read | a profile has `tx-indexer-url` |
| `gno_call` | write | always (agent key or active session) |
| `gno_run` | write | always (agent key or active session) |
| `gno_session_propose` | session | always (needs `master-address` to succeed) |
| `gno_session_revoke` | session | always (needs `master-address` to succeed) |
| `gno_auth_status` | session | always |
| `gno_addpkg` | write | always |
| `gno_key_address` | agent key | always |
| `gno_key_generate` | agent key | always |
| `gno_faucet_fund` | agent key | a testnet profile exists |

Gated tools appear mid-session when `gno_profile_add` flips their gate (the server sends `tools/list_changed`). Dynamic profiles are in-memory, testnet/dev only, and carry no `master-address` — reads and agent-key writes work, sessions require a profile persisted in `profiles.toml`.

## Security posture

- **User keys never leave `gnokey`.** Sessions are authorized by the user running a printed `gnokey` command on their own machine; gnomcp never sees a mnemonic and never asks for one.
- **Mainnet cannot enter the config.** The chain-id allowlist (`dev`, `testNN`) is enforced at validation time — there is no confirm flag to get past it, because there is nothing to confirm against.
- **Untrusted-content envelope on every chain byte.** Tool output derived from the chain is wrapped in `<untrusted_content kind="…" source="…">` with embedded-tag neutralization, so the LLM treats it as data, not instructions.
- **Output budgeting.** Read tools cap inline output and summarize overflows instead of returning chopped halves.
- **Audit log on every write.** JSON-lines, append-only, mode `0600`; records the tool, profile, result, and signing identity (agent vs session address). Operator-facing — not queryable through MCP.
- **Structured errors.** Failures carry a machine-routable code (`insufficient_funds`, `authentication_required`, `scope_mismatch`, …) plus a recovery hint, so agents fail forward instead of guessing.

See [docs/security.md](docs/security.md) for the full posture.

## Skills

The repo bundles the `gno` skill (knowledge + routing, at `skills/gno/`) plus three thin side skills — `gno-audit`, `gno-debug`, `gno-onboard` — that compose its reference library into explicit workflows. Everything ships as one plugin per harness — see [Install](#install).

> [!NOTE]
> The skill's content is currently hand-distilled from the [gnolang/gno](https://github.com/gnolang/gno) monorepo, so it can drift as the language evolves. The goal is a single source of truth — the monorepo as the sole reference — with the skill reduced to a thin wrapper that adds routing, guidance, and best practice on top of monorepo knowledge, never a fork of it.

Authoring conventions live in [docs/skills.md](docs/skills.md).

## Agent faucet (operators)

`agentfaucet` is the optional, standalone HTTP service that funds agent keys on a testnet — a per-chain piece operators run, then advertise to gnomcp via a profile's `faucet-service-url`. It is independent of the MCP server (HTTP-only coupling) and ships as both a release binary (`agentfaucet_<os>_<arch>.tar.gz`) and a multi-arch image `ghcr.io/gnoverse/agentfaucet`.

```bash
docker run --rm -p 8590:8590 \
  -e GNOMCP_FAUCET_MNEMONIC="<funding key mnemonic>" \
  ghcr.io/gnoverse/agentfaucet:latest \
  -rpc-url https://rpc.testN.gno.land:443 -chain-id testN -listen 0.0.0.0:8590
```

The funding mnemonic is read from `GNOMCP_FAUCET_MNEMONIC` (never a flag — a flag default leaks to `-help`/logs). The default `-listen` is `127.0.0.1:8590` for host safety, so in a container you must pass `-listen 0.0.0.0:8590`. Anti-abuse is built in: per-address cooldown, per-IP rate limit, and a hard global daily outflow cap (`-per-addr-cooldown`, `-per-ip-max`, `-per-ip-window`, `-daily-cap`, `-grant`). Only `test*` chain-ids are accepted. `agentfaucet -help` lists every flag.

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
