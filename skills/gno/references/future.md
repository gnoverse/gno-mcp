# Future / in-flight changes

> **Category: pending merge.** Single home for unmerged work: open PRs, RFC issues, agreed-but-not-shipped doctrine.
> **Maintenance rule:** when an entry merges, its content migrates to the appropriate categorical reference (`interrealm.md` / `security.md` / `patterns.md` / `stdlib.md`) and the entry here is deleted. Don't let `future.md` accumulate merged history.

## Purpose

Until a change is in master, treat it as guidance, not doctrine. This file is where agents look when a user asks "what about X — is that landing?" or when emitting code that should NOT use a pattern that exists only on a branch.

## Entry format

Each entry:

```
### <short-name> — <PR# / issue# / RFC>, status as of <YYYY-MM-DD>

**Status**: open / draft / approved / WIP / merged-pending-migration
**One-line summary**.
**Today's posture**: what to generate / accept *until* this lands.
**Post-merge posture**: what changes once it lands.
**Maintenance**: which categorical file absorbs this entry on merge.
```

## Initial entries (scaffold)

### cross2(rlm) explicit caller form — PR #5669, WIP

- **Status**: open WIP by jaekwon, 309 files, +6140/-2738, 56 commits. Empty PR body. 0 reviews at scaffold-time.
- **Summary**: introduces `cross2(rlm)` as the explicit form of bare `cross`. `cross2(cur)` ≡ bare `cross`; `cross2(otherRealm)` permits explicit target realm.
- **Today's posture**: generate bare `cross`. Flag `cross2(...)` as won't-compile.
- **Post-merge posture**: bare `cross` in security-sensitive call sites becomes YELLOW (review against explicit-target doctrine). `cross2(rlm)` enters `interrealm.md` as canonical.
- **Maintenance**: on merge, fold into `interrealm.md` §"crossing-function calls".

### realm.SentCoins() — PR #5039, merged 2026-04-16, adoption pending

- **Status**: merged, but `examples/` not yet swept; canonical `OriginSend()` still in use.
- **Summary**: frame-local coin-receipt query; re-entrancy-safe successor to `banker.OriginSend()`.
- **Today's posture**: `OriginSend() + IsUserCall()` remains canonical and safe.
- **Post-adoption posture**: `SentCoins()` becomes canonical; `OriginSend()` becomes YELLOW.
- **Maintenance**: on adoption sweep (or independent decision to flip), fold into `stdlib.md` §"std/banker" and `security.md` §"payment-guard pattern".

### daokit framework upgrade — PR #4884, open

- **Status**: open, 2 MEMBER approvals, not yet merged at scaffold-time.
- **Summary**: replaces `examples/gno.land/p/samcrew/daokit/*` with upstream-mirrored upgradeable version.
- **Today's posture**: new realms should import `gno.land/p/nt/commondao` for new DAO work, not `p/samcrew/daokit` directly. The pre-#4884 daokit is the source of several known bug classes.
- **Post-merge posture**: daokit becomes safe to import as `p/samcrew/daokit`.
- **Maintenance**: on merge, fold into `patterns.md` §"imports" (DAO recommendations) and remove §"daokit re-entrancy" entries from `security.md` if applicable.

### `<gno-form exec>` wire format — PRs #4858/#4974/#4978/#4979/#5018/#5046, Adena #5002 in flight

- **Status**: ext_forms in master, but the on-the-wire submission format is "modifiable" per gfanton's own comment. 6 patching PRs late-2025 through 2026-Q1. Adena wallet integration tracked at #5002, in flight.
- **Summary**: `<gno-form exec="r/myrealm/Func">` is currently the only Markdown → tx primitive. Users see exactly what's submitted (hidden fields explicitly rejected per PR #4858 transparency principle).
- **Today's posture**: Treat `<gno-form exec>` as **experimental**. Authoring realms that depend on its exact HTML output for downstream tooling is risky. Mark prominently in `Render()` if used. Don't try to slip data via hidden fields — the renderer strips them and the design intent is full visibility.
- **Post-stability posture**: Folds into `render.md` §"Markdown extension surface" with the canonical wire format pinned.
- **Maintenance**: on stable wire format, fold into `render.md`; remove this entry.

### Upgradeable realm patterns — PR #4816, draft

- **Status**: open draft, leohhhn exploring.
- **Summary**: documents Proxy + Implementation upgrade pattern in `r/docs`.
- **Today's posture**: prefer single top-level state struct so future Proxy upgrade can serialize cleanly. `lplist` (linked-pointer list) is the recommended substrate.
- **Post-merge posture**: full pattern available; reference becomes canonical.
- **Maintenance**: on merge, fold into `patterns.md` §"state shape".

## Cross-references

- All categorical references — entries here migrate into them on merge.

## Source (internal)

`.mynote/gno-agentic/reference/17-pr5669-and-security-comments.md` (PR #5669).
`.mynote/gno-agentic/reference/15-security-evolution-interrealm.md` §3 (active migrations in flight).
