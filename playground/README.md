# Playground — Dockerized gnomcp test harness

Run Claude Code in a clean container with layered amounts of gno tooling, to test the
gnomcp MCP server + `gno` skill + `auditor` agent. No host config leaks in (token-only
auth, fresh `$HOME`). The L1–L3 images carry no `gnokey`; the **e2e/sim** target ships
one so the driver (playing the user/supervisor) can authorize chain sessions — the AUT
must never use it (a hard-fail invariant, see the driver's judging rules).

## Layers

| Make target | Layer | Contains | Use it to test |
| --- | --- | --- | --- |
| `make playground-fresh` | L1 | clean Claude Code only | the first-time-user install UX (install the plugin + MCP yourself) |
| `make playground-gnomcp` | L2 | L1 + skill + agent + MCP pre-installed, **no gnodev** | gnomcp against a remote testnet (no local node) |
| `make playground-full` | L3 | L2 + `gnodev` | the full local-devnet experience |
| `make playground-sim` | e2e | L3 + **simnet running** (in-memory node + faucet + gnoweb, chain `test9999`) | hands-on work against the simulated testnet; gnoweb published to the host — `GNOWEB_PORT=9999` overrides the default `8688` |

`L3 builds on L2 builds on L1`; `docker build --target` builds the ancestors automatically.

## Setup (once)

1. On the host, generate a long-lived token (requires a Claude subscription — **not** an API key):
   ```bash
   claude setup-token
   ```
2. Create the env file and paste the token:
   ```bash
   cp playground/.env.example playground/.env
   # edit playground/.env → CLAUDE_CODE_OAUTH_TOKEN=<paste here>
   ```
   `playground/.env` is gitignored and is the only place the token lives.

## Run

```bash
make playground-fresh     # or playground-gnomcp / playground-full
```

Each target builds the image (cached after the first run — the first build is slow: it clones
the gno monorepo and compiles gnodev) and drops you into a **ready shell** (not an auto-started
Claude session). Run `claude` when you want it. Override the engine with `DOCKER=podman` if needed.

## Inside the container

- You land in `zsh` (with a [starship](https://starship.rs) prompt) and the environment ready. Run
  `claude` to start Claude Code; the token from `.env` is already in the environment.
- Claude Code defaults to **Bypass Permissions mode** (no per-action approval prompts — this is a
  throwaway dev container). Claude still shows a one-time "Yes, I accept" on launch that no config can
  pre-suppress (upstream `claude-code#52501`); accept it once per session.
- `emacs-nox`, `vim`, `tmux`, `git`, `curl` are available for demos.
- **L3 only:** start a local devnet yourself — `gnodev` is on PATH but not auto-started. Once it's
  serving `:26657`, gnomcp's built-in `local` profile auto-discovers it and write tools sign with the
  built-in `test1` key.
- Containers are ephemeral (`--rm`); the workspace is not persisted to the host.

## e2e harness (playground driver)

> ⚠️ **Developer tooling for this project only.** The batch targets run a Claude driver
> **unattended on your host with permissions bypassed** (`--dangerously-skip-permissions`) so it
> can manage Docker containers and write reports headlessly. Don't run it on machines or in
> directories you don't fully trust it with, and read a scenario before running it.

A driver Claude (host) QAs the containerized Claude+gnomcp+gno-skill (the AUT) scenario by
scenario: it plays the user turn-by-turn over `docker exec`, verifies chain ground truth against
the in-container **simnet** (in-memory gnoland node + faucet + gnoweb on chain `test9999`, the
`e2e` Docker target's main process), then interviews the AUT about its tool and skill choices.

- **Interactive (debug):** `cd playground && claude`, then `/playground-driver [scenario-id]`.
- **Batch:** `make playground-e2e` (local tier — gates: every scenario must pass) /
  `make playground-e2e-external` (outside-resource scenarios — random failure tolerated, only
  hard `fail` gates). Narrow the run via `ARGS`: `make playground-e2e ARGS="--scenario session-flow"`
  runs one scenario (fastest way to iterate on a single test); `ARGS="--category reads"` (writes,
  connect, …) filters by feature set.
- **Scenarios:** `e2e/scenarios/<tier>/*.md` — copy `e2e/TEMPLATE.md`; authoring guide in
  `.claude/skills/playground-driver/references/scenario-format.md`. Only `### Instruct` is ever
  sent to the AUT; `Expect`/`Verify` stay driver-side.
- **Coverage:** `e2e/COVERAGE.md` — the feature → scenario ledger (what each scenario `covers:`,
  what is still a gap and why). Update it when adding scenarios or features.
- **Reports:** `e2e/reports/<timestamp>/` (gitignored) — `report.md` (verdicts, findings, debrief
  transcript, improvement leads), `results.json` (the make exit-code contract), per-turn
  stream-json logs (the tool-call evidence).
- **Lifecycle is driver-managed:** batch recreates the container at sweep start, tears down on
  green, and **keeps it alive after failures for postmortem** — clean up with
  `e2e/scripts/down.sh`.
