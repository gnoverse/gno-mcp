#!/usr/bin/env bash
# SessionStart hook (gnomcp plugin): inject the gno orientation rule so it is
# always present in context, independent of the MCP server. This is a CONTEXT
# INJECTION, not a gate — it never blocks a tool; it just keeps one unconditional
# rule in front of the agent so engaging the gno skill is the default for any gno
# work. The rule is deliberately terse and absolute (no conditions, no rationale):
# conditions invite the agent to judge its way out, and felt-familiarity — exactly
# the case where gno's drift-prone conventions bite — is where that judgment fails.
#
# The rule text has no JSON metacharacters, so it is embedded directly.
set -uo pipefail

printf '{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"Any task involving gno: engage the gno skill first, before acting. Always. No exceptions."}}\n'
exit 0
