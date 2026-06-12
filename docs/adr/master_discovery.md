# Master Address Discovery

**Status: deferred (automatic on-chain discovery), but partially realized — a writable profile without a `master-address` now accepts a user-supplied PUBLIC `master_address` at `gno_session_propose` time, stored on the session record. See the note under Decision.**

## Context

Session-signed writes need the master's bech32 address at write time: `MsgCall.Caller` is the master, and the chain's session lookup is master-keyed (`auth/accounts/<master>/session/<session>` — there is no reverse `session → master` endpoint). The address is public data, but it has to enter gnomcp's state somehow.

The original v2 design wanted zero-config sessions: the user should not have to paste a bech32 string into a config file before the first session write. The proposed mechanism was discovery via an on-chain signal — a `bank.MsgSend` to the session address carrying a fixed memo (`gnomcp-auth`), found through tendermint `tx_search`, with the funder recorded as the session's master.

## Decision

Discovery is deferred. Sessions require a per-profile `master-address` in `profiles.toml`; `gno_session_propose` and session-signed writes are unavailable on profiles without it (including all dynamic profiles, which deliberately cannot carry one).

What changed since the original design:

- **The agent identity removed the zero-config pressure.** Writes work out of the box via the MCP-owned agent key (separate ADR); sessions are now the deliberate "act as the user" grant, where a one-time config edit is an acceptable ceremony.
- **The chain emits no `MsgCreateSession` events** (verified 2026-05-21), ruling out the cleaner event-subscription variants and leaving only the Send-with-memo dance or indexer-dependent searches.
- The Send-with-memo flow adds a second user action per session and a protocol-significant memo string — real complexity for a problem the config field already solves.

> **Update — per-propose `master_address`.** A writable profile that has no `master-address` no longer dead-ends: `gno_session_propose` accepts a `master_address` parameter (the user's PUBLIC g1… address), stored on the session record and used as `MsgCall.Caller` for that session. This is distinct from the rejected **per-call** `master_address` below: it is supplied **once** at propose time (not on every write), the value is public (not key material), and the tool rejects seed-phrase-shaped input without echoing it. It removes the edit-`profiles.toml`-and-restart ceremony for first-time sessions while keeping the user's gnokey as the sole authorization. On-chain *automatic* discovery remains parked; the user still states their address explicitly.

## Alternatives considered

**Send-with-memo discovery (original design).** A funded `MsgSend` to the session address with memo `gnomcp-auth`; the funder becomes the master. Parked, not rejected — the design is recorded in the repo history. Revisit if config-free session onboarding becomes a priority (e.g. driven by a2a flows).

**Per-call `master_address` tool argument.** Rejected: surfaces the master into agent context on every call and breaks the "agent reasons about realms, not keys" premise.

**Indexer search for `MsgCreateSession` by session pubkey.** Rejected: tx-indexer is optional per profile; discovery must not depend on it.

**Event subscription.** Ruled out: the chain does not emit session-creation events.

## Consequences

- Enabling sessions on a profile is a one-time config edit (`master-address = "g1..."`). CI and headless setups get determinism for free.
- Dynamic profiles are structurally read+agent-write only, which keeps the riskiest path (acting as a user) tied to deliberately persisted configuration.
- One master per profile. Multiple masters require multiple profiles pointing at the same chain — acceptable at current scale.
- If discovery is revived, the Send-with-memo design in this file's history is the starting point; nothing in the current architecture blocks it.
