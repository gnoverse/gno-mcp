# a2a v1.0 Integration: Serve Mode + Realm-as-Source-of-Truth

## Context

The meta-issue (`.mynote/gno-agentic/issues/meta-issue.md`) lists a2a v1.0 task interfaces as the **default agent-write path** for v2 (T8: realm-as-a2a-service pattern). The session-authorization ADR established sessions as the substrate for all chain-bound writes. This ADR addresses how gnomcp v2 exposes the a2a protocol surface to (a) Claude via MCP/stdio and (b) external a2a clients (CrewAI, LangGraph, etc.).

Research into the a2a v1.0 spec and SDK ecosystem (`a2aproject/A2A`, `a2aproject/a2a-python`, `a2aproject/a2a-js`, community clients) confirmed:

- a2a v1.0 transport is HTTP/JSON-RPC, gRPC, or HTTP+JSON/REST. All network-based.
- Stdio transport for a2a is proposed (`a2aproject/A2A#1074`) under TSC review but not yet in the spec or any official SDK.
- No major a2a client (CrewAI, LangGraph) supports stdio or local subprocess transport today.
- `a2a-python` has a `ClientFactory.register()` extensibility point, but the agent-card resolver is httpx-hardcoded; bypassing it requires non-trivial SDK patching.

Implication: to interop with current external a2a clients, gnomcp must speak HTTP. The architectural question is **how to host HTTP without requiring a long-running background daemon** — daemons add operational complexity (systemd / launchd / `docker run -d` / Unix socket IPC / shim binary for MCP transport) that the v2 base architecture explicitly avoided.

## Decision

gnomcp v2 has **two operational modes**, both foreground and user-controlled. **No background daemon.**

### Mode 1: `gnomcp stdio` (default, v2 base, unchanged)

Spawned by Claude Code via MCP stdio. Lifetime tied to the Claude session. Existing v2 base tool surface plus the three new a2a tools (see below).

### Mode 2: `gnomcp a2a --serve <realm>` (new for a2a interop)

User-invoked, foreground, per-realm process. On startup:

1. Loads or proposes a session for the target profile + realm (per the session-authorization ADR's flow).
2. Prints the `gnokey maketx session create` command if a fresh session is needed, then waits for chain activation.
3. Binds HTTP on `127.0.0.1:7777` (configurable via `--port`).
4. Prints endpoint URL + bearer token for the user to hand to their a2a client.
5. Serves the a2a v1.0 protocol for the configured realm.
6. User Ctrl+C to stop; process exits, session remains on chain.

The single-realm scope is enforced at CLI parse time. The internal handler registry is keyed by realm path (`map[realmPath]→handler`); supporting `--serve r/foo --serve r/bar` in a future release is a one-line CLI validation lift.

### Realm as source of truth for task state

Per meta-issue T8, the realm implements the a2a v1.0 task lifecycle (8 states) **on chain**. The realm exports Gno methods like:

```go
func SubmitTask(cur realm, method string, args interface{}) string  // returns task_id
func GetTaskStatus(taskID string) (state string, result interface{})
func CancelTask(cur realm, taskID string) error                     // optional
func AgentCard() AgentCard
```

**gnomcp does NOT maintain an internal task state machine.** It is a thin bridge:

| a2a operation | gnomcp action |
|---|---|
| `submit_task` (HTTP POST or MCP tool) | Validate args against the realm's agent card; call realm's `SubmitTask` via session signer |
| `get_task_status` (HTTP GET or MCP tool) | ABCI query against the realm's `GetTaskStatus`; no session needed |
| `cancel_task` (HTTP DELETE or MCP tool) | Call realm's `CancelTask` via session signer if exposed; best-effort otherwise |
| SSE stream of state transitions | Poll `GetTaskStatus` at intervals (default 2s); push diffs to subscribed clients |

### Tool surface delta from v2 base

Three new MCP tools, plus one extension:

- `gno_inspect` (extended) — now returns the realm's agent card alongside existing godoc + signatures
- `a2a_submit_task(profile, realm, method, params)` — typed write, **always-on** (not gated by `allow-dangerous-tools`), session-signed
- `a2a_get_task_status(profile, realm, task_id)` — read-only chain query, always-on
- `a2a_cancel_task(profile, realm, task_id)` — typed write, always-on, session-signed, realm-dependent

The a2a tools are **typed-validated against the realm's agent card**. gnomcp refuses to invoke methods not in the card or with args that don't match the declared schema. This is structurally narrower than `gno_call` (which is open-ended); the dangerous-tools gate exists for `gno_call`'s shape, not for typed calls.

### Card-validation extends to all writes

For card-having realms, **gnomcp refuses to invoke any method not in the card, regardless of which tool was used.** `gno_call` against a card-having realm is also validated against the card. The card becomes the gnomcp-side ABI for any realm that publishes one. Card-less realms retain open-ended `gno_call` behavior under the existing `allow-dangerous-tools` gate.

### HTTP endpoints and auth

a2a HTTP endpoints follow a2a v1.0 spec paths:

- `GET /agents/<realm>/.well-known/agent-card.json` — fetched from chain at startup, served as JSON
- `POST /agents/<realm>/tasks` — submit
- `GET /agents/<realm>/tasks/<task_id>` — status
- `GET /agents/<realm>/tasks/<task_id>/events` — SSE stream
- `DELETE /agents/<realm>/tasks/<task_id>` — cancel

Transport auth uses a bearer token generated per spawn and written to `/var/lib/gnomcp/api-key`. a2a clients read the file and present `Authorization: Bearer <token>` on every request. Tokens die with the process; restart yields a fresh token. Binding is `127.0.0.1` only by default; external exposure requires explicit `--bind 0.0.0.0` and triggers a startup warning.

### Agent card endpoint URL

The card stored on chain uses a logical URI in its `url` field (e.g., `gno://gno.land/r/myorg/blog` — gnomcp's read tools emit the same scheme, so a single convention covers both card URLs and tool resource URIs). gnomcp rewrites this at serve time when delivering the card to a2a clients, substituting `http://127.0.0.1:7777/agents/<realm>/`. The chain-stored card stays portable and host-independent; gnomcp does the local binding substitution.

## Alternatives considered

**Long-running daemon (`gnomcp serve` style).** A background process exposing HTTP for a2a clients and Unix-socket IPC for an MCP-stdio bridge binary. Sessions shared across all clients via daemon memory. Rejected: significant operational complexity (systemd / launchd / docker daemon lifecycle / Unix socket / shim binary). The per-spawn `a2a --serve` model achieves the same HTTP interop with materially simpler ops.

**Stdio transport for a2a (matching `a2aproject/A2A#1074`).** Mirror MCP's pattern: a2a clients spawn `gnomcp a2a-stdio` per session. Rejected for Phase 1: no a2a client supports this transport today; we'd be the only consumer of our own transport. Reconsider when #1074 lands and SDK adoption follows.

**gnomcp as its own agent identity (its own a2a agent card).** Considered but rejected as confusing — gnomcp is the runtime, realms are the agents. Adding gnomcp-identity-on-top-of-realm-identity creates ambiguity in audit, signing, and discovery. Service-discovery (which realms does this gnomcp bridge?) can be a small registry tool if needed, not an identity layer.

**gnomcp maintains its own task state machine.** Considered for simplicity (no chain cost per task; faster status queries from in-process map). Rejected: tasks would be lost on restart; tasks invisible to other gnomcp processes or observers; meta-issue T8 explicitly says the realm holds the state machine. Aligning with the meta-issue and getting persistent on-chain task state for free is the right trade.

**Gating a2a tools by `allow-dangerous-tools`.** Considered for consistency with `gno_call`. Rejected because the trust posture differs: `gno_call` is dangerous because it's open-ended; a2a tools are typed-validated against the realm's published card. Session authorization (which AllowPaths the user signed for) is the gate that matters for typed calls. Forcing `allow-dangerous-tools` for the safe path would discourage adoption.

## Consequences

- Two clean modes, no daemon, no shim, no Unix socket. Each mode has obvious lifecycle.
- External a2a clients (CrewAI, LangGraph) work via `a2a --serve` with no special client-side support — standard HTTP + bearer token.
- Task state is naturally persistent (on chain), naturally observable (chain reads), naturally auditable (every state transition is a tx).
- Realm authors must implement the a2a task-lifecycle pattern. A community `p/.../a2atask` library is the expected path; without it, realms are responsible for their own state-machine boilerplate.
- Every task submission is a chain write — gas cost per task. Acceptable for typed-bounded work; not for trivial high-volume operations.
- SSE streaming is polling-based (gnomcp polls the realm); could be optimized via tx-indexer subscription if/when that becomes available.
- gnomcp's `internal/tasks/` package is a polling/streaming helper, not a state machine — materially less code than the alternative.
- Tool count rises from v2 base's 13 to 16 with all opt-ins enabled. Within mcp-creator's 1–15 sweet spot edge but each new tool is justified by a concrete use case.
- The card-validation extension means `gno_call` against card-having realms is now safer than v2 base implied. Card-less realms retain the original open-ended posture.
- Stdio a2a transport (and daemon-less external-client interop without an HTTP listener) waits on `a2aproject/A2A#1074` landing and SDK adoption. Expected to be revisited in a follow-up ADR when that happens.
- Bearer-token-per-spawn means a2a clients re-read the token file after each gnomcp restart. Minor friction; acceptable.
