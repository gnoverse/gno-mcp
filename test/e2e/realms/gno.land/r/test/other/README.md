# r/test/other

Minimal ping realm used by the gnomcp e2e harness.

Exports one function: `Ping() string` — returns `"pong"`.
A second realm for scope-mismatch / multi-session checks. Seeded into the simnet
(and the integration node) at genesis — no manual deploy.
