# Docker as Default Deployment

**Status: accepted, not yet implemented. Current distribution is `go install` + goreleaser binaries; no image is published.**

## Context

The write-authorization model promises that gnomcp never touches the user's master key. As policy that is enforced by the absence of any master-key code path; as a *structural* property it needs the surrounding environment's help — a native binary on the host could in principle reach `~/.gnokey` via a bug or supply-chain compromise.

A container has no view of the host filesystem outside explicit volume mounts, turning the policy into a physical property.

## Decision

The canonical end-user deployment will be a Docker container published at `ghcr.io/gnoverse/gnomcp`, with `~/.gnokey` never mounted:

| Host path | Container path | Mode |
|---|---|---|
| `~/.config/gnomcp/` | config | read-only |
| `~/.local/share/gnomcp/` | state (agent keys, sessions, audit log) | read-write |

The `gnokey` commands emitted by `gno_session_propose`/`gno_session_revoke` always run on the user's host — the container only produces command strings — so no mount ever needs to expose key material.

Until the image ships, distribution is the native binary (`go install` or goreleaser artifacts), documented as the weaker posture: isolation is policy, not structure. State paths follow XDG conventions on the host (`~/.config/gnomcp`, `~/.local/share/gnomcp`); the container mapping will mount those same directories.

## Alternatives considered

**Standalone binary as the permanent default.** Rejected as the end state: it caps the master-key claim at policy strength. Acceptable as the interim because only dev/testnet chains are reachable (chain-id allowlist), bounding what a compromise can sign for.

**Per-chain container.** Subsumed by multi-chain profiles: one container, many profiles.

**Embedding `gnokey` in the container.** Rejected: recreates the master-key-in-MCP problem the model exists to avoid. The user's host `gnokey` is exactly the isolation boundary.

**System packages (`brew`, `apt`).** Deferred; possible secondary channel after the image ships.

## Consequences

- Until the image exists, "master key never touches gnomcp" is enforced by code review and the absence of key-reading code, not by isolation. The chain-id allowlist limits stakes in the meantime.
- When implemented: releases become image tags, updates become `docker pull`, and the structural claim holds regardless of code-path bugs.
- Docker becomes a documented prerequisite for the recommended path; the native binary stays supported for development.
