# Milestone B — E2E Protocol

Run after `test/e2e/setup.sh` has gnodev + the three test realms ready, and after `make build` has produced `bin/gnomcp`.

## Pre-flight

- [ ] setup.sh banner clean, three realms deployed.
- [ ] `make build` succeeded; `bin/gnomcp` exists.
- [ ] `bin/gnomcp --config test/e2e/profiles.toml` starts cleanly (no startup errors in stderr).

## Section A — Milestone A regression checks

If any of A1–A6 fails, fix the read-tool break before proceeding to Section B.

(Section A1–A7 — filled in during Phase 7 Task 7.1.)

## Section B — Milestone B feature checks

Pre-flight for Section B: A1–A6 all pass; `gno_eval Total()` returns the expected baseline value.

(Check 1–14 — filled in during Phase 7 Task 7.1.)

## Teardown

- [ ] Run `test/e2e/teardown.sh`.
- [ ] Confirm: no gnodev process in `ps aux | grep gnodev`.
- [ ] Confirm: `test/e2e/.keyring/` removed.
