# Installing gnomcp for OpenCode

## Prerequisites

- [OpenCode.ai](https://opencode.ai) installed

## Installation

Add `gnomcp` to the `plugin` array in your `opencode.json` (global or project-level):

```json
{
  "plugin": ["gnomcp@git+https://github.com/gnoverse/gno-mcp.git"]
}
```

Restart OpenCode. The plugin installs through OpenCode's plugin manager and registers the `gno` skill via `.opencode/plugins/gnomcp.js`.

Verify by asking a Gno-specific question, e.g.:

> What does `cross(cur)` do in interrealm v2?

The `gno` skill should activate via OpenCode's description-match and route you to the relevant references.

OpenCode uses its own plugin install. If you also use Claude Code, Codex, Cursor, or Gemini CLI, install gnomcp separately for each one — see the repository README for per-agent install paths.
