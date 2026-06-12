# Verify toolkit

A scenario's `### Verify` states **binding facts**, not commands. You establish each fact
however you like with the tools below. The fact binds; the method is yours. Three fact kinds:
**chain ground truth** (use `gnoquery`), **turn-log behavior** (use the transcript schema),
and **container state** (use `docker exec`).

## `gnoquery` — chain ground-truth oracle

A Go binary baked into the e2e image (`playground/e2e/cmd/gnoquery`), backed by `gnoclient` —
the same pinned library gnomcp wraps, but called below gnomcp's tool layer, so it is an
independent check of what actually landed on chain. You run host-side, so exec it in the
container, where the simnet RPC (`testnet.gnomcp.sim:26687`) resolves:

```
docker exec "${E2E_CONTAINER:-gnomcp-e2e}" gnoquery <command>
```

| Command | Prints | Use for |
|---|---|---|
| `status` | `chain-id: <id>` / `height: <n>` | chain identity + liveness |
| `height` | `<n>` | latest block height |
| `render <pkgpath> [path]` | the realm's `Render(path)` output | a realm's rendered state |
| `eval <pkgpath> <expr>` | the typed result (e.g. `(1 int64)`) | reading a value/getter |
| `balance <bech32>` | the account's coins (e.g. `1000000ugnot`) | funds checks |

Exit codes: **0** ok, **1** RPC/query error (e.g. realm not found — the message is on stderr),
**2** usage/bad-address. A non-zero exit on a `render`/`eval` is itself ground truth that the
realm/expression is absent — that confirms "does not exist" facts.

Read the output and compare against the AUT's claim. An answer the chain contradicts is `fail`
no matter how plausible — never skip the chain check on a step that has one.

## Turn-log behavior — the transcript schema

Each turn is `<scenario-run-dir>/turn-<n>.jsonl` (stream-json). Establish behavioral facts from
the structured events, never by grepping raw text (the SKILL.md body name-drops every reference,
so a filename string appears the moment a skill loads — see judging.md § Observing):

- **Tool call**: an `assistant` event whose `message.content[]` has `type == "tool_use"` with
  `.name` and `.input`. Filter on `.name` (e.g. `Skill`, `Read`, `Agent`, `mcp__gnomcp__gno_*`)
  and inspect `.input` (e.g. `.input.file_path`, `.input.subagent_type`).
- **Final answer**: the `result` event (`.result` text, `.is_error`).
- **No-call facts** ("did NOT call X"): assert the absence of a matching `tool_use` across the
  step's turns — count matches and expect zero.

`jq` is available; treat any jq in a scenario as an example you may adapt, not a command to run
verbatim. Useful shapes:

```
# tool_use names in a turn
jq -r 'select(.type=="assistant").message.content[]?|select(.type=="tool_use").name' turn-N.jsonl
# Read file_paths (judge reference/skill loads by these, not by raw grep)
jq -r 'select(.type=="assistant").message.content[]?|select(.type=="tool_use" and .name=="Read").input.file_path' turn-N.jsonl
```

The AUT loads MCP tools lazily — a `ToolSearch`/deferred-tool load before the first gnomcp call
is normal, never a finding.

## Container state — `docker exec`

Scenarios whose outcome lives in the container itself (installs, config changes, file
state — typically non-e2e images, where there is no chain to ask) state facts about the
container. Establish them from the host:

```
docker exec -w /home/dev/work "${E2E_CONTAINER:-gnomcp-e2e}" <command>
```

Examples: a file exists and is executable (`test -x`), a command exits 0 and prints an
expected shape (`~/.local/bin/gnomcp version`), Claude Code config state (`claude mcp list`,
`~/.claude/plugins/installed_plugins.json`). Always set a cwd outside any repo checkout
(`-w /home/dev/work`) — project-scope config (e.g. a `.mcp.json` in a cloned repo) leaks
into `claude` CLI output otherwise. These checks are YOUR actions; they never appear in the
AUT transcript and never collide with the gnokey hard-fail rule.
