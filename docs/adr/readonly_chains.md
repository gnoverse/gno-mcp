# Read-Only Chains for Audit

**Status: implemented. Supersedes the "mainnet is structurally unreachable" stance of `multichain_via_profiles.md` for READS only — writes stay confined to dev/testnet.**

## Context

`multichain_via_profiles.md` made mainnet/betanet unreachable by construction: config validation rejected any `chain-id` not matching `^(dev|test-?\d+)$`, at startup, at `gnomcp profile add`, and at `gno_profile_add`. The threat model was "an LLM-driven signer must not reach a chain holding real funds."

That allowlist did double duty — it gated *write capability* and it *admitted* chains — and conflating the two blocked a core use case. Auditing deployed source means reading the code that is live on its chain. An agent asked to audit `https://gno.land/r/gnoland/blog` could not reach gno.land at all; the realistic failure was worse than useless — it fell back to off-chain repo source and audited a different artifact than the one deployed. `multichain_via_profiles.md` anticipated this: "lifting that for reads would be a deliberate future decision." This is that decision.

## Decision

**Chain-id decides capability, not admission.** The single allowlist splits in two:

- `ChainIDWritable(id)` — `dev`, or an id starting with a known testnet name (the release-time `testnetChainNames` list: `test`, `topaz`; bare or hyphenated — `test5`, `test-13`, `topaz-1`). Codenamed testnets cannot be recognized by a `test<N>` pattern, so the gate is a name list, not a regex. Write-capable: agent key path, faucet, sessions, `master-address`, and presence in every write tool's `profile` enum.
- `ChainIDValid(id)` — a format-safety gate, `^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$`, ≤64 chars. Any chain-id passing it may enter config. It still refuses whitespace and shell metacharacters, because the chain-id is interpolated into the `gnomcp profile add` / `gnokey` commands the user pastes into a terminal.

A format-safe but non-writable chain-id (betanet `gnoland1`, `staging`, mainnet) is admitted **read-only**: the read tools list it; every write tool excludes it from its `profile` enum; the keystore refuses to derive an agent key for it (defense in depth); no faucet; and `master-address` on a read-only chain is a config error. **No code path signs for a read-only chain.**

**Three-way classification.** `Profile` is now `IsLocal` / `IsTestnet` (a write-capable testnet) / `IsReadOnly`, with `Kind()` returning `local | testnet | read-only`.

**URL-driven resolution.** A gnoweb URL is authoritative for *which* chain. The audit method, the `gno-auditor` agent, and the MCP server instructions require resolving the chain from the URL (`gno_profile_add` with `gnoweb_url=…`) before reading, and reading on-chain source only — never substituting repo/local source for a named deployed realm.

## Alternatives considered

**Keep the hard allowlist (status quo).** Rejected: it makes auditing gno.land impossible and pushes agents toward off-chain source substitution — strictly worse for audit integrity than a read-only profile.

**Explicit `read-only = true` profile flag.** Rejected: more config surface for no gain; the chain-id already carries the capability signal, so inference needs no new field.

**Named read-tier allowlist** (only blessed mainnet ids admitted read-only). Rejected: a list to maintain; betanet RPCs and ids move; format-safety plus capability-by-chain-id is general and self-healing.

**Built-in `gnoland1` read-only profile.** Rejected for now: it hardcodes an external betanet RPC that moves; connect-driven resolution (`gno_profile_add` from the URL's `gnoconnect` meta-tags) self-heals.

## Consequences

- gnomcp can read — and audit — mainnet/betanet. It still cannot write to them: the agent key, faucet, sessions, and `master-address` remain `ChainIDWritable`-gated and re-checked at the keystore. The original threat model holds where it matters — the signer cannot sign for a real-funds chain; it can only read one.
- The `multichain_via_profiles.md` consequence "mainnet interaction is structurally impossible" and its "out of scope entirely" framing are narrowed to **writes**.
- The chain-id format gate replaces the allowlist's admission role; the error code `chain_forbidden` is replaced by `chain_id_malformed` (format violations only).
- `master-address` on a read-only chain fails validation loud — sessions remain a writable-chain-only path.
- A non-test chain-id typo (e.g. `test_5`) is admitted read-only rather than rejected; on the dynamic path the node's reported chain-id must still match the declared one, so a typo surfaces as `chain_id_mismatch`.
