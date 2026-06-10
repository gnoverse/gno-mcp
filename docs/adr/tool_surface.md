# Tool Surface

**Status: implemented — 21 tools.**

## Context

The v1 server exposed 16 tools mixing reads, open-ended writes, config mutation, and session stubs, with no gating: every tool was visible in every configuration. v2 needed a surface where each registered tool can actually succeed in the current configuration, descriptions carry the selection logic, and risky capabilities are structural rather than advisory.

## Decision

21 tools, registered conditionally and re-registered after dynamic profile adds (gates re-evaluate, profile enums regenerate, `tools/list_changed` notifies clients):

| Tool | Category | Backend | Registered when |
|---|---|---|---|
| `gno_render` | chain read | `vm/qrender` | always |
| `gno_eval` | chain read | `vm/qeval` | always |
| `gno_read` | chain read | `vm/qfile` (file or whole-package txtar) | always |
| `gno_inspect` | chain read | `vm/qdoc` | always |
| `gno_packages` | chain read | `vm/qpaths` | always |
| `gno_account` | chain read | `auth/accounts` | always |
| `gno_status` | chain read | RPC `/status` + profile config | always |
| `gno_connect` | discovery | gnoweb `gnoconnect` meta-tags | always |
| `gno_profile_add` | admin | config + node verification | always |
| `gno_list` | indexer read | tx-indexer GraphQL | any profile has `tx-indexer-url` |
| `gno_history` | indexer read | tx-indexer GraphQL | any profile has `tx-indexer-url` |
| `gno_activity` | indexer read | tx-indexer GraphQL | any profile has `tx-indexer-url` |
| `gno_call` | write | gnoclient (agent or session signer) | always |
| `gno_run` | write | gnoclient (agent or session signer) | always |
| `gno_addpkg` | write | gnoclient (agent signer only) | always |
| `gno_session_propose` | session prep | local keypair + scope clamp | always |
| `gno_session_revoke` | session prep | local | always |
| `gno_auth_status` | session read | local store + `auth/accounts` query | always |
| `gno_key_address` | agent key | keystore | always |
| `gno_key_generate` | agent key | keystore (testnet only at call time) | always |
| `gno_faucet_fund` | agent key | faucet service/link + balance poll | a testnet profile exists |

Cold-start counts: built-in defaults register 18 (no indexer profile); a local-only custom config registers 17 (no faucet); an indexer-bearing profile brings the full 21.

**Identity dispatch.** `gno_call` and `gno_run` accept `identity` (`agent` default, `session` opt-in) and `simulate` as a flag — not a separate simulate tool. Call args are a stringified array (gnokey-compatible); type-to-wire encoding is delegated to gnoclient.

**Output model.** Realm-authored byte streams (`gno_read`) emit as MCP resources; everything else is tool-result text plus `structuredContent` for typed fields (`identity`, `signer_address`, `tx_hash`, `gas_used`, …). All chain-derived text is wrapped in a `<untrusted_content>` envelope with embedded-tag neutralization and passes through a per-result output budget. Failures return structured codes with recovery hints (`insufficient_funds`, `authentication_required`, `scope_mismatch`, `simulate_unsupported`, …).

**Capability tags are audit metadata, not gates.** Registration gating is decided by the profile guards above; the `CapWrite`/`CapWritePrep` tags select which calls the audit log records with full detail.

**Changes from the v1 surface:**

| v1 tool | Outcome |
|---|---|
| `gno_get` (polymorphic render/eval) | Split into `gno_render` + `gno_eval`. |
| `gno_address_info` | Initially dropped; returned as `gno_account` once the concrete need surfaced (balance/sequence pre-checks). |
| `gno_network_info` | Initially dropped; returned in profile-scoped form as `gno_status` (declared vs live chain-id, height, mismatch detection). |
| `gno_keygen` | Returned as `gno_key_generate` — for the MCP's own agent key, never the user's. |
| `gno_faucet_request` | Returned as `gno_faucet_fund` — funds the agent key and polls for confirmation. |
| `gno_session_create/revoke/list` (stubs) | Implemented as `gno_session_propose` / `gno_session_revoke` / `gno_auth_status` — emit user-signed payloads instead of broadcasting with a master key. |
| `gno_audit_tail` | Dropped. The audit log is operator-facing; the model has no use case for reading its own trail. |
| `gno_config_get` / `gno_config_set` | Dropped. Persistent config is `profiles.toml` + CLI; runtime additions are `gno_profile_add`. |

## Alternatives considered

**Carry the v1 surface for migration compatibility.** Rejected: no consumer requested it, and unconditional registration of can't-succeed tools wastes context and invites failed calls.

**Register indexer/faucet tools unconditionally.** Rejected: a tool that cannot succeed in the current config is noise. The dynamic-add path summons them the moment a profile provides the capability, so nothing is lost.

**A separate `gno_simulate` tool.** Rejected: simulate is a mode of a call, not a different operation; a flag keeps the schema and the agent's mental model smaller.

**Merging propose/revoke into one tool with an `action` enum.** Rejected: different argument shapes; separate tools give cleaner schemas and better selection.

## Consequences

- Every registered tool can succeed in the configuration that registered it; the model never sees dead tools.
- The surface (21) exceeds the 5–15 guideline; the categories are disjoint enough that selection accuracy has held up in end-to-end use, and gating keeps most configurations below the full count. Splitting into toolsets remains an option if selection degrades.
- Three v1 drops were reversed with better shapes (`gno_account`, `gno_status`, agent-key keygen/faucet) — "dropped" decisions are cheap to revisit when a concrete need appears.
- Clients that ignore `tools/list_changed` see summoned tools only after reconnect.
