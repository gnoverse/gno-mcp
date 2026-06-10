# AGENTS.md — working on this repo

This file is for agents **contributing to gnomcp** (a Go MCP server for gno.land). If you're here to *write Gno realms or use the tools*, you want the bundled skill instead: `skills/gno/SKILL.md`.

## Commands

```bash
make test                # unit tests (no network)
make test-integration    # in-process gno node (build tag: integration)
make lint                # go vet + gofmt -l
make build               # bin/gnomcp
make dev                 # go run ./cmd/gnomcp (starts MCP server over stdio)
```

Prefer `go run` over `go build` for ad-hoc runs — no stray binaries.

## Map

| Path | What |
|---|---|
| `cmd/gnomcp/` | entry point, flags, tool registration (`register.go`), profile CLI, MCP server instructions |
| `internal/chain/` | chain access seam: `Client` interface / `Real` (gnoclient+RPC) / `Fake` (tests) / `Resolver` |
| `internal/server/` | tool `Registry`, `Tool`/`Result`/`ToolError` types, profile-arg schema |
| `internal/tools/{read,write,indexer,admin}/` | one `Register*` func per tool |
| `internal/session/`, `internal/keystore/` | session lifecycle/scope; per-profile agent keys |
| `internal/profiles/` | profiles.toml loading, validation, chain-id allowlist, hard limits |
| `internal/untrusted/`, `internal/budget/`, `internal/audit/` | envelope+neutralization, output budget, JSONL audit log |
| `faucet/`, `cmd/agentfaucet/`, `internal/clientfaucet/` | faucet service + client tiers |
| `test/integration/` | real-node tests (`-tags=integration`) |
| `test/e2e/` | **manual** protocol checklists — keep them manual, don't convert to Go suites |
| `docs/adr/` | decision records, reconciled to shipped state (status line at top, no `prxxxx_` prefixes) |
| `skills/gno/` | the bundled user-facing skill (product, not contributor guidance) |

## Conventions

- TDD: failing test first; unit tests use `chain.Fake`, integration tests the in-process node.
- testify `require`/`assert`; small focused test funcs over mega-tables.
- New tool = copy the shape of `internal/tools/read/packages.go`: `Register*` func, schema via `addProfileArg`, explicit Annotations, description answering what/when/returns/NOT/format. Wire it in `cmd/gnomcp/register.go`.
- Gated tools (indexer/faucet) register per profile guards in `register.go`; re-registration after dynamic adds must stay idempotent.
- Commit style: conventional commits; ADR-only changes use `docs(adr):`.

## Security invariants — never break

- Chain-id allowlist `^(dev|test-?\d+)$` — no path may admit other chains.
- Every chain-derived text output goes through `budget.Wrapped` (untrusted envelope + budget). Structured numeric fields may stay raw.
- The user's keys/mnemonics never enter the process; session/agent authorization happens via printed `gnokey` commands the user runs themselves.
- Never log raw tool args — audit records use redacted summaries.
- Key and session files: mode `0600`; honor `GNOMCP_SESSION_PASSPHRASE` encryption.
- Every write result must name its signing identity (agent vs session).

## Housekeeping — what to update when

| When you… | Update |
|---|---|
| Add / rename / remove a tool | `docs/tools.md` (catalog) · README tool table + counts (two places: "Why this exists", table header) · `skills/gno/references/mcp.md` task table · `docs/security.md` envelope list (if it's a text tool) · server `instructions` in `cmd/gnomcp/main.go` (if a flow changes) |
| Change write-auth / session / scope behavior | `docs/security.md` · README "Write authorization" · `docs/adr/session_authorization.md` |
| Change profile fields or config semantics | README "Profiles" · `docs/adr/multichain_via_profiles.md` · `test/e2e/profiles.toml` if protocols use it |
| Make or revise an architectural decision | `docs/adr/` — edit in place with an updated status line; keep records matching shipped state, not plans |
| Add a skill or a reference file under `skills/` | `skills/plugin.json` · `docs/skills.md` · README skills section · **this file** (Map + this table) |
| Add a make target or change the dev flow | this file (Commands) |
| Bump the version | every plugin manifest must match the tag — `package.json`, `.claude-plugin/`, `.codex-plugin/`, `.cursor-plugin/`, `gemini-extension.json` (release CI enforces it) |
| Touch skill content | monorepo (`gnolang/gno`) is the source of truth — point at it, don't restate it (`docs/skills.md` § source of truth) |

Stale docs are bugs: if a change makes any claim in README/docs wrong, fixing it is part of the change, not a follow-up.

## Special files

- `GEMINI.md` imports the gno skill **on purpose** — it's the Gemini extension's `contextFileName` (see `gemini-extension.json`), i.e. skill delivery to extension users, not contributor context. Leave it.
- `.mcp.json` at the repo root is the dev-mode server config (`go run`); user-facing config lives in the README.
