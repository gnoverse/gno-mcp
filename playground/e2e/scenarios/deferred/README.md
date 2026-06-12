# Deferred scenarios

Scenarios here are excluded from every tier sweep (the driver only reads
`local/` and `external/`). Move a file back under its tier directory to
re-enable it.

- `08-local-gnodev.md` — blocked on an upstream gnodev fix (package-tree
  loading is not usable enough for the flow this scenario tests).
