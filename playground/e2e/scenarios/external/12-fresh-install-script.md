---
id: fresh-install-script
tier: external
category: install
image: l1-fresh
timeout-minutes: 15
covers: [install.script-oneliner, install.script-idempotent]
---
# The README's curl|sh installer sets up everything in one shot — and is safe to re-run

Driver context: the AUT container is the `l1-fresh` image (clean Claude Code, no gno
tooling, `~/.local/bin` not on PATH); GitHub egress required — external tier. Every
Verify fact is turn-log or container-state (verify-toolkit.md § Container state, always
`-w /home/dev/work`). The one-liner in step 1 is the README's quick-install command and
fetches `scripts/install.sh` from raw.githubusercontent on `main` — this scenario asserts
the PUSHED script, not the local tree; keep the Instruct in sync with the README. The
expected script behavior: platform-matched release download, checksum verification
against checksums.txt, binary to `~/.local/bin`, Claude Code wired via its own CLI
(plugin marketplace + plugin install + mcp add), and a restart reminder.

## Step 1: run-the-one-liner
### Instruct
Set up gnomcp for me using its install script — the README says to run: curl -fsSL https://raw.githubusercontent.com/gnoverse/gno-mcp/main/scripts/install.sh | sh — run it and tell me what it did.
### Expect
- correctness: the AUT runs the script (it may download then execute it; reimplementing the steps by hand instead of running the script is a fail — the script is the point).
- correctness: the AUT reports what the script did: checksum verified, binary installed, Claude Code plugin + MCP server wired, and the restart instruction.
- correctness: no step errors and the AUT does not ask the user for input.
### Verify
- Turn log: a Bash `tool_use` executes `scripts/install.sh` (piped from the raw URL or downloaded-then-run); no sequence of hand-rolled `claude plugin`/`claude mcp add` calls replaces it.
- Container state: `~/.local/bin/gnomcp` exists and is executable; running it with `version` exits 0 and prints a semver.
- Container state: `~/.claude/plugins/installed_plugins.json` lists `gnomcp@gnoverse`, and the five gno skill directories exist under `~/.claude/plugins/cache/gnoverse/gnomcp/*/skills/`.
- Container state: `claude mcp list` (cwd `/home/dev/work`) reports server `gnomcp` as connected and contains NO `Failed to connect` line.

## Step 2: rerun-is-safe
### Instruct
I might end up running that install script again on a machine that already has gnomcp — is that safe? Run it once more here and check nothing broke or got duplicated.
### Expect
- correctness: the AUT re-runs the script, it exits successfully, and the AUT confirms the re-run is safe (idempotent) with evidence, not assumption.
- correctness: no duplicate MCP server entries or error states are reported.
### Verify
- Turn log: a second execution of the install script in this step's turns.
- Container state: `claude mcp list` still reports exactly one `gnomcp` server, connected, no `Failed to connect` line.

## Debrief
- Did the script's output tell you everything you needed, or did you have to inspect anything else to answer me?
- If the script had failed partway, what state would this machine be in — could you tell from what you saw?
