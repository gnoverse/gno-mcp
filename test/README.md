# How testing works here

Four ways to test gnomcp, by purpose. The first three are automated; the fourth is how
you poke it by hand.

| Layer | Command | What it covers |
|---|---|---|
| **unit** | `go test ./...` | logic against an in-memory fake chain (`internal/chain.Fake`); session/keystore encryption-at-rest; argument and error mapping. Fast, no node. |
| **integration** | `make test-integration` | the MCP tools against a real in-process gno.land node (gnoclient-signed): reads, agent-key writes, faucet dispense, profile-add (incl. unreachable / chain-id-mismatch). Build-tagged `integration`; CI vet-compiles them. |
| **agent e2e** | `make playground-e2e` | the *agent experience*: does the LLM pick the right tool, does the gno skill route, does the auditor agent dispatch, does it recover from errors — plus the full gnokey session lifecycle (the driver plays the user). Non-deterministic (LLM-driven); gated locally. See `playground/`. |
| **manual / exploratory** | `make playground-fresh` or `make playground-sim`, then run `claude` | a human in a clean container: a from-scratch skill+gnomcp install, or eyeballing a flow end to end. Ad-hoc — not a suite, not gated. |

## No overlap
Each behaviour is tested once, at the layer that fits it. Session *logic* is unit (fake
chain); the session *handshake* with a real `gnokey` is the playground; tool↔chain
*correctness* is integration. If you find the same assertion in two layers, that's a bug
in the test architecture — collapse it.

## The playground
`playground/` is the agent-e2e harness: a driver Claude (host) QAs a containerized
Claude+gnomcp+gno-skill scenario by scenario, verifying chain ground truth against an
in-container simnet. See `playground/README.md`. Coverage ledger:
`playground/e2e/COVERAGE.md`.
