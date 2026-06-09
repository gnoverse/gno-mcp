# Using a Gno MCP server (if available)

> **Category: tooling.** *Optional accelerator, not a dependency.* This skill works on raw `.gno`
> source from any origin (local files, a paste, gnoweb). When a Gno MCP server (e.g. `gnomcp`) is
> connected, prefer it for fetching on-chain source and discovering packages — it removes guesswork
> about what's deployed. The MCP self-describes (its tool descriptions + server instructions own the
> *what/how*); this reference only covers *when* to reach for it inside Gno work.

## Detecting availability

If tools named `gno_read`, `gno_packages`, `gno_inspect`, `gno_render`, `gno_eval` are present (the
host may namespace them as `mcp__<server>__gno_read`), a Gno MCP is connected. If not, use the
fallbacks below — never block on the MCP.

## When to reach for it

| Task | If a Gno MCP is connected | Fallback (no MCP) |
|---|---|---|
| Read a package's full source (realm **or** pure) | `gno_read` with no `file` → whole package as txtar | local `.gno` files, gnoweb source view |
| Read one file | `gno_read` with `file` | same |
| Discover packages under a namespace/path | `gno_packages` (prefix `gno.land/r/x/` or `@namespace`) | gnoweb, `gno` CLI |
| Understand the API surface without full source | `gno_inspect` | read the source |
| See rendered `Render()` output | `gno_render` | gnoweb |
| Read on-chain state / evaluate an expression | `gno_eval` | — |

Both `/r/` realms and `/p/` pure packages are readable — don't assume realm-only.

Writes (deploy/call/simulate, testnet faucet, user-authorized sessions) also exist via the MCP if you
need to exercise a realm; consult the MCP's own tool list for those.
