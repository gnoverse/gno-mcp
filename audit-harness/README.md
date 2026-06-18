# Audit harness

This directory holds small, sanitized audit-loop fixtures and expectations.
It is for evaluating agent-facing audit rules before any stable public guide or
example moves upstream into `gnolang/gno`.

Run the current slice:

```sh
go run ./cmd/auditharness ./audit-harness/expected/*.yaml
```

If `gno` is not on `PATH`, pass it explicitly:

```sh
go run ./cmd/auditharness -gno-bin /path/to/gno ./audit-harness/expected/current-guard.yaml
```

Some development builds of `gno` need `GNOROOT` set so stdlibs resolve:

```sh
GNOROOT=/path/to/gnolang/gno \
  go run ./cmd/auditharness -gno-bin /path/to/gno ./audit-harness/expected/*.yaml
```

Emit JSON instead of Markdown:

```sh
go run ./cmd/auditharness -format json ./audit-harness/expected/*.yaml
```

## Expected record format

Each `expected/*.yaml` file describes one finding family and the fixtures that
exercise it.

```yaml
id: current-guard
title: cur.Previous without cur.IsCurrent
rule: current_guard
fixtures:
  - name: vulnerable
    path: ../fixtures/current-guard/vulnerable
    want_gno_test: pass
    want_pattern_hits: 1
  - name: fixed
    path: ../fixtures/current-guard/fixed
    want_gno_test: pass
    want_pattern_hits: 0
```

Paths are relative to the YAML file. `want_gno_test` is currently `pass` or
`fail`. `want_pattern_hits` is the exact count of source locations expected from
the rule.

## Adding a slice

1. Add sanitized fixtures under `fixtures/<slice>/`.
2. Add an `expected/<slice>.yaml` record with one or more fixture entries.
3. Teach `internal/auditharness` the new `rule` value.
4. Run the command above and commit the generated report only if it is useful
   review material; the YAML and fixtures are the durable test inputs.
