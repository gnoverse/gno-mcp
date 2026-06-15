# gnomcp setup & usage

Everything beyond the [README](../README.md) quickstart: full install steps for each client, running in Docker, configuring chains, and authorizing writes.

## Contents

- [Install](#install)
  - [Claude Code](#claude-code)
  - [Codex CLI](#codex-cli)
  - [OpenCode](#opencode)
  - [Gemini CLI](#gemini-cli)
  - [Cursor](#cursor)
  - [Other MCP clients](#other-mcp-clients)
- [Docker](#docker)
- [Configuration](#configuration)
- [Write authorization](#write-authorization)

## Install

gnomcp is two parts, and every client needs both: the **binary** (the MCP server — [release archives](https://github.com/gnoverse/gno-mcp/releases/latest) for linux/darwin × amd64/arm64) and the **plugin** (the skills + agents, installed through your client's own plugin manager).

The one-line installer downloads the binary (checksum-verified, into `~/.local/bin`) and sets up every client it finds — Claude Code and Gemini CLI are fully automatic; for Codex and OpenCode it prints the steps to run.

```bash
curl -fsSL https://raw.githubusercontent.com/gnoverse/gno-mcp/main/scripts/install.sh | sh
```

This pipes a script straight from the internet into your shell. Open [scripts/install.sh](../scripts/install.sh) and read it first — that's good practice for any `curl | sh`.

Pass options after `sh -s --`: `--harness claude|gemini|codex|opencode|none` (repeatable), `--bin-dir DIR`, `--version vX.Y.Z`.

Prefer not to run a script? Download `gno-mcp_<os>_<arch>.tar.gz` from the [releases page](https://github.com/gnoverse/gno-mcp/releases/latest), put `gnomcp` wherever you like, and follow your client's section below.

**When it's done, restart your client.** Plugins and MCP servers load at startup, so a window that was already open won't see gnomcp.

### Claude Code

Run the installer above — or paste this prompt into a session and let the agent do it (short enough to read first; each step runs under your normal permission prompts):

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

Add the plugin to your `opencode.json` and restart OpenCode — details in [.opencode/INSTALL.md](../.opencode/INSTALL.md):

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

## Docker

gnomcp speaks MCP over stdio, so the image runs attached. Multi-arch images (amd64/arm64) are published to GitHub Container Registry on every release:

```json
{
  "mcpServers": {
    "gnomcp": { "command": "docker", "args": ["run", "-i", "--rm", "ghcr.io/gnoverse/gnomcp:latest"] }
  }
}
```

Docker is fine for a quick, isolated server, but **not the best choice for local development**: the container can't reach a local gnodev node on `127.0.0.1`, and its state (audit log, sessions, agent keys) is wiped on exit unless you mount a volume at `/home/nonroot/.local/share/gnomcp`. For day-to-day local work, use the binary.

Pin a version with `ghcr.io/gnoverse/gnomcp:0.2.0` instead of `:latest`. Images carry a signed build-provenance attestation — verify with `gh attestation verify oci://ghcr.io/gnoverse/gnomcp:0.2.0 --repo gnoverse/gno-mcp`.

For in-repo development register a dev server once: `claude mcp add gnomcp -- go run ./cmd/gnomcp` (do **not** add a `.mcp.json` at the repo root — the Claude Code plugin is sourced from the whole repo, so a root `.mcp.json` would ship to every plugin user as a broken server definition).

## Configuration

Profiles are the source of truth for which chains gnomcp can reach. The built-in `local` and `testnet` are read-only defaults. Only `dev` and `testNN` chain-ids are accepted — betanet, staging, and mainnet ids cannot enter a profile.

gnomcp can connect to a chain on the fly during a session, but a chain added that way isn't saved. Persist the ones you want it to remember between runs with `gnomcp profile add` (below); a saved profile is also what user-session writes need, since it carries the `master-address`.

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
