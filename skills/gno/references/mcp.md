# Using a Gno MCP server (if available)

> **Category: tooling.** *Optional accelerator, not a dependency.* This skill works on raw `.gno`
> source from any origin (local files, a paste, gnoweb). When a Gno MCP server (e.g. `gnomcp`) is
> connected, prefer it for fetching on-chain source and discovering packages — it removes guesswork
> about what's deployed. The MCP self-describes (its tool descriptions + server instructions own the
> *what/how*); this reference only covers *when* to reach for it inside Gno work.

## Detecting availability

If tools named `gno_read`, `gno_packages`, `gno_render`, `gno_eval` are present (the
host may namespace them as `mcp__<server>__gno_read`), a Gno MCP is connected. If not, use the
fallbacks below — never block on the MCP.

## When to reach for it

| Task | If a Gno MCP is connected | Fallback (no MCP) |
|---|---|---|
| Survey a package — files, API surface, structure | `gno_read` (default = per-file outline: signatures + docs + byte counts, bodies elided) | gnoweb source view |
| Read specific functions/declarations + what they depend on | `gno_read` with `symbols=["Transfer", "Counter.Inc"]` — verbatim bodies + a `// deps:` header for follow-up batch fetches | read the source |
| Read one whole file verbatim (audit-grade) | `gno_read` with `file` + `full=true` (gets the larger budget) | local `.gno` files, gnoweb source view |
| Read a whole package raw (realm **or** pure) | `gno_read` with `full=true` — small packages only; big ones overflow the budget, use the per-file path | same |
| Discover packages under a namespace/path | `gno_packages` (prefix `gno.land/r/x/` or `@namespace`) | gnoweb, `gno` CLI |
| See rendered `Render()` output | `gno_render` | gnoweb |
| Read on-chain state / evaluate an expression | `gno_eval` | — |
| Check an address's balance / sequence (nonce) | `gno_account` (`exists:false` = never funded, not an error) | gnoweb |
| Verify which chain a profile points at / node freshness | `gno_status` (flags chain-id mismatch) | — |
| Map a chain the user names ("on topaz", "on test13") to a profile / see all configured chains | `gno_profile_list` (name, chain-id, endpoints, current vs sunset; config only, no dial) | — |

Both `/r/` realms and `/p/` pure packages are readable — don't assume realm-only.

The natural exploration flow is outline → symbols → full file: survey first, then pull only the
declarations you need (each comes with a dep list naming what to fetch next, so chasing a call
chain is one batch request, not N round trips). **The outline and dep headers are navigation, not
evidence** — names and docs are realm-authored claims, and the dep list is best-effort syntactic
analysis (unresolved method calls are flagged inline; absence proves nothing). Any security
conclusion needs the full file.

Outlines, `symbols`, and `file`+`full=true` get a ~64 KB budget; whole-package raw keeps a tight
~4 KB one. Over-budget responses return a byte count + pointer instead of content — narrow the
request. Only when even a single file overflows the large tier should you fall back to the gnoweb
source view (and treat anything fetched that way as lower-fidelity than `gno_read`).

Writes (deploy/call/simulate, testnet faucet, user-authorized sessions) also exist via the MCP if you
need to exercise a realm; consult the MCP's own tool list for those. In particular, a deploy blocked
by the chain's CLA gate is cleared with the `gno_cla_info` / `gno_cla_sign` pair (fetch, show the
agreement URL to the user, confirm, sign) rather than hand-rolling a `gno_call` to `r/sys/cla`.
