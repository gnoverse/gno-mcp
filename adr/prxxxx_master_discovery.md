# Master Address Discovery

## Context

ADR `prxxxx_session_authorization.md` establishes that gno-mcp signs writes with chain-bounded session keys. For a session-signed `MsgCall` or `MsgRun`, the chain (PR #5307) requires:

- `MsgCall.Caller` (or `MsgRun.Caller`) set to the **master address**.
- `Signature.SessionAddr` set to the session's own address.
- A prior `MsgCreateSession` on chain at path `/a/<master>/s/<session>` linking the session pubkey to the master and declaring the scope.

To populate `Caller` and to look up the session record via the per-master ABCI endpoint `/auth/accounts/<master>/session/<session>`, gnomcp must know the master's bech32 address at write time.

The chain (PR #5307) does not expose a reverse lookup (`/auth/sessions/<session>` → `{master, scope}`). All session query endpoints are master-keyed.

The v2 design spec asserts gnomcp must not require master credentials in its process or configuration. The master's signing material never enters gnomcp; that is structurally enforced by the OAuth-style authorization flow. The master's **address**, by contrast, is public bech32 data. The constraint that needs preserving is "gnomcp does not require the user to type or paste a master address into config to function" — the address must enter gnomcp's state at runtime, derived from an unambiguous on-chain signal, not pre-configured.

## Decision

gnomcp discovers the master address per session by observing a chain-side signal the user produces as part of the authorization ceremony. The signal is a `bank.MsgSend` to the session address carrying a fixed memo. The funder of that Send is taken as the master for the session.

**Discovery flow (default):**

1. `gno_session_propose` generates the session keypair and returns to the agent:
   - The session bech32 address (`g1...`) and pubkey (`gpub1...`).
   - Two user-side instructions, presented together in the tool result text:
     - **(a)** Send 1 ugnot to the session address with memo `gnomcp-auth`. Any wallet that broadcasts `MsgSend` works.
     - **(b)** Authorize the session scope by running the pre-formatted `gnokey maketx session create -pubkey <gpub> -allow-paths ... -spend-limit ... -expires-at ... <master-key-name>` command.
2. The user performs both actions from the same master wallet. Order does not matter. Some wallets can bundle them into a single multi-msg transaction; gnomcp does not require this.
3. On the next write attempt or `gno_auth_status` call for the profile, gnomcp queries tendermint `tx_search` for transactions matching `transfer.recipient='<session_addr>' AND tx.memo='gnomcp-auth'`. The most recent matching Send's signer address is recorded as `SessionMeta.MasterAddress`.
4. gnomcp then queries `/auth/accounts/<master>/session/<session>` to confirm the `MsgCreateSession` from step (b) landed and to read back the chain-enforced scope. If found, the session is marked active; subsequent writes use `MsgCall.Caller = master` and `Signature.SessionAddr = session.Address()`.

The discovered master address is persisted alongside the session in `SessionMeta`. Subsequent broadcasts on the same session skip discovery.

**Hardcoded escape hatch:**

A per-profile optional `master-address = "g1..."` field in `profiles.toml`. When set:

- `gno_session_propose` omits the Send-discovery instruction in its output. The user runs only the `gnokey maketx session create` command.
- gnomcp uses the configured address directly when populating `SessionMeta.MasterAddress`.

The field accepts only a bech32 address — no key material. It is intended for CI environments, scripts, or users who prefer a one-time config edit over the Send dance.

**State transitions:**

- A session in `pending` state runs discovery on every write attempt against its profile until either (a) the Send is found and the session is observed active on chain, transitioning to `active`, or (b) the operator cancels the proposal.
- If discovery finds a Send but the corresponding `MsgCreateSession` has not yet landed, the session stays `pending` and the call returns `authentication_required` with text "session creation transaction not observed yet — retry shortly."
- A session whose `MsgCreateSession` is observed but whose funder cannot be resolved (Send not found) stays `pending` with text "fund the session address with memo `gnomcp-auth` from your master wallet."

## Alternatives considered

**Per-call `master_address` tool argument.** Rejected. Forces every agent call to include master, which agents generally do not have. Surfaces master to the agent context, expanding the leak surface beyond gnomcp's process. Also breaks the "agent reasons about realms, not about master keys" UX premise.

**Profile-only configuration with no discovery.** Rejected as default. Requires every user to copy a bech32 string into a config file before any write tool works. Eliminates the zero-config Docker target and forces non-trivial setup. Available as the escape hatch above for users who prefer it.

**Tx-hash confirm-back tool.** Considered. After running the gnokey command, the user pastes the tx hash back to the agent, which calls a new `gno_session_confirm(tx_hash)` tool. gnomcp queries `/tx?hash=<hash>` to extract `MsgCreateSession.Creator`. Chain-verified, no extra Send. Rejected as primary mechanism because it requires the agent to ask the user for the tx hash mid-flow — a back-and-forth the Send-with-memo path avoids by piggybacking on the user's wallet output. The mechanism may still be added as a fallback for users whose wallets do not surface tx hashes in a copy-friendly form, but is not part of the default flow.

**Indexer search for `MsgCreateSession.SessionKey = <our_pubkey>`.** Considered. Would let gnomcp discover the session creation tx (and master via `Creator`) directly, without requiring a separate Send. Rejected because it depends on a tx-indexer being configured for the profile; tx-indexer is optional in the profile config (`tx-indexer-url`), and the discovery mechanism must work on profiles without one. Not all chain deployments run the indexer (notably local gnodev).

**Tendermint websocket subscription for `MsgCreateSession` events filtered by `SessionKey`.** Considered. Real-time, no indexer dependency. Rejected for v2 baseline because it adds a long-lived subscription connection to chain.Real and complicates the on-demand query model the rest of gnomcp uses. May be added in a future ADR if discovery latency becomes a UX concern.

**Send memo as the sole authorization (no `MsgCreateSession`).** Adopted in `gno-mcp` v1 PR #3 for the simpler "MCP-as-self" model. Rejected for v2 because it abandons chain-enforced scope (`AllowPaths`, `SpendLimit`, `SpendPeriod`, `ExpiresAt`) — the realm sees the MCP's address as caller, and authorization is binary against the funded balance. v2's chain-bounded session model is stronger; the Send is reused here only as a discovery channel for the master address, not as the authorization mechanism.

## Consequences

- gnomcp boots with zero configuration. The user's first write attempt produces the proposal payload; the user's wallet ceremony (Send + session-create) completes setup.
- `SessionMeta` gains a `MasterAddress` field, persisted alongside the session on disk. This is the master's bech32 string only — no key material. Files remain `0600`; encryption-at-rest still opt-in per the session-authorization ADR.
- `chain.Client` gains a discovery method (or `chain.Real.Call` performs the discovery inline before signing). The method uses tendermint `tx_search`, which all gno chains expose by default.
- The user does two operations to authorize a session: a Send and a `MsgCreateSession`. Wallets that bundle them as a single multi-msg tx (a single signature) reduce this to one operation. gnomcp does not require multi-msg support.
- The memo string `gnomcp-auth` is part of the protocol. Changing it is a backwards-incompatible change to the discovery flow.
- Discovery introduces one extra tendermint RPC call per pending-session write attempt. The result is cached in `SessionMeta`; once a session is `active`, no further discovery queries occur for that session.
- Master address per session means gnomcp can support multiple master accounts simultaneously, one per session, without per-profile master configuration. A user with separate work and personal master keys can authorize two sessions for the same profile and gnomcp picks the correct one per call based on `AllowPaths` matching.
- The escape-hatch field `Profile.MasterAddress` covers CI and headless scenarios where Send observation is impractical (no wallet, no human to perform the ceremony).
- A user who misses the memo or sends from a non-master wallet sees a clear text error pointing back to the proposal instructions. There is no silent failure mode.
- Discovery does not attempt to validate that the funder address is the same one that signs `MsgCreateSession`. The chain enforces this implicitly: if they differ, `/auth/accounts/<funder>/session/<session>` will not find the session and gnomcp stays in `pending` with text "session not found under funder address — confirm Send and session-create came from the same master wallet."
