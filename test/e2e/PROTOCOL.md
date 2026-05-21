# gnomcp Milestone B — E2E Protocol

Manual checklist. Run sequentially. Record every result in the runlog
at `.mynote/v2-issues/YYYY-MM-DD-milestone-b-section-a-runlog.md` (Section A)
and `.mynote/v2-issues/YYYY-MM-DD-milestone-b-section-b-runlog.md` (Section B).

---

## Pre-flight

- [ ] `./test/e2e/setup.sh` exits with the ready banner (port, master address, mnemonic printed).
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

### Check 1 — simulate=true bypasses session check

Skip if simulate did not ship (gnoclient has no simulate primitive — see Task 2.3 finding).

```
gno_call(profile=local, realm=gno.land/r/test/counter, func=Increment, simulate=true)
```

Pass: text contains `Simulated: true`; no error.
Confirm `gno_eval Total()` still returns `(0 int)`.

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
gno_session_propose(profile=local, allow_paths=["gno.land/r/test/counter"])
```

Pass:
- Text contains fenced `gnokey maketx session create ...` with `--pubkey gpub1...` and `--allow-paths`.
- Session file appears at `~/.local/share/gnomcp/sessions/local/<session_addr>.key`, mode `0600`.

### Check 5 — User signs and session activates

Copy the command from Check 4. Replace `<your-master-key-name>` with `e2e-master`.
Run with `GNOKEYHOME=test/e2e/.keyring gnokey ...`.

Pass: gnokey reports tx success.
`gno_auth_status(profile=local)` shows `[active]` for the session address.

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
gno_session_propose(profile=local, allow_paths=["gno.land/r/test/counter"], spend_limit="200gnot")
```

Pass:
- Proposal succeeds (no error).
- Text includes clamp warning: e.g. `"WARNING: requested spend_limit 200gnot exceeds local cap of 100gnot; clamped to 100gnot."`.
- The `gnokey` command in the text shows `100gnot` (the clamped value).

### Check 11 — gno_run broadcasts ad-hoc script

```
gno_run(profile=local, code="package main\nfunc main() { println(\"hi from run\") }")
```

Pass: result has `tx_hash`; `Output` contains `hi from run`.

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
