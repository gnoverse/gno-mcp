# Skills

The `skills/` directory is a Claude plugin package. Each subdirectory is a single skill; `plugin.json` lists them. Skills are pure markdown — no Go code — and are designed to drive `gno-mcp` tools through coherent workflows without dropping the security guardrails.

## When to invoke each

| Skill | Trigger |
|---|---|
| [`gno-onboarding`](../skills/gno-onboarding/SKILL.md) | `onboarding_required` error, or "I want to try gno", "set me up". Default network testnet. Never asks for or shows a mnemonic. |
| [`gno-explore-realm`](../skills/gno-explore-realm/SKILL.md) | User pastes a realm path / gnoweb URL / asks "what does this realm do". Calls `gno_get` + `gno_inspect`, summarises. |
| [`gno-read-contract`](../skills/gno-read-contract/SKILL.md) | "How does this contract work / is it safe / what does function X do". Reads in slices, surfaces invariants and risk patterns. Never dumps full source. |
| [`gno-debug-tx`](../skills/gno-debug-tx/SKILL.md) | Failed-tx hash or error string. Classifies the failure, reads the called function, proposes a concrete fix. |

## Authoring conventions (so v0.2+ skills stay coherent)

1. **Frontmatter `description`** must be specific enough that Claude's skill router picks it correctly without overlap with sibling skills. Lead with the trigger.
2. **`When to use`** section comes first — concrete user phrasings.
3. **`Guardrails` / `Judgment`** sections call out the rails: untrusted-content envelope handling, mainnet confirmation, no mnemonic, slice-don't-dump.
4. **`Flow`** is a numbered list of tool calls. One step = one tool call where possible.
5. **Failure modes** appear as a closing `If anything fails` section.

When you add a new skill, also:
- list it under `skills` in [`skills/plugin.json`](../skills/plugin.json)
- mention it in [the README](../README.md) and in this file

## v0.2 deferred

Skill transcript snapshots (Plan Task 25) — a scripted MCP client that replays "user said X" and asserts "tool sequence Y" — are deferred past v0.1. Track progress in the milestone.
