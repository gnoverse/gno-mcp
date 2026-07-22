# Installing gnomcp for OpenCode

## Prerequisites

- [OpenCode.ai](https://opencode.ai) installed
- The `gnomcp` binary, for the on-chain tools (the [one-line installer](../README.md#install)
  puts it in `~/.local/bin`). Without it the plugin is skill-only: the `gno` references load,
  but no MCP tools.

## Installation

Add `gnomcp` to the `plugin` array in your `opencode.json` (global or project-level):

```json
{
  "plugin": ["gnomcp@git+https://github.com/gnoverse/gno-mcp.git"]
}
```

Restart OpenCode. The plugin installs through OpenCode's plugin manager and, via
`.opencode/plugins/gnomcp.js`, registers the `gno` skill plus the gnomcp MCP server
(when the binary is on PATH or in `~/.local/bin`).

If the binary lives somewhere else, register the server yourself in `opencode.json`
(a user-configured entry always wins over the plugin's):

```json
{
  "mcp": {
    "gnomcp": { "type": "local", "command": ["/path/to/gnomcp"], "enabled": true }
  }
}
```

## Verify

Ask a Gno-specific question, e.g.:

> What does `cross(cur)` do in interrealm v2?

The `gno` skill should activate via OpenCode's description-match. For the tools, ask
something that needs the chain:

> Which chain is gnomcp pointed at? Use gno_status.

OpenCode uses its own plugin install. If you also use Claude Code, Codex, Cursor, or Gemini CLI, install gnomcp separately for each one — see the repository README for per-agent install paths.
