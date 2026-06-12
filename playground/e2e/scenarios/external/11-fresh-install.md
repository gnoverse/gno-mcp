---
id: fresh-install
tier: external
category: install
image: l1-fresh
timeout-minutes: 20
covers: [install.binary-release, install.plugin-marketplace, install.mcp-register, install.no-stray-server, install.skills-live]
---
# A first-time user pastes the install prompt into clean Claude Code and ends up fully installed

Driver context: the AUT container is the `l1-fresh` image — clean Claude Code, no Go
toolchain, no gno tooling, `~/.local/bin` NOT on PATH. Requires egress to github.com
(release download + marketplace clone), hence external tier. No simnet/gnoquery/gnokey
in this image: every Verify fact is turn-log or container-state (see verify-toolkit.md
§ Container state — always `-w /home/dev/work`). Step 1's Instruct is the product's
copy-paste install prompt (README § Install); if the published prompt changes, this
Instruct changes with it. The no-stray-server fact asserts the repo stays free of a
root `.mcp.json` (a plugin sourced from `./` ships any root `.mcp.json` to every user
as an auto-registered broken server) — a regression there fails this scenario only
after a push to main, since the marketplace installs from GitHub, not the local tree.

## Step 1: paste-the-install-prompt
### Instruct
Install gnomcp: download the right gno-mcp_<os>_<arch>.tar.gz for this machine from https://github.com/gnoverse/gno-mcp/releases/latest, extract the gnomcp binary to ~/.local/bin/gnomcp, and check that `gnomcp version` prints a version. Then add the Claude Code plugin: `claude plugin marketplace add gnoverse/gno-mcp` and `claude plugin install gnomcp@gnoverse`. Finally register the MCP server: `claude mcp add gnomcp --scope user -- ~/.local/bin/gnomcp`, and verify with `claude mcp list`.
### Expect
- correctness: every phase (binary, plugin, MCP registration, verification) completes; the AUT never asks the user for input or gives up; the final answer reports the gnomcp server as connected.
- correctness: the platform-matched archive was downloaded (asset name embeds the container's actual os/arch).
- correctness: a binary sanity check ran and the AUT reports a concrete version string.
- correctness: the final state contains no failed/broken MCP server entry — and the AUT does not have to explain one away.
### Verify
- Container state: `~/.local/bin/gnomcp` exists and is executable; running it with `version` exits 0 and prints a semver.
- Container state: `~/.claude/plugins/installed_plugins.json` lists `gnomcp@gnoverse`.
- Container state: `claude mcp list` (cwd `/home/dev/work`) reports server `gnomcp` as connected and contains NO `Failed to connect` line.
- Turn log: no `tool_use` edits files under `~/.claude/` directly (the install goes through the `claude` CLI, not hand-written config).

## Step 2: surface-check-next-session
### Instruct
Before I start using it — what gno-related skills and gnomcp tools do you have available now? Just list them; don't call anything on-chain.
### Expect
- correctness: the answer names the gno skill family — gno, gno-audit, gno-build, gno-debug, gno-onboard.
- correctness: the gnomcp MCP server is reported available/connected, with no confusion about broken or duplicate servers.
### Verify
- Container state: the five skill directories exist under the installed plugin copy (`~/.claude/plugins/cache/gnoverse/gnomcp/*/skills/`).
- Turn log: the answer text names all five skills.

## Debrief
- How did you decide which release asset to download?
- Did any step behave differently from what the instructions implied, or need a retry?
- If a teammate had only that install prompt and no other docs, what would you add or change in it?
