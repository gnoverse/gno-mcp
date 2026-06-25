# AGENTS.md — working on the playground harness

This is the agent-e2e harness: a host-side **driver** Claude QAs a containerized
Claude+gnomcp+gno-skill (the AUT) scenario by scenario against an in-memory simnet.

**Load the `playground-driver` skill before doing anything here** — running or debugging e2e,
authoring or editing scenarios, or changing the driver. It carries the wire protocol, judging
rules, scenario format, and verify toolkit; without it you will get the mechanics wrong. It
lives at `.claude/skills/playground-driver/` (also linked from the repo-root `.claude/skills/`,
so it surfaces whether your cwd is here or the repo root).

Layer/target map and run instructions → `README.md`. Contributor guidance for the wider repo →
the root `AGENTS.md`.
