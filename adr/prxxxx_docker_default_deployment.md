# Docker as Default Deployment

## Context

The session-based authorization model (separate ADR) requires that gnomcp never have access to the user's master key. As a policy this is enforced by the absence of any master-key code path inside gnomcp. As a structural property it must be enforced by the surrounding environment: a process running on the host with read access to `~/.gnokey` could in principle reach the master via a code-path bug, a transient dependency, or a supply-chain compromise.

`gno-mcp` PR #3 (open) already ships a Docker image at `ghcr.io/gnolang/gno-mcp` as the recommended distribution. The container has no view of the host filesystem outside explicit volume mounts.

## Decision

The canonical deployment shape for gnomcp v2 is a Docker container. The image is published at `ghcr.io/gnoverse/gnomcp` and tagged per release.

**Standard volume conventions:**

| Host path | Container path | Mode |
|---|---|---|
| `~/.config/gnomcp/profiles.toml` | `/etc/gnomcp/profiles.toml` | read-only |
| `~/.local/share/gnomcp/` | `/var/lib/gnomcp/` | read-write (audit log) |

`~/.gnokey` is **never** mounted. The container has zero filesystem access to master keys, by design.

**Standard invocation:**

```bash
docker run --rm -i \
  -v ~/.config/gnomcp:/etc/gnomcp:ro \
  -v ~/.local/share/gnomcp:/var/lib/gnomcp \
  ghcr.io/gnoverse/gnomcp:latest
```

**Claude Code `.mcp.json`** uses the `docker run` invocation as the canonical pattern.

**Standalone binary mode** is supported for development and testing. When `gnomcp` runs as a native binary on the host, the structural isolation property is lost — the binary can in principle reach `~/.gnokey`. This is documented as a weaker posture, intended for local development of gnomcp itself, not for end users.

**The `gnokey maketx session create` command** emitted by `gno_session_propose` runs on the user's host, not in the container. The container produces the command string; the user runs it via their own `gnokey` install. Volume mounts never need to expose `~/.gnokey` to gnomcp.

## Alternatives considered

**Standalone binary as the recommended default**, with Docker as an option. Rejected: weakens the "master key never touches MCP" claim from structural to policy. The session-authorization ADR depends on this property being structural for the security claim to hold.

**Per-chain container** (one container per chain profile). Subsumed by the multi-chain-via-profiles ADR: a single container loads multiple profiles.

**System-level package** (`apt`, `brew`, etc.). Deferred. May coexist with Docker as a secondary distribution channel once the security model is settled. Initial release ships Docker only.

**Embed `gnokey` inside the container** so the user does not need it on the host. Rejected: this re-creates the master-key-in-MCP problem the session model exists to avoid. The user's gnokey on the host is exactly the isolation boundary.

## Consequences

- Distribution depends on Docker being installed on the user's machine. Documented as a prerequisite.
- Image cold-start cost adds a small latency on first invocation per session (typically <1s for a distroless-based image).
- Master keys are physically isolated from gnomcp regardless of any code-path bug, supply-chain attack, or runtime error inside the container.
- Releases ship as image tags; updates are `docker pull` operations. No host install/update mechanism to maintain.
- The standalone binary mode remains available but is not the supported path for end users. Documentation states this clearly.
- The user retains full control of master keys; gnomcp's container has no read or write access to `~/.gnokey`.
