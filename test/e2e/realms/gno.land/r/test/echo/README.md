# r/test/echo

Minimal echo realm used by the gnomcp e2e harness.

Exports one function: `Echo(msg string) string` — returns msg unchanged.
Exercises gno_call args-encoding round-trip in the write-tools scenario. Seeded into
the simnet (and the integration node) at genesis — no manual deploy.
