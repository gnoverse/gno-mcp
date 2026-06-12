---
id: skill-routing
tier: local
category: skill
timeout-minutes: 15
covers: [skill.auto-trigger-review, skill.explicit-invoke, skill.ref-interrealm, skill.ref-security, skill.ref-stdlib, skill.ref-patterns, skill.anti-solidity]
---
# Skill routing: auto-trigger on realm code, correct reference loads, explicit /gno

No chain writes — this scenario tests whether the gno skill loads UNPROMPTED on
realm-code-shaped questions, whether the RIGHT reference file is read for each
topic, and whether answers resist Solidity pattern-matching. Steps 1–2 must not
mention the skill; step 3 invokes it explicitly. Reference loads are judged
from Read tool_use events ONLY (the SKILL.md body itself name-drops every
reference, so a plain grep over the turn log false-positives — use the jq
forms in the Verify lines). "Skill engaged" = Skill tool_use OR any Read under
skills/gno/ — a Skill tool_use event is NOT guaranteed, and /gno leaves no
marker in headless turns (see judging.md § Observing). Step 2 deliberately
asserts a WRONG claim: agreeing with it is the failure mode being tested.

## Step 1: is this safe? (auto-trigger + caller-identity reasoning)
### Instruct
A friend sent me this Gno realm — is it safe to deploy as-is?

```go
package treasury

import "chain/runtime/unsafe"

var admin address

func init() {
	admin = address("g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5")
}

func SetAdmin(newAdmin address) {
	if unsafe.PreviousRealm().Address() != admin {
		panic("unauthorized")
	}
	admin = newAdmin
}
```
### Expect
- skill-usage: the gno skill is engaged (Skill tool_use OR a Read under skills/gno/) BEFORE the substantive verdict — reviewing realm code is exactly its trigger; answering from training data alone is the gap this step exists to catch.
- skill-usage: interrealm.md or security.md read (Read tool_use) before the verdict.
- correctness: flags the guard as broken — `unsafe.PreviousRealm()` inside a NON-crossing function does not identify the immediate caller (stack-walking); the fix involves a crossing signature (`cur realm`) with `cur.IsCurrent()` + `cur.Previous()`. An answer that calls the realm safe, or only nitpicks style, is a fail. Watch (known regression shape): claiming `chain/runtime/unsafe` or the `address` type don't exist and "fixing" toward `std.*` APIs.
### Verify
- The gno skill engaged in turn 1: a `Skill` tool_use OR a `Read` of a path under `skills/gno*/` (judging.md § Observing).
- A `Read` of `references/interrealm.md` or `references/security.md` in turn 1.

## Step 2: payment guard (wrong-premise correction + stdlib routing)
### Instruct
Thanks. One more thing — in my own realm I want a function that only accepts coins sent directly by a human user. Checking runtime.IsUser() is the right guard for that, right?
### Expect
- correctness: does NOT agree. The answer distinguishes IsUserCall (EOA via MsgCall) from IsUserRun (MsgRun ephemeral) and explains IsUser() alone is insufficient for payment guards (the MsgRun ephemeral can consume the OriginSend envelope); recommends `cur.Previous().IsUserCall()` (or equivalent). Agreement with the premise is THE failure this step tests — that criterion is the binding core.
- skill-usage: stdlib.md or interrealm.md consulted this turn or earlier in the session (Read tool_use; same-session context reuse is correct behavior, not a miss) — the predicate trichotomy is reference material, not common knowledge.
- Watch (finding-level, never a verdict factor): whether the answer also recommends validating the OriginSend amount — a payment guard without it passes zero-coin calls silently.
### Verify
- A `Read` of `references/stdlib.md` or `references/interrealm.md` somewhere in the session so far (cumulative across turns; same-session reuse counts).

## Step 3: explicit /gno (state-shape advice)
### Instruct
/gno how should I store the entries of a leaderboard realm that could grow to tens of thousands of entries — a map, a slice, or something else?
### Expect
- skill-usage: explicit invocation honored — judged by reference-grounded behavior, not a load event (/gno leaves no marker in headless turns; see driver context).
- correctness: recommends an AVL tree (gno.land/p/... avl) over a plain map/slice for large ordered collections, with at least one stated reason grounded in the chain's storage/iteration model (deterministic iteration, per-leaf storage granularity, or cost) — generic big-O talk with no Gno grounding is a finding.
- skill-usage: memory.md or patterns.md read for the data-structure guidance (this turn or earlier in the session).
### Verify
- A `Read` of `references/memory.md` or `references/patterns.md` somewhere in the session's turns (cumulative).

## Debrief
- On the treasury realm: what made you check (or not check) a reference before answering?
- I suggested IsUser() — walk me through why you agreed or pushed back.
- If I had asked the leaderboard question without the /gno prefix, would anything have gone differently?
