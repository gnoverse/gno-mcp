# Tool Surface (Phase 1)

## Context

v1 exposed 16 MCP tools, including a mix of meta-issue-aligned reads (`gno_render`, `gno_eval`, `gno_read`, `gno_inspect`), v1-original carry-overs (`gno_audit_tail`, `gno_network_info`, `gno_address_info`, `gno_config_get`, `gno_config_set`), open-ended writes (`gno_call`, `gno_run`), key-and-faucet utilities (`gno_keygen`, `gno_faucet_request`), and session-management stubs (`gno_session_create`, `gno_session_revoke`, `gno_session_list`).

The meta-issue (`.mynote/gno-agentic/issues/meta-issue.md`, `.mynote/gno-agentic/issues/sub-mcp-read.md`) specifies Phase 1 T1 as four chain reads (`gno_render`, `gno_eval`, `gno_read`, `gno_inspect`) plus three indexer-backed reads (`gno_list`, `gno_history`, `gno_activity`) gated on tx-indexer availability. Open-ended writes are explicitly deferred; the meta-issue routes agent writes through a2a (typed task interface) and x402 (payment-gated typed task), not through general-purpose `gno_call`.

v2's session-authorization model (separate ADR) introduces new requirements: tools to expose session state (`gno_auth_status`, `gno_session_list`) and tools to drive the OAuth-style authorization flow (`gno_session_propose`, `gno_session_revoke_propose`).

## Decision

v2 Phase 1 ships 13 tools maximum, registered conditionally based on configuration:

| Tool | Category | Output | Backend | Registered when |
|---|---|---|---|---|
| `gno_render` | read | MCP `resource` (untrusted realm markdown) | `vm/qrender` via gnoclient | always |
| `gno_eval` | read | text (typed value) | `vm/qeval` | always |
| `gno_read` | read | MCP `resource` (untrusted realm source) | `vm/qfile` | always |
| `gno_inspect` | read | text (typed godoc + signatures) | `vm/qdoc` | always |
| `gno_list` | read | text (typed listing) | tx-indexer GraphQL | any profile has `tx-indexer-url` |
| `gno_history` | read | text (typed deploy/tx log) | tx-indexer GraphQL | any profile has `tx-indexer-url` |
| `gno_activity` | read | text (typed `MsgCall` / `MsgRun` log) | tx-indexer GraphQL | any profile has `tx-indexer-url` |
| `gno_auth_status` | read (session state) | text | local + ABCI query | always |
| `gno_session_list` | read (session state) | text | ABCI query | always |
| `gno_call` | write | text (sim/broadcast result, session-signed) | gnoclient with session signer | any profile has `allow-dangerous-tools = true` |
| `gno_run` | write | text (sim/broadcast result, session-signed) | gnoclient with session signer | any profile has `allow-dangerous-tools = true` |
| `gno_session_propose` | write-prep | text (auth payload for user to sign) | local keypair gen + scope clamp | any profile has `allow-dangerous-tools = true` |
| `gno_session_revoke_propose` | write-prep | text (revoke payload) | local | any profile has `allow-dangerous-tools = true` |

**Output-kind classification** (design-time T8): realm-authored byte streams (`gno_render`, `gno_read`) emit as MCP `resources` with `audience: ["assistant"]`. Typed metadata, computed values, indexer query results, and operation outcomes emit as tool result text.

**Cold-start tool count by configuration:**

- Default (no indexer, no dangerous): 6 tools
- + indexer in any profile: 9 tools
- + dangerous in any profile: 13 tools

This stays within mcp-creator's 1–15 sweet spot for context efficiency.

**Dropped from v1:**

| Dropped tool | Replacement / rationale |
|---|---|
| `gno_audit_tail` | Audit log is exposed via `gnomcp audit tail` CLI subcommand. The model has no use case for reading its own audit trail. |
| `gno_network_info` | Chain metadata is in the MCP `initialize` server-info field. |
| `gno_address_info` | No v2 use case identified; can return if a concrete need surfaces. |
| `gno_config_get`, `gno_config_set` | Configuration is CLI flags and `profiles.toml`, not MCP tools. |
| `gno_keygen` | Sessions remove the need: master keys come from external `gnokey`; session keys are gnomcp-internal and never need a tool to be generated. |
| `gno_faucet_request` | Testnet faucet funding targets the user's master address, which gnomcp does not have. The user funds their master via their own wallet directly. |
| `gno_session_create`, `gno_session_revoke` (v1 stubs) | Replaced by `gno_session_propose` and `gno_session_revoke_propose` which emit payloads for the user to sign rather than broadcasting via master-key-in-MCP. |

**Deferred to follow-up ADRs (not in Phase 1):**

- a2a tools (default agent-write path).
- x402 tools (payment-gated typed writes).
- T4 dev tools (`gno_test`, `gno_scaffold`, `gno_devnode_*`).
- Trust/safety middleware (Unicode sanitization, provenance wrap, TOFU pinning).

## Alternatives considered

**Maintain v1's surface for migration compatibility.** Rejected. v2 is a restructure on a fresh branch; the meta-issue defines the canonical surface. Carrying v1 tools without v2 justification trades context budget for backward-compat that no consumer has requested.

**Include a2a and x402 tools in Phase 1.** Rejected. a2a and x402 are substantial design surfaces (typed task interfaces, payment flows, agent-card discovery, HTTP transport for a2a clients). Each warrants its own design pass. Sessions are the shared substrate; the Phase 1 work establishes that substrate.

**Register `gno_session_propose` and `gno_session_revoke_propose` always-on**, independent of `allow-dangerous-tools`. Rejected. There is no use case for proposing sessions when no profile enables writes; session-prep tools share the gate with the writes they prepare for.

**Combine `gno_session_propose` and `gno_session_revoke_propose` into a single tool with an `action` enum.** Rejected. The two operations have different argument shapes (propose requires `allow_paths` and scope fields; revoke takes a session address). Splitting them yields cleaner schemas and clearer tool selection.

**Expose `gno_keygen` for users without a local `gnokey` install.** Rejected. The session-authorization model assumes `gnokey` (or wallet) on the user's host as the master signer. A user without `gnokey` cannot authorize sessions regardless of whether the MCP can generate keypairs. The right answer is "install gnokey", not "make gnomcp generate master keys."

## Consequences

- v1 users do not get a 1:1 upgrade; v2 is a restructure with renamed and dropped tools. Migration is via re-onboarding to the v2 model, not in-place upgrade.
- Open-ended writes (`gno_call`, `gno_run`) remain opt-in only and are expected to be deprecated when a2a/x402 reach feature parity for agent workflows. Their continued existence is for human-developer use through Claude Code.
- Cold-start surface (6 tools default) is materially smaller than v1's 16, reducing context budget consumption.
- Tool descriptions can be tighter because each registered tool has a clear purpose; no "context bloat" tools left in by default.
- Adding a2a or x402 tools later requires its own ADR and design pass. The base architecture (registration pipeline, profile resolution, output-kind classification) accommodates them without restructuring.
