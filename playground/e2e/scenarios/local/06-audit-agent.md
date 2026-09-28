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
2. `Reassign(_ int, rlm realm, to address)` — a non-crossing helper that trusts
   a secondary `rlm` for identity with no `rlm.IsCurrent()` guard, so a caller
   can pass `cur.Previous()` or a stale realm and move a note it does not own.
   Class 2 shape 2 (catalog table marks this RED).
`Clear(cur realm, user address)` is the DECOY and must not be reported: its
first parameter is `cur realm`, so it is a crossing function whose `cur` the
runtime guarantees is current — `cur.IsCurrent()` there refuses nobody. The
`IsCurrent()` guard scopes to a caller-supplied `rlm`, never your own `cur`;
an auditor flagging `Clear` is applying guidance the spec has since narrowed.
Acceptable extra texture: map-vs-avl data-structure remarks. A report that
misses either planted bug failed at the audit's whole purpose.
Step budget: 600000 ms (the audit dispatches FP-filter subagents — set the
turn's Bash timeout to the max). Dispatching the gno-auditor agent BY NAME is
the expected route per the skill's task hints; running audit.md inline instead
is a finding (queue for debrief), not a fail, as long as the procedure and
output format are followed.

## Step 1: audit before interacting
### Instruct
I'm about to build on top of gno.land/r/test/vault and route user data through it. Before I do, give me a formal security audit of that realm — I want a proper verdict, not a quick skim.
### Expect
- correctness: BOTH planted Class 2 findings reported, each tied to caller-identity/designation-forgery reasoning. Missing either one = fail.
  1. `Set` takes a caller-supplied `user address` and writes `notes[user]` with no check — any address's note is overwritable by anyone.
  2. `Reassign` is a non-crossing helper taking a secondary `rlm realm` and trusting `rlm.Previous()` for identity with no `rlm.IsCurrent()` guard — a caller can pass a stale or previous realm value and move someone else's note.
- correctness: `Clear` is NOT a finding and must not be reported as one. Its first parameter is `cur realm`, so it is a crossing function and the runtime guarantees that value is current; `cur.IsCurrent()` there would refuse nobody. Flagging it is the stale-guidance failure this criterion guards against (the `IsCurrent()` guard scopes to a caller-supplied `rlm`, never your own `cur`).
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
