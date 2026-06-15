# Skills

The `skills/` directory is a Claude plugin package. Each subdirectory is a single skill, discovered automatically from `skills/` by the plugin harnesses (Claude Code; the Codex and Cursor manifests point at the directory). Skills are pure markdown — no Go code — and are designed to drive `gnomcp` tools through coherent workflows without dropping the security guardrails.

**Source of truth:** skill content is currently hand-distilled from the [gnolang/gno](https://github.com/gnolang/gno) monorepo and can drift as the language evolves. The direction is to make the monorepo the sole reference and reduce skills to thin wrappers — routing, guidance, and best practice layered on monorepo knowledge, not a copy of it. When writing skill content, prefer pointing at monorepo sources over restating them.

## Bundled skills

| Skill | Trigger |
|---|---|
| [`gno`](../skills/gno/) | Reading, reviewing, and reasoning about existing Gno code: interrealm semantics, on-chain payments, `Render()` output, the memory and data model. The reference library every side skill loads. Defers building to `gno-build`, audits to `gno-audit`, tx debugging to `gno-debug`, onboarding to `gno-onboard`. |
| [`gno-build`](../skills/gno-build/) | Authoring: write, test, and deploy a realm or `/p/` package, or scaffold a project — including when the user only describes the behavior they want ("deploy a counter I can bump"). Owns writing Gno source. |
| [`gno-audit`](../skills/gno-audit/) | Explicit security audit of a realm/package ("is this safe", pre-funding review). Thin wrapper over `audit.md` + `security.md`. |
| [`gno-debug`](../skills/gno-debug/) | Failed transaction/call triage. Thin wrapper over `debug.md`. |
| [`gno-onboard`](../skills/gno-onboard/) | First-contact teaching; interview-first, adapts to the user's actual background (no fixed tiers). |

Side skills are thin entry points: each reads the main `gno` skill (source index) plus only
the references it needs. Content lives in `skills/gno/references/` — side skills compose it,
never duplicate it. The `agents/auditor.md` agent reuses the same bricks autonomously.

## Authoring conventions (so future skills stay coherent)

1. **Frontmatter `description`** must be specific enough that the skill router picks it correctly without overlap with sibling skills. Lead with the trigger.
2. **`When to use`** section comes first — concrete user phrasings.
3. **`Guardrails` / `Judgment`** sections call out the rails: untrusted-content envelope handling, no mnemonic, slice-don't-dump, session scope.
4. **`Flow`** is a numbered list of tool calls. One step = one tool call where possible.
5. **Failure modes** appear as a closing `If anything fails` section.

When you add a new skill, also:
- mention it in [the README](../README.md), in this file, and in `AGENTS.md` (housekeeping map) — no per-skill registration is needed; harnesses discover `skills/` automatically

## Using gnomcp tools from skills

Skills should use `gno_connect` to help users reach a chain they reference by gnoweb URL — then `gno_profile_add` to use it immediately (in-memory, dev/testnet only), or the printed `gnomcp profile add` command when the user wants it persisted. Skills must never call write tools (`gno_call`, `gno_run`) without first confirming an active session exists via `gno_auth_status` and guiding the user through `gno_session_propose` if needed.
