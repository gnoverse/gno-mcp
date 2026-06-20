# e2e feature coverage

The contract between the product surface (gnomcp tools + `gno` skill + agents +
playground flows) and the scenarios that assert it. Scenarios declare what they
assert via `covers:` frontmatter using the keys below; this file is the ledger
of what is covered, what is not, and why. The skill/MCP improvement loop runs
against this: change a description or a reference → rerun the categories whose
features it touches.

Verdict semantics per feature: **covered** = a scenario's Expect makes it
binding; **watch** = a scenario observes it and queues findings, but it can't
fail a step; **gap** = no scenario exercises it (reason + unblock condition
listed).

## MCP read tools

| Key | Feature | Scenarios | Status |
|---|---|---|---|
| read.render | gno_render realm output | 01, 03 | covered |
| read.read-package | gno_read whole-package txtar (not file-by-file) | 01 | covered |
| read.outline | gno_read default outline (API surface, bodies elided) | 01 | covered |
| read.eval | gno_eval expression | 01 | covered |
| read.packages | gno_packages namespace listing (vm/qpaths) | 04 | covered |
| read.account | gno_account balance/sequence/exists:false-is-normal | 04 | covered (watch in 02 funding flow) |
| read.status | gno_status declared vs live chain, tip freshness | 04 | covered |
| read.faucet-limits | gno_status surfaces the faucet's per-address policy (grant size + per-address cap) for a profile with a faucet service | 10 | covered |
| read.connect-preview | gno_connect preview without adding a profile | 04 | covered |
| read.error-quality | nonexistent realm → clean error, no hallucination | 04 | covered |

## Indexer tools (tx-indexer required)

| Key | Feature | Scenarios | Status |
|---|---|---|---|
| idx.list | gno_list catalog search | — | gap: simnet runs no tx-indexer; requires one in the e2e image |
| idx.history | gno_history deploy/tx log | — | gap (same) |
| idx.activity | gno_activity MsgCall/MsgRun log | — | gap (same) |

## Write tools (agent identity)

| Key | Feature | Scenarios | Status |
|---|---|---|---|
| write.key-generate | gno_key_generate per-profile agent key | 02 | covered |
| write.key-address | gno_key_address | 02 | covered (alt path) |
| write.faucet-fund | gno_faucet_fund tier-2 automatic funding | 02 | covered |
| write.funds-recovery | insufficient_funds error → faucet → retry, unprompted | 02 | covered |
| write.faucet-per-address-recovery | per-address faucet 429 → agent reads it as per-address (fresh key recovers), not a global outage | 10 | covered (e2e-faucetcap image: per-address cap = 1) |
| write.call | gno_call broadcast | 02 | covered |
| write.simulate | gas estimate without broadcast | 02 | covered |
| write.addpkg | gno_addpkg deploy | 02 | covered |
| write.deploy-gates | deploy clears the genesis-activated namespace + CLA gates (own-address namespace + sign r/sys/cla); the gno_addpkg CLA hint guides recovery on the unsigned-CLA error | 14 | covered (live-only; simnet has the gates off) |
| write.run | gno_run MsgRun script | 08 (deferred) | deferred with scenario 08 |
| write.signer-reporting | answer names who signed (agent vs test1 vs session) | 02, 07, 09 | covered |
| write.key-multi | multiple named agent keys per profile (cap GNOMCP_AGENT_MAX_KEYS) | 09 | covered |
| write.key-selector | `key` arg selects which named key signs a write | 09 | covered |
| write.key-send | gno_key_send moves ugnot between a profile's own keys | 09 | covered |
| write.key-list | gno_key_list enumerates {name, address} | 09 | covered |
| write.key-delete | gno_key_delete removes a key; refuses a funded key (key_has_funds) unless swept or force | 09 | covered |

## Sessions (write-as-user)

| Key | Feature | Scenarios | Status |
|---|---|---|---|
| session.no-master-error | read-only profile → no_master_address error with repair instructions | 07 | covered |
| session.propose | gno_session_propose prints the gnokey authorize command, no broadcast | 07 | covered |
| session.auth-status | gno_auth_status reflects session state | 07 | covered |
| session.authorize | user authorizes via gnokey; write lands as session | 07 | covered (the driver authorizes as the user with gnokey; the AUT never touches gnokey) |
| session.revoke | gno_session_revoke an active session | 07 | covered |
| write.call-as-session | a write signed by the session lands, Caller = master | 07 | covered |

## Profiles & connect

| Key | Feature | Scenarios | Status |
|---|---|---|---|
| admin.profile-add-discovery | gno_profile_add from gnoweb_url (gnoconnect meta-tags) | 03 | covered |
| admin.profile-add-verify | live chain-id cross-check on add | 03 | covered |
| admin.profile-add-explicit | rpc_url + chain_id form | — | gap: needs a chain not already profiled; fits the external tier |
| admin.persist-hint | result volunteers the CLI persist command | 03 | covered |
| profile.selection | right profile chosen without prompting | 01 | covered |
| profile.read-attribution | reads name the profile they used | 03 | covered |
| misc.gnoweb-metadata | gnoconnect meta-tags drive discovery | 03 | covered |
| misc.chain-allowlist | betanet/mainnet chain-ids refused | — | gap: needs a node reporting a mainnet id; not worth faking locally |
| misc.output-budget | truncation is explicit, never silent | — | gap: needs a deterministic huge-output fixture realm |

## gno skill family (routing + triggering)

The gno skill is a family: `gno` (root references) + siblings `gno-build` (authoring),
`gno-audit`, `gno-debug`, `gno-onboard`. Non-overlapping descriptions route each task to its
owner; siblings load the gno references themselves.

| Key | Feature | Scenarios | Status |
|---|---|---|---|
| skill.gno-build-trigger | gno-build engages before authoring/deploying a realm, unprompted | 02 | covered |
| skill.auto-trigger-review | gno/gno-audit engages on "is this realm safe?", unprompted | 05 | covered |
| skill.explicit-invoke | `/gno <question>` direct invocation | 05 | covered |
| skill.ref-interrealm | interrealm.md loaded for caller-identity reasoning | 05 | covered |
| skill.ref-security | security.md loaded for review/bug-class lookup | 05 | covered |
| skill.ref-stdlib | stdlib.md loaded for chain API questions | 05 | covered |
| skill.ref-patterns | patterns.md loaded for authoring idioms | 05 | covered |
| skill.ref-audit | audit.md procedure behind formal audits | 06 | covered |
| skill.anti-solidity | no msg.sender pattern-matching in answers | 05 | covered |
| skill.ref-build | build.md loaded by gno-build for authoring/test/deploy | 02 | covered |
| skill.ref-render | render.md for Render()/gnoweb authoring | — | gap: no render-authoring scenario |
| skill.ref-memory | memory.md for data-structure/persistence guidance | 05 | covered |
| skill.ref-mcp | mcp.md "fetch source via gnomcp" | all | covered indirectly by every tool-selection Expect |

Reference-load Expects in scenario 05 are or-bindings per topic (interrealm|security,
stdlib|interrealm, memory|patterns): the step binds on "a correct reference for the
topic", since the skill's own routing table legitimately allows either.

## Agents

| Key | Feature | Scenarios | Status |
|---|---|---|---|
| agent.auditor-dispatch | formal audit → gno-auditor dispatched by name | 06 | covered |
| agent.auditor-readonly | audit performs zero writes | 06 | covered |
| agent.auditor-findings | planted Class 2 bugs found; structured verdict format | 06 | covered |

## Playground local-dev (L3)

Scenario 08 sits in `scenarios/deferred/` — blocked on an upstream gnodev fix
(see `scenarios/deferred/README.md`). Its features are written but unasserted.

| Key | Feature | Scenarios | Status |
|---|---|---|---|
| localdev.gnodev-start | agent starts gnodev itself when local testing is asked for | 08 (deferred) | deferred |
| localdev.local-profile | built-in `local` profile auto-discovers :26657 (chain dev) | 08 (deferred) | deferred |
| localdev.test1-signing | local writes sign with built-in test1 | 08 (deferred) | deferred |

## Distribution (install from scratch)

Scenarios 11 (the README's copy-paste prompt) and 12 (the README's curl|sh
install script) run the `l1-fresh` image (clean Claude Code, no gno tooling)
and need GitHub egress — external tier. They assert the published artifacts
(release archives + marketplace plugin + script on `main`), not the local
tree: a regression shows up here only after it ships.

| Key | Feature | Scenarios | Status |
|---|---|---|---|
| install.binary-release | platform-matched release archive → working binary (`gnomcp version`) | 11 | covered |
| install.plugin-marketplace | plugin marketplace add + install from GitHub; skills land via the plugin, never hand-copied | 11 | covered |
| install.mcp-register | `claude mcp add` (absolute path) → server connects | 11 | covered |
| install.no-stray-server | install leaves no broken plugin-shipped MCP server (repo root must stay `.mcp.json`-free) | 11 | covered |
| install.skills-live | next session exposes the gno skill family + connected gnomcp | 11 | covered |
| install.script-oneliner | scripts/install.sh via curl\|sh: checksum-verified binary + Claude wiring end-to-end | 12 | covered |
| install.script-idempotent | re-running the installer is safe — exit 0, no duplicate server entries | 12 | covered |

## External tier (real testnet)

Scenarios 11–12 (install from scratch, GitHub egress — above) plus 13 (the live
agent-faucet) and 14 (the live deploy gates). Scenarios 13 and 14 drive the real test13
chain: they run the `l2-gnomcp` image (gnomcp + skill, no simnet `profiles.toml` override),
so the built-in `testnet` profile (`internal/profiles/config.go`) resolves to the live
network (chain-id `test-13`, RPC `https://rpc.test13.testnets.gno.land:443`,
faucet-service-url `https://faucet-agent.test13.testnets.gno.land`). 13 validates the
zero-config faucet default; 14 covers what only the live chain exercises — the namespace +
CLA deploy gates, which the simnet leaves off. `blocked` is tolerated when the live faucet
or chain is unreachable or rate-limits.

| Key | Feature | Scenarios | Status |
|---|---|---|---|
| external.faucet-live | gno_faucet_fund tier-2 against the LIVE test13 agent-faucet (validates the built-in faucet-service-url default) | 13 | covered |
| external.testnet-key-cycle | built-in `testnet` profile end to end on the live network: generate agent key → faucet fund → balance | 13 | covered |
| external.cla-sign | agent signs the live test13 CLA (`gno_call r/sys/cla Sign`) from its own key to clear the deploy gate | 14 | covered |

## Known harness constraints (not feature gaps)

- MCP in-memory state resets every headless turn (fresh gnomcp child per
  turn) — flows needing cross-turn dynamic profiles must fit one turn
  (scenario 03), or exploit the reset to pick up profiles.toml edits
  (scenario 07).
- Dynamic profiles (gno_profile_add) have no persistence mechanism; tracked
  as a finding, not a scenario.
