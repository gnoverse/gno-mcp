# Status + next-iteration plan

Date: 2026-05-15 (end of session, post audience-clarification)
Branch: `feat/gnomcp-skills`

## Skill audience — clarified this session

**This skill is for `/r/` + `/p/` builders, not for the gno team / chain investigators.**

Builders write realms and packages. They need to understand the chain's behavior **from their code's perspective**: what's deterministic, what costs gas, what reverts on panic, what crosses realm boundaries. They do NOT need to debug consensus halts, navigate `gnovm/pkg/` internals, or interpret WALs.

This distinction reshapes what content belongs in the skill:
- **In scope**: behavioral consequences of the runtime, idioms that fit the constraints, bug classes that affect deployed realms, audit signals, canonical primitives to import.
- **Out of scope**: internal file paths into `gnovm/`, `tm2/`, `gno.land/pkg/`; MCP debugging tools (`node_compare`, `realm_eval`, etc.); consensus diagnostic sequences; WAL analysis; cache cold/warm framing.

## Where we are

```
b1cf833c feat: scaffold gno skill with categorical references
e5ecece3 refactor: /p/ audit lens + operational signals from RED phases
f8ca36bb test: RED-phase test corpus and findings
655af735 docs: blockchain-context drivers in patterns + initial status report
[uncommitted] builder-extractions + stdlib.md MCP-introspection framing + render.md de-investigatorized + this status update
```

Three RED-phase iterations completed with empirical evidence of where the skill helps and where it doesn't (RED-01 / RED-02 / RED-03-findings.md in this directory).

## What's filled

| Reference | Lines | State |
|---|---|---|
| `SKILL.md` | 81 | done — router + quick reference + red flags |
| `interrealm.md` | 181 | done — two-contexts model, crossing, PreviousRealm |
| `security.md` | 353 | done — 10 bug classes + audit signals (now incl. non-determinism + refcount + `panic`-reverts) + operational + /p/ audit lens |
| `patterns.md` | 237 | done — idioms + 7 blockchain-context drivers + canonical-imports table |
| `render.md` | 196 | done — Render() contract + 6 extensions (file-path framings dropped, behavior-only) |
| `stdlib.md` | 70 | done — **thin by design**: routes to `gno_inspect` for live API surface; static restatement would go stale |
| `future.md` | 73 | rolling — 5 in-flight entries with migrate-on-merge rule |

All six categorical references are end-to-end content. The skill is iteration-ready, not deployment-ready (still needs more RED phases against varied realms).

## Backup survey — earlier proposal pulled back

I had earlier proposed three new internals references (`gnovm.md` / `tm2.md` / `gnoland.md`) sourced from the backup drafts at `~/Backup/.../gnolang/gno/.claude/agents/`. **Pulled back.** Those drafts are written for chain investigators: they list MCP debugging tools, internal file paths, WAL analysis, consensus halt sequences. None of that is what a realm builder needs.

The codebase-map.md proposal is also pulled. A builder edits their own realm, not the gno repo; they don't need a navigation map into `gnovm/pkg/`.

## Builder-relevant extractions from the backup (DONE this session)

Four specific facts were extracted from the investigator-shaped drafts and woven into existing references:

| Concept | Source | Landed in |
|---|---|---|
| Non-determinism sources (map iteration, `math.MinInt`/`MaxInt`, `append` capacity observation, float formatting) | gnovm-expert.md | `security.md` operational signals — each as a RED/YELLOW grep row |
| `panic`-reverts-state guarantee vs error-return-does-not | gnoland-expert.md | `patterns.md` § Error vs panic — explicit state-revert paragraph |
| Delete-then-recreate refcount footgun | gnovm-expert.md | `security.md` operational signals — YELLOW grep row |
| Canonical-imports table (grc20, AVL, mux, pagination, ownable, commondao, authz) | implicit in codebase-map + observation | `patterns.md` § Imports — expanded as a table with notes |

No new files needed; the existing references absorbed the additions naturally.

## Stdlib live-introspection plan (per your direction)

`stdlib.md` is now framed as a **thin pointer to live introspection** rather than a static API restatement. APIs change; static refs go stale; an agent emitting stale signatures produces broken code. The gnomcp design exposes `gno_inspect <pkgPath>` (queries `vm/qdoc` against the current chain). The reference names which packages a builder will encounter and what's security-relevant; for exact signatures, the agent queries live.

## Unresolved items (next iteration)

1. **§8 (slice readonly-taint) fixture** — the only `OPEN` bug class still untested. Worth building a small fixture once that bug class is hit in real code.
2. **`/r/` realm test** — all RED tests so far have been synthetic fixtures or `/p/` libraries. A real deployed `/r/` realm (boards2? gnoland/blog?) would test the skill on realm-context findings (state mutation, payment, render path routing).
3. **Default-load reference set for audit mode** — currently the agent decides per-finding which refs to load. A hint in SKILL.md ("for audits, load security.md + patterns.md + interrealm.md") might shave latency on multi-class audits.
4. **Skill packaging** — eventually the skill needs to be packaged into something a user/builder can install (mcpb? plain directory? per the meta-issue T4 distribution discussion).

## What is explicitly NOT planned (audience-clarified)

- New `gnovm.md` / `tm2.md` / `gnoland.md` internals references.
- A codebase-navigation map (`codebase-map.md`).
- Importing the backup agent definitions (`gnovm-expert.md` / `tm2-expert.md` / `gnoland-expert.md`) wholesale or as sub-agents.
- Static restatement of stdlib API surface (use live introspection instead).

## On the wider gnomcp scaffold

The skill is one half of the meta-issue T1 + T2 + T5 + T6 work. The other half — the MCP server binary at `contribs/gnomcp/cmd/gnomcp/` with `gno_render`, `gno_eval`, `gno_read`, `gno_inspect` tools — is not yet scaffolded. That's the next major piece when you're ready. The skill we just built is what the MCP-server's `gno_inspect` etc. will hand off to.
