# a2a Serve Mode + Realm-as-Source-of-Truth

**Status: accepted, not yet in tree. A proof of concept exists on a development branch; its wire shape is not a2a v1.0, and a rewrite onto the official a2a-go SDK is planned before merge.**

## Context

Typed task interfaces are the intended default agent-write path: `gno_call` is open-ended by shape, and narrowing agent writes to realm-published task interfaces is the structural fix. The a2a v1.0 protocol is the chosen vehicle, which raises two questions: how gnomcp exposes a2a to (a) the MCP host it already serves and (b) external a2a clients (CrewAI, LangGraph, …).

Research into the a2a v1.0 spec and SDK ecosystem established:

- a2a v1.0 transports are HTTP/JSON-RPC, gRPC, or HTTP+JSON/REST — all network-based. A stdio transport is proposed upstream (`a2aproject/A2A#1074`) but not in the spec or any SDK.
- No major a2a client supports subprocess/stdio transport today; interop requires speaking HTTP.
- The PoC validated the flow end-to-end but hand-rolled the wire protocol; the official a2a-go SDK is the correct base for a spec-conformant implementation.

The architectural constraint: host HTTP without a long-running background daemon — daemons add lifecycle complexity (systemd/launchd, sockets, shim binaries) the architecture deliberately avoids.

## Decision

Two operational modes, both foreground and user-controlled; no daemon.

**Mode 1: `gnomcp` (stdio, default — shipped today).** Spawned by the MCP host; lifetime tied to the host session.

**Mode 2: `gnomcp a2a --serve <realm>` (planned).** User-invoked, foreground, per-realm process: establishes a session for the target profile+realm (printing the `gnokey` authorization command if needed), binds HTTP on `127.0.0.1` with a per-spawn bearer token, and serves the a2a protocol for that realm until Ctrl+C.

**Realm as source of truth.** The realm implements the a2a task lifecycle on chain (`SubmitTask` / `GetTaskStatus` / `CancelTask` / `AgentCard`). gnomcp is a thin bridge — submit and cancel are session-signed calls, status is an ABCI read, streaming is polling-based. gnomcp maintains no internal task state machine.

**Card validation.** For realms that publish an agent card, gnomcp validates invocations against the card's declared methods and argument schemas — including `gno_call` against card-having realms. This is the planned narrowing of open-ended writes; card-less realms retain the current open-ended behavior under the identity/scope gates.

## Alternatives considered

**Long-running daemon** sharing sessions across clients. Rejected: significant operational complexity for the same HTTP interop the per-spawn model provides.

**Stdio a2a transport.** Rejected until `a2aproject/A2A#1074` lands and SDKs adopt it; being the only consumer of a custom transport helps no one.

**gnomcp with its own a2a agent identity.** Rejected: realms are the agents; gnomcp is the runtime. An identity layer on top would muddy audit, signing, and discovery.

**gnomcp-internal task state machine.** Rejected: tasks would die with the process and be invisible to observers; on-chain task state is persistent, observable, and auditable for free.

**Keeping the hand-rolled PoC wire protocol.** Rejected: interop is the entire point; spec conformance via the official SDK beats a lookalike.

## Consequences

- Merging this work means: a2a-go SDK dependency, the `a2a` CLI mode, bridge tools, and card validation — each landing against the shipped base without restructuring it (registration pipeline, profile resolution, and identity dispatch already accommodate them).
- Realm authors must implement the task-lifecycle pattern; a community library is the expected path.
- Every task submission is a chain write with gas cost — appropriate for typed, bounded work, not high-volume trivia.
- Until this ships, agent writes remain open-ended (`gno_call`/`gno_run` under identity + session scope); the session ADR records that as a known, deliberate gap.
