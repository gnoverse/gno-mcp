---
id: namereg
tier: external
category: writes
image: l2-gnomcp
timeout-minutes: 20
covers: [write.namereg, write.addpkg, write.address-namespace, external.test13]
---
# Register a name and deploy under it on test13

Tests that the agent can register a human-readable name via gno_namereg and then
deploy a realm under that name. The registered name becomes a permanent namespace on
the chain that the agent (or the user via session) owns.

Driver note: name registration is irreversible and costs real test13 coin (~200M ugnot).
The run must use a fresh key per run (key named `run-$RUN_ID`) to avoid name collisions
across runs. Names must match `^[a-z][a-z0-9_]*$` (no hyphens — gnoweb rejects them).
Generate a unique name per run by appending a short numeric suffix, e.g. `drv$RUN_ID`.

## Step 1: own identity, funded
### Instruct
I want to register a custom name on the testnet and deploy under it. First, set yourself
up with a key named `run-$RUN_ID` and make sure you have enough GNOT to cover registration
(at least 200M ugnot). Tell me your address and balance.
### Expect
- correctness: a fresh g1… address, NOT test1.
- correctness: balance >= 200M ugnot confirmed by gno_account.
- tool-selection: gno_key_generate, gno_faucet_fund, gno_account used.
### Verify
- Turn log: gno_key_generate + gno_faucet_fund + gno_account calls visible.
- blocked (not fail) if live faucet is unreachable.

## Step 2: register a name
### Instruct
Register the name `drv$RUN_ID` for yourself so you can deploy under gno.land/r/drv$RUN_ID/.
### Expect
- correctness: name registration succeeds (tx hash, no error).
- correctness: the agent uses gno_namereg (the dedicated tool), not a raw gno_call to r/sys/namereg/v1.
- correctness: the agent reports the resulting namespace (gno.land/r/drv$RUN_ID/).
### Verify
- Turn log: a gno_namereg tool_use with `.input.name` == `drv$RUN_ID` whose output contains the registered namespace.
- blocked (not fail) if test13 is unreachable.

## Step 3: deploy under the registered name
### Instruct
Now deploy a small realm under your new name — a simple counter at gno.land/r/drv$RUN_ID/counter.
Anyone should be able to bump it and read the current count.
### Expect
- correctness: deploy path is gno.land/r/drv$RUN_ID/counter (the registered name, not an address).
- correctness: deploy succeeds; a bump call confirms the count at 1.
- correctness: CLA is signed before the deploy (either via gno_cla_sign or gno_call to r/sys/cla).
### Verify
- Turn log: gno_addpkg with deploy_path = gno.land/r/drv$RUN_ID/counter.
- Turn log: gno_call bump + a gno_render (or gno_eval) confirming count == 1.
- blocked (not fail) if test13 is unreachable.

## Debrief
- What was the difference between deploying under your address namespace versus the registered name?
- Did the name registration tool prompt you about cost before you confirmed? What did it show?
- Would you use address-first or namereg for a permanent project?
