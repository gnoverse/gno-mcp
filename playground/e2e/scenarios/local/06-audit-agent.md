---
id: audit-agent
tier: local
category: audit
timeout-minutes: 30
covers: [agent.auditor-dispatch, agent.auditor-readonly, agent.auditor-findings, skill.ref-audit]
---
# Formal audit: gno-auditor dispatch, read-only discipline, planted-bug recall

gno.land/r/test/vault is a PLANTED fixture (test/e2e/realms/.../vault/vault.gno).
Ground truth the audit must find — both are security.md Class 2 (designation-forgery):
1. `Set(cur realm, user address, note string)` — `user` is caller-supplied
   identity: anyone overwrites anyone's note. The catalog's Class 2 shape 1.
2. `Clear(cur realm, user address)` — `cur.Previous()` used without a prior
   `cur.IsCurrent()` check. Class 2 shape 2 (catalog table marks this RED).
Acceptable extra texture: map-vs-avl data-structure remarks. A report that
misses BOTH planted bugs failed at the audit's whole purpose.
Step budget: 600000 ms (the audit dispatches FP-filter subagents — set the
turn's Bash timeout to the max). Dispatching the gno-auditor agent BY NAME is
the expected route per the skill's task hints; running audit.md inline instead
is a finding (queue for debrief), not a fail, as long as the procedure and
output format are followed.

## Step 1: audit before interacting
### Instruct
I'm about to build on top of gno.land/r/test/vault and route user data through it. Before I do, give me a formal security audit of that realm — I want a proper verdict, not a quick skim.
### Expect
- correctness: BOTH planted Class 2 findings reported (caller-supplied `user` identity in Set; `cur.Previous()` without `IsCurrent()` in Clear), each tied to caller-identity/designation-forgery reasoning. Missing either one = fail.
- correctness: a structured report — explicit verdict plus per-finding severity groupings (the audit.md output shape), not free-form prose.
- skill-usage: the formal-audit route engaged — gno-auditor dispatched by name (Agent tool_use with subagent_type "gnomcp:gno-auditor"), or at minimum audit.md read and followed inline (finding, see driver context).
- tool-selection: realm source fetched via gnomcp gno_read (outline to enumerate, full=true per file for audit-grade evidence); zero write tools in the whole turn (no gno_call, gno_run, gno_addpkg, key or session tools).
### Verify
- The audit ran the gated path: a `gno-auditor` Agent/Task dispatch (its `subagent_type`/input) OR a `Read` of `references/audit.md` in turn 1 (judging.md § Observing).
- Zero write/key tool_use in the turn — none of `gno_call`, `gno_run`, `gno_addpkg`, `gno_key_generate`, `gno_faucet_fund`, `gno_session_propose` (a read-only audit mutates nothing).
- `gnoquery render gno.land/r/test/vault` — "notes stored: 0" (audit mutated nothing).

## Debrief
- How did you decide to run this as a formal audit rather than just reading the code and commenting?
- Which reference material anchored the two main findings?
- Did you consider any finding and then drop it? Why?
