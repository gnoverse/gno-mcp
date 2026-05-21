# r/test/counter

Stateful counter realm used by the gnomcp Milestone B e2e protocol.

Exports:
- `Increment() int` — adds 1 to the running total; returns new value.
- `Total() int` — returns current total without mutation.
- `Render(path string) string` — returns `# Counter\n\nTotal: N` (markdown).

`Render` is intentionally present so Section A1 of PROTOCOL.md can exercise
`gno_render` against a realm with a real Render function, without needing a
separate realm. Deploy with setup.sh before running PROTOCOL.md.
