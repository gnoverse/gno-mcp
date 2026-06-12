# r/test/counter

Stateful counter realm used by the gnomcp e2e harness.

Exports:
- `Increment() int` — adds 1 to the running total; returns new value.
- `Total() int` — returns current total without mutation.
- `Render(path string) string` — returns `# Counter\n\nTotal: N` (markdown).

`Render` is intentionally present so the read-tool scenario can exercise
`gno_render` against a realm with a real Render function. The realm is seeded
into the simnet (and the integration node) at genesis — no manual deploy.
