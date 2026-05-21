# gnomcp Milestone B — E2E Protocol

Manual checklist. Run sequentially. Record every result in the runlog
at `.mynote/v2-issues/YYYY-MM-DD-milestone-b-section-a-runlog.md` (Section A)
and `.mynote/v2-issues/YYYY-MM-DD-milestone-b-section-b-runlog.md` (Section B).

---

## Pre-flight

- [ ] `./test/e2e/setup.sh` exits with the ready banner (port, master address, mnemonic printed).
- [ ] **Master address sanity check.** Confirm `master-address` in `test/e2e/profiles.toml` matches the `master addr:` line printed by `setup.sh`. If they differ, edit `profiles.toml` to match. gnomcp uses this as `MsgCall.Caller`; a mismatch makes every session-signed write fail.
- [ ] `make build` produces `bin/gnomcp` without error.
- [ ] `bin/gnomcp --config test/e2e/profiles.toml` starts and stays running (second terminal).

Note on indexer: `setup.sh --with-indexer` is not wired in Milestone B. The
`--with-indexer` flag currently prints "indexer not wired yet" and continues.
Check A7 documents the expected absent behavior for that configuration.

---

## Section A — Milestone A regression (read tools)

Run these before Section B. If any fails, fix the read-tool break before
proceeding — the write-tool flow depends on gnodev being healthy.

### A1 — gno_render

```
gno_render(profile=local, realm=gno.land/r/test/counter, path="")
```

Pass: result is MCP `resource`, MIME `text/markdown`, URI `gno://gno.land/r/test/counter`, body non-empty.

### A2 — gno_eval

```
gno_eval(profile=local, realm=gno.land/r/test/counter, expr="Total()")
```

Pass: result text is `(0 int)` (or the current state if the protocol has run before).

### A3 — gno_read (file)

```
gno_read(profile=local, realm=gno.land/r/test/counter, file="counter.gno")
```

Pass: result is MCP `resource`, MIME `text/x-gno`, body contains `package counter` and the `Increment` function.

### A4 — gno_read (list)

```
gno_read(profile=local, realm=gno.land/r/test/counter)
```

Pass: result MIME `text/plain`, body is newline-separated file names including `counter.gno`.

### A5 — gno_inspect (exports present)

```
gno_inspect(profile=local, realm=gno.land/r/test/counter)
```

Pass: result text contains `Increment` and `Total`.

### A6 — gno_inspect (no-doc realm)

```
gno_inspect(profile=local, realm=gno.land/r/test/other)
```

Pass: result text non-empty (even if only the package declaration).

### A7 — Indexer tools (conditional)

`test/e2e/profiles.toml` does NOT include `tx-indexer-url`; indexer tools
are absent from gnomcp at runtime.

Pass: `initialize` response (or `tools/list`) does NOT include `gno_list`,
`gno_history`, or `gno_activity`.

If the operator ran `setup.sh --with-indexer` (out-of-scope for default Milestone B run):
- `gno_history(realm=gno.land/r/test/counter)` returns at least the AddPackage event.
- `gno_activity(realm=gno.land/r/test/counter)` returns only MsgCall events.
- `gno_activity` with `since != nil` returns `error_unavailable`.
- `gno_list` returns "realms query not supported" error.

---

## Section B — Milestone B feature checks (writes + sessions)

Pre-flight: A1–A6 all pass. `gno_eval Total()` returns the expected baseline.

### Check 1 — authentication_required on simulate without session

`simulate=true` still requires an active session under the chain-bounded session
model (the ante handler validates the same way for simulate and broadcast).

```
gno_call(profile=local, realm=gno.land/r/test/counter, func=Increment, simulate=true)
```

Pass: `isError`, `code=authentication_required`. Confirm `gno_eval Total()` is unchanged.

### Check 2 — authentication_required on real call without session

```
gno_call(profile=local, realm=gno.land/r/test/counter, func=Increment)
```

Pass: `isError`, `code=authentication_required`, `next_action=gno_session_propose`.

### Check 3 — dangerous_disabled on non-dangerous profile

```
gno_call(profile=local-safe, realm=gno.land/r/test/counter, func=Increment)
```

Pass: `isError`, `code=dangerous_disabled`, text mentions editing `profiles.toml`.

### Check 4 — Session propose returns a runnable gnokey command

```
gno_session_propose(profile=local, allow_paths=["gno.land/r/test/counter"], spend_limit="100000000ugnot")
```

**Pass `spend_limit="100000000ugnot"` (100M).** The chain debits the tx's `GasFee` against the session's spend limit, and gnomcp's default `GasFee` is `10000000ugnot`. The hardcoded scope default `100000ugnot` is below that and would reject every broadcast as "session not allowed."

Pass:
- Text contains fenced `gnokey maketx session create ...` with `--pubkey gpub1...` and `--allow-paths vm/exec:gno.land/r/test/counter`.
- Session file appears at `~/.local/share/gnomcp/sessions/local/<session_addr>.key`, mode `0600`.

### Check 4b — Realm functions need `cur realm` for MsgCall

Realms invoked by `gno_call` must declare an explicit `cur realm` first parameter (e.g. `func Increment(cur realm) int`). The chain refuses MsgCall against non-crossing functions with `function X is non-crossing and cannot be called with MsgCall; query with vm/qeval or use MsgRun`. The bundled `test/e2e/realms/*.gno` files conform; if you add your own realm to test session-signed writes, ensure every callable function follows this signature.

### Check 5 — User signs and session activates

Copy the command from Check 4. Replace `<your-master-key-name>` with `e2e-master`.
Append `-home test/e2e/.keyring` and the remote/chainid flags so the command runs
against the test gnodev:

```
gnokey -home test/e2e/.keyring maketx session create \
  --pubkey gpub1... --allow-paths gno.land/r/test/counter \
  --spend-limit 100000ugnot --expires-at <unix-ts> \
  --gas-fee 10000000ugnot --gas-wanted 10000000 \
  --remote http://127.0.0.1:<PORT> --chainid dev \
  --insecure-password-stdin --broadcast \
  e2e-master
```

(Send an empty password via `printf '\n' | gnokey ...` — the e2e master key has no passphrase.)

Pass: gnokey reports tx success.
`gno_auth_status(profile=local)` shows `[active]` for the session address.

### Check 5b — Simulate with session now succeeds

Repeat Check 1 now that a session is active:

```
gno_call(profile=local, realm=gno.land/r/test/counter, func=Increment, simulate=true)
```

Pass: text contains `Simulated: true`; no error. `gno_eval Total()` is unchanged
(simulate must not mutate state).

### Check 6 — Authorized write succeeds and mutates chain

```
gno_call(profile=local, realm=gno.land/r/test/counter, func=Increment)
```

Pass:
- Result has `tx_hash`; `Simulated` absent or false.
- `gno_eval Total()` returns `(1 int)`.
- `gnomcp audit tail` shows entry with `tool=gno_call, profile=local, session_address=g1..., result=ok, duration_ms=<int>`.

### Check 7 — Args encoding (stringified array)

```
gno_call(profile=local, realm=gno.land/r/test/echo, func=Echo, args=["hello world"])
```

Pass: result text contains the echoed string `hello world`.

### Check 8 — Scope mismatch

```
gno_call(profile=local, realm=gno.land/r/test/other, func=Ping)
```

Pass: `isError`, `code=scope_mismatch`, `available_paths=[gno.land/r/test/counter]`.

### Check 9 — Second session coexists; two active sessions

Propose + sign for `gno.land/r/test/other` (same steps as Check 4–5, different allow_paths).

`gno_auth_status(profile=local)` lists two active sessions.

```
gno_call(profile=local, realm=gno.land/r/test/other, func=Ping)
```

Pass: call succeeds via new session.

```
gno_call(profile=local, realm=gno.land/r/test/counter, func=Increment)
```

Pass: call succeeds via original session.

### Check 10 — Hard-limit clamp + warning

```
gno_session_propose(profile=local, allow_paths=["gno.land/r/test/counter"], spend_limit="200000000ugnot")
```

(Use `ugnot` to match the local cap's denomination — gnomcp's `clampCoin` rejects cross-denomination compares, so a `gnot` request against a `ugnot` cap errors instead of warning. Tracked as a separate scope-policy decision.)

Pass:
- Proposal succeeds (no error).
- Text includes clamp warning: e.g. `"WARNING: requested spend_limit 200000000ugnot exceeds local cap of 100000000ugnot; clamped to 100000000ugnot."`.
- The `gnokey` command in the text shows `100000000ugnot` (the clamped value).

### Check 11 — gno_run broadcasts ad-hoc script

**Deferred.** MsgRun requires a different chain permission (`vm/run`, no realm path) than gno_session_propose currently emits — propose only outputs `vm/exec:<realm>` entries. Wiring gno_run end-to-end needs propose to also support `vm/run` permissions or a separate `gno_session_propose_run` flow. Out of scope for the per-profile-master MVP; revisit alongside MsgRun's permission model.

For now, gno_run against Real fails with `session not allowed error` at the ante handler.

### Check 12 — Revoke flow

```
gno_session_revoke(profile=local, session_address=<original g1...>)
```

Pass: text contains fenced `gnokey maketx session revoke ...`.

Run the revoke command with `GNOKEYHOME=test/e2e/.keyring`.

`gno_auth_status` no longer lists the revoked session (or marks `[revoked]` then GCs).

Next `gno_call` against `gno.land/r/test/counter`:
- Succeeds via the wider session from Check 9 if it covers counter, OR
- Returns `authentication_required`.

### Check 13 — Encryption-at-rest round-trip

1. Stop gnomcp.
2. Restart with `GNOMCP_SESSION_PASSPHRASE=test-pass` + `--config test/e2e/profiles.toml`.
3. Propose + sign a new session.
4. Stop gnomcp.
5. `cat` the session file: confirm `privkey` field is not plain base64 (should be AES-GCM ciphertext).
6. Restart with `GNOMCP_SESSION_PASSPHRASE=test-pass`: `gno_auth_status` shows session active.
7. Restart with wrong passphrase (`GNOMCP_SESSION_PASSPHRASE=wrong`): `gno_auth_status` shows no active sessions; stderr contains `"could not decrypt session file"`.

Pass: all three sub-steps hold.

### Check 14 — Teardown clean

```
./test/e2e/teardown.sh
```

Pass: gnodev process exits; temp directories removed; no leftover gnomcp or gnodev processes (`ps aux | grep gnodev` empty).

---

## What this protocol is NOT

- Not run by `go test`. Not in CI.
- Not a substitute for unit + integration tests.
- Not a soak test. One pass per check, sequentially.
- Does not simulate fault injection (covered by unit tests with `chain.Fake`).
