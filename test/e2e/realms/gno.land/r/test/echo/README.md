# r/test/echo

Minimal echo realm used by the gnomcp Milestone B e2e protocol.

Exports one function: `Echo(msg string) string` — returns msg unchanged.
Used in Check 7 (gno_call args encoding round-trip).
Deploy with setup.sh before running PROTOCOL.md.
