# Deploying to Chains That Park Code

**Status: implemented. The default testnet, onyx, runs the inert code-submission policy.**

## Context

A chain running the `inert` code-submission policy accepts a `MsgAddPackage` from anyone but does not run it: the package is stored parked, with no type check and no `init()`, until an address in `params/vm:p:pkg_approvers` sends `MsgEnablePackage`. onyx and mainnet both run it. On onyx the approver is an oracle that type-checks and preprocesses each submission and enables the ones that pass, usually a few seconds after the deploy commits; a package that fails stays parked indefinitely.

Three things break for an agent that treats such a chain like a permissionless one:

- The deploy transaction succeeds, so a tool that reports the transaction reports "AddPackage succeeded" and a gnoweb link for a package that is not callable and may never be.
- Every read (`vm/qrender`, `vm/qeval`, `vm/qfuncs`, `vm/qfile`, `vm/qpaths`) answers a parked path exactly like a path never submitted, and a call into one fails with an internal error. An agent reading back its own deploy concludes the deploy vanished.
- Parking skips the type check, so the simulate-before-broadcast step that catches authoring bugs at zero cost on a permissionless chain reports success for ill-typed code.

The chain exposes one query that tells the states apart, `vm/qpkgmeta_json` (`status: live | inert | absent`, a `pending` flag, and the reason a parked package is not live), and it answers on every live chain.

onyx also restricts `MsgRun` to the addresses in `params/vm:p:run_submitters`, and the ante handler's refusal reaches gnomcp as a bare `unauthorized error`.

## Decision

**`gno_addpkg` reads the policy and, on an inert chain, waits for the approver.** The policy is read before anything is signed; failing to read it fails the tool. After a broadcast on an inert chain the tool polls `vm/qpkgmeta_json` every 2s for up to 30s. 30s covers the approver's latency plus its ten-second verify budget, with slack.

The result reports `package_status`:

- `live`: an ordinary success, with the gnoweb link when the profile has a gnoweb host.
- `inert`: PARKED, with the chain's reason in an untrusted envelope, the recovery, and no link.
- `redeploy_parked`: a redeploy parked over a live private realm; the result says the previous version still serves reads and calls.
- `unknown`: the chain reported the package as neither live nor parked, or never answered. Never reported as live.

For every status but `live`, the recovery also goes into the structured field `next_steps`: Claude Code hands the model a successful result's structured content without its text. Audit records distinguish `parked` and `status_unknown` from `ok`. On any other chain the only addition is the policy read; nothing is polled after the broadcast.

**A simulation on an inert chain says it did not type-check the code.**

**Reads and calls on a parked path say so.** `gno_render`, `gno_eval`, `gno_read` and `gno_call` ask `vm/qpkgmeta_json` only when their chain operation fails, and return `package_parked` (reason, plus the recovery addressed to the package's deployer) when the path is `inert`. Any other status, or a failed status query, leaves the original error untouched, and a typed tool error passes through without a query. The helper lives in `internal/tools/parked`, shared by the read and write tool packages.

**`gno_run` checks `run_submitters` before signing.** When the list is non-empty and does not name the message's caller (the agent key, or the session's master), the tool returns `run_not_allowed` without signing. The check runs through a `precheck` hook on the write dispatcher, ahead of the unfunded-account check, so an agent is never sent to the faucet for a write the chain refuses, and the refusal is audited as a denial rather than a broadcast error.

The recovery text is one constant shared by the deploy result and the read hint: lint against the chain's release, check the key can pay the storage deposit (charged when the package is enabled), redeploy to the same path with the same key. The chain accepts a resubmission from the original submitter and refuses one from anyone else.

## Alternatives considered

**Return immediately after the broadcast and let the agent poll.** Rejected: every deploy on the default testnet would need a second round-trip to learn what almost always resolves within seconds, and no read tells parked from absent.

**Match the chain's error strings to detect a parked path.** Rejected: a parked path answers with the same error types as an absent one, and simulation drops the deliver-tx log where a distinguishing message would sit. `vm/qpkgmeta_json` answers the question directly.

**Type-check locally before an inert deploy.** Rejected: resolving imports needs the chain's copy of each dependency, which is what the approver oracle already does; gnomcp would duplicate it and still disagree with the chain whenever the two drift. The simulate result and the skills point the agent at the chain-matched toolchain instead.

**Detect a refused `MsgRun` from its error.** Rejected: the simulate path drops the deliver-tx log naming `run_submitters`, leaving a bare `unauthorized error` that also covers other refusals. Reading the param is one query and cannot be ambiguous.

## Consequences

- gnomcp never links a package that is not live, and reports a parked path as parked.
- Every `gno_addpkg` call reads one param, and every `gno_run` call another; a failed read stops the tool before it signs.
- The wait blocks the tool call for up to 30s when the approver does not enable the package.
- gnomcp cannot say why an approver refused a package: the chain reports the same reason for a refused package as for one still waiting. The recovery covers both.
