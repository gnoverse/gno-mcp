# Multi-Chain via Profile Arg

## Context

A gno developer typically works across multiple chains: a local devnet (`gnodev` or `gnoland start`), the current public testnet (`test11` at time of writing; successor testnets are expected on roughly the same cadence as past resets), and occasionally mainnet (`portal-loop`). The MCP server must support all of these without forcing the user to install or configure separate MCP instances per chain.

The threat model treats mainnet writes as materially more dangerous than testnet writes. Any multi-chain mechanism must keep mainnet writes from being broadcast by mistake when the user intended a testnet or local target.

## Decision

gnomcp runs as a single binary with a single instance, loading multiple chain profiles from `profiles.toml`. Every chain-bound tool takes a `profile` argument.

**`profiles.toml`** lives at `~/.config/gnomcp/profiles.toml` (host) and is mounted read-only into the container at `/etc/gnomcp/profiles.toml`. Each profile declares:

```toml
[<profile_name>]
chain-type = "local" | "testnet" | "mainnet"   # default: "testnet"
rpc-url = "<url>"
chain-id = "<id>"
tx-indexer-url = "<url>"                       # optional; enables list/history/activity tools
allow-dangerous-tools = true | false           # default: false
default-spend-limit = "<coins>"                # optional; clamped to chain-type max
default-expires-in = "<duration>"              # optional; clamped to chain-type max
bypass-hard-limits = true | false              # default: false
```

Mainnet is intentionally not shipped in the default `profiles.toml`; users add it deliberately.

**`profile` arg behavior** is determined at startup based on which profiles loaded successfully:

| Configuration | `profile` arg |
|---|---|
| Single profile loaded | optional; defaults to the only profile |
| Multiple profiles, local gnodev probe succeeds | optional; defaults to the discovered local profile |
| Multiple profiles, no local detected | **required** for chain-bound tools |

**Write tools always require explicit `profile`** even when a default exists for reads. Each write call names its target chain in the call itself.

**Discovery at startup** probes the RPC endpoint of any profile whose `chain-type = "local"` (or `rpc-url` host is `127.0.0.1` / `localhost`). If `/status` responds and reports a `chain-id` matching the profile, the profile is activated as the discovered local default.

**Per-profile capability checks** at execution time enforce `allow-dangerous-tools` and `tx-indexer-url`. A capability mismatch returns `isError: true` with a structured payload identifying the missing capability and the `profiles.toml` edit that would enable it.

**Tool schemas** are built at startup based on what's loaded. The `profile` parameter is declared as an enum over loaded profile names. Its description surfaces per-profile semantics (`chain-type`, `allow-dangerous-tools` status, indexer availability) so the agent has factual context for selection without being told how to behave.

## Alternatives considered

**One MCP server process per chain.** The user would launch `gnomcp` separately for each chain they target (local devnet, testnet, mainnet), each with its own configuration. Rejected: the ergonomic cost compounds across the project's lifetime; cross-chain read workflows require multiple concurrent processes; the multi-profile design with per-call hard errors + structural session authorization (separate ADR) provides comparable safety with materially lower setup cost.

**Active-profile state machine** with a `select_profile` tool that mutates an in-server "current chain" state. Rejected: stateful design complicates audit, tool-list caching, and concurrent access. Per-call `profile` arg with hard errors is equivalent in safety and stateless.

**Pure `profile` arg with no default ever**, requiring the agent to specify on every call. Rejected: poor ergonomics in the common single-profile case. The schema-conditional defaulting (single profile → optional, multiple-with-local → defaults to local, multiple-no-local → required) gives a defensible balance.

**Free-form `chain-id` arg** rather than an enum over named profiles. Rejected: enum constraint prevents the agent from passing arbitrary identifiers; mainnet protection benefits from the user-curated profile list.

## Consequences

- Single Claude Code MCP entry; users do not reconfigure their host MCP setup per chain.
- Cross-chain reads in a single session are possible via the optional `profile` override.
- Mainnet broadcast requires (1) explicit mainnet entry in `profiles.toml`, (2) `allow-dangerous-tools = true` on that entry, (3) explicit `profile=mainnet` in the tool call, plus the upstream session authorization gates (separate ADR). Five-gate accidental-mainnet protection.
- Tool descriptions are less chain-specific than a per-chain alternative would allow. Mitigated by surfacing per-profile semantics in the `profile` enum description and in the `initialize` server-info.
- A tool-list mutation when a new profile is loaded mid-session is not supported. Profile changes require a server restart; `profiles.toml` is read once at startup.
- Discovery probing adds startup cost (one HTTP request per `local`-type profile). Bounded at ~2 seconds per profile via timeout.
