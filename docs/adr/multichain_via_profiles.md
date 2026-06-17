# Multi-Chain via Profile Arg

**Status: implemented. The chain-id allowlist is superseded *for reads* by [readonly_chains.md](readonly_chains.md): non-`(dev|test-?\d+)` chains are admitted read-only; writes stay confined to dev/testnet.**

## Context

A gno developer works across multiple chains: a local devnet (`gnodev`), the current public testnet (`test-13` at time of writing; successor testnets reset on a regular cadence), and occasionally other test deployments. The MCP server must support all of these without forcing the user to install or configure separate MCP instances per chain.

The threat model treats mainnet as out of scope entirely: an LLM-driven signer must not be able to reach a chain holding real funds, by construction rather than by ceremony.

## Decision

gnomcp runs as a single binary with a single instance, loading multiple chain profiles. Every chain-bound tool takes a `profile` argument declared as an enum over loaded profile names.

> **Superseded for read tools.** Read tools (`gno_render`, `gno_eval`, `gno_read`, `gno_packages`, `gno_account`, `gno_status`) now declare `profile` as a **free-form string**, not an enum (still validated against the loaded set — an unknown name errors cleanly with no chain client). Two reasons the read enum stopped earning its place: (1) once read-only chains became reachable ([readonly_chains.md](readonly_chains.md)), the enum's mainnet-protection rationale (see the rejected free-form alternative below) no longer applies to reads; (2) the enum actively blocked a runtime-added profile — a client caches the tool schema, so a profile added via `gno_profile_add` was rejected against the *stale* cached enum until the schema was refetched, which is not a reliable client action. A wrong read profile errors at call time, so the enum bought no safety reads still needed. **Write tools keep the filtered enum**: there it gates on chain writability, and no read-only chain may be named in a write call.

**Built-in zero-config profiles.** `testnet` (the current public testnet) and `local` (`dev` chain at `http://127.0.0.1:26657`) ship built in. Reads work with no config file; writes work via the agent identity (see the session-authorization ADR).

**Chain-id allowlist.** Config validation rejects any profile whose `chain-id` does not match `^(dev|test-?\d+)$`. Betanet, staging, and mainnet ids cannot enter the config; there is no override flag. Locality derives from the chain-id (`dev` = local, `test*` = testnet) — there is no separate `chain-type` field.

> **Superseded in part by [readonly_chains.md](readonly_chains.md).** The allowlist is now a *capability* gate, not an *admission* gate: non-`(dev|test-?\d+)` chain-ids are admitted **read-only** (no agent key, faucet, session, or `master-address`), so deployed source on mainnet/betanet can be audited. A format check on `chain-id` remains, and writes stay confined to dev/testnet.

**Profile fields** (`profiles.toml`):

```toml
[<profile_name>]
rpc-url             = "<url>"
chain-id            = "<id>"          # must match ^(dev|test-?\d+)$
master-address      = "g1..."         # optional; enables session writes (bech32 address only)
tx-indexer-url      = "<url>"         # optional; gates gno_list/gno_history/gno_activity
default-spend-limit = "<coins>"       # optional; per-session default, clamped to hard limits
default-expires-in  = "<duration>"    # optional; clamped to hard limits
bypass-hard-limits  = true | false    # default false; disables the clamp layer
faucet-url          = "<url>"         # optional; faucet page gno_faucet_fund links to
faucet-service-url  = "<url>"         # optional; automatic faucet service gno_faucet_fund calls
```

**Config precedence:** built-in defaults < `~/.config/gnomcp/profiles.toml` < `./profiles.toml` < `-config` flag. A profile entry is a whole-profile replacement — an overlay redefining a built-in must re-supply `rpc-url` and `chain-id`.

**`profile` arg behavior.** The schema defaults to the discovered local profile when a local node is found; otherwise the arg is required or defaults per the loaded set. Write tools always require an explicit `profile` — each write names its target chain in the call itself.

**Discovery at startup** probes `127.0.0.1:26657`; if a node responds with a chain-id matching the local profile, that profile becomes the discovered default.

**Dynamic profiles at runtime.** `gno_profile_add` adds a profile in memory for the process lifetime: same allowlist validation, plus the node is dialed to confirm it reports the declared chain-id (gnoweb `gnoconnect` meta-tags are treated as a hint, not truth; a non-loopback gnoweb advertising a loopback RPC is rejected). Init-time profiles are immutable; `default` is a reserved name. Dynamic profiles carry no `master-address` — reads and agent-key writes only; sessions require a persisted profile. After an add, the tool set re-registers (gates re-evaluate, write-tool profile enums regenerate; read tools accept the new profile immediately via their free-form `profile` string) and the server emits `tools/list_changed`, so gated tools can appear mid-session.

**Gnoweb discovery.** `gno_connect` (and `gnomcp profile add --from-gnoweb`) reads `gnoconnect:{rpc,chainid}` meta-tags from a gnoweb page and derives the profile arguments, previewing without mutating anything.

## Alternatives considered

**One MCP server process per chain.** Rejected: the ergonomic cost compounds; cross-chain read workflows would require multiple concurrent processes. The multi-profile design provides comparable safety with materially lower setup cost.

**Active-profile state machine** with a `select_profile` tool mutating an in-server "current chain". Rejected: stateful design complicates audit, tool-list caching, and concurrent access. Per-call `profile` is equivalent in safety and stateless.

**Pure `profile` arg with no default ever.** Rejected: poor ergonomics in the common discovered-local case. Schema-conditional defaulting balances explicitness and ergonomics; writes stay explicit regardless.

**Free-form `chain-id` arg** rather than an enum over named profiles. Rejected: the enum prevents arbitrary identifiers, and the allowlist + user-curated profile list is the mainnet protection.
> **Revised for reads** (see Decision): read tools now take a free-form `profile` *name* (still resolved against loaded profiles — not an arbitrary chain-id), because the enum's mainnet protection no longer holds once mainnet/betanet is reachable read-only. This does not revive the rejected free-form *chain-id*: an unknown profile still errors, and adding a chain still goes through profile validation (`gno_profile_add`/config). The enum is retained on write tools, where it gates writability.

**Per-call mainnet confirmation (`confirm=true`)** as used by the v1 server. Rejected in favor of the allowlist: a gate that cannot be passed beats a gate that asks nicely.

## Consequences

- Single MCP entry in the host config; per-call chain selection; cross-chain reads in one session.
- Mainnet interaction is structurally impossible — there is nothing to misconfigure, confirm, or bypass. The trade-off: gnomcp cannot read mainnet either; lifting that for reads would be a deliberate future decision.
  > **Superseded by [readonly_chains.md](readonly_chains.md):** that future decision was made. Mainnet/betanet is now **readable** (read-only) so deployed source can be audited; it remains **unwritable** — no path signs for it. "Structurally impossible" now applies to writes.
- Mid-session tool-list growth is supported: `gno_profile_add` can summon gated tools (faucet, indexer) without a restart via `tools/list_changed`.
- Testnet resets require updating the built-in `testnet` profile (a release) or overriding it locally.
- Discovery probing adds bounded startup cost (one HTTP request with a short timeout).
