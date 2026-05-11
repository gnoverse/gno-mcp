# gno-mcp

> MCP server + Claude skills for [gno.land](https://gno.land).

`gno-mcp` exposes gno.land as a [Model Context Protocol](https://modelcontextprotocol.io) server: read realms, evaluate expressions, manage testnet keys, simulate and broadcast transactions — all from any MCP-aware client (Claude Code, Claude Desktop, Cursor, etc.). The repo also ships four Claude skills that drive the tools through the common workflows: onboarding, exploring a realm, reading a contract, debugging a failed tx.

---

> [!WARNING]
> **Work in progress. Unaudited. Pre-release.**
>
> - **No security review.** The threat model exists on paper (see [docs/security.md](docs/security.md)) but no third party has audited the code, the prompts, or the skill flows.
> - **API surface is unstable.** Tool names, argument shapes, and error codes may change before v0.1 is tagged.
> - **Mainnet is gated for a reason.** Writes against `gno.land` require explicit `confirm=true`. Do not disable that gate. Do not point this at a wallet holding real funds you cannot afford to lose.
> - **The real gnopie client is not wired yet.** Until [gnolang/gno#5444](https://github.com/gnolang/gno/pull/5444) tags a release, `gno-mcp` runs against an in-memory fake — tools work end-to-end over MCP, but they are not actually talking to a chain.
> - **No upgrade path guaranteed.** Audit log shape, config schema, and the `GnopieClient` interface will move; expect breakage between pre-1.0 tags.
>
> Use it on testnet. Read the security doc. File issues when something looks off.

---

## Why this exists

LLMs are starting to drive real on-chain actions, but the bridge between an LLM and a live chain is where most things go wrong: leaked seeds, prompt-injected memos, transactions broadcast on the wrong network, errors that read like noise. The MCP ecosystem solves the "how does the model call a tool" half, but the "what does the tool look like, how is it bounded, and how do we keep the model on rails" half is per-domain.

`gno-mcp` is that bridge for gno.land. It ships **two coupled artifacts** so the bridge is not just a tool surface but a usable one:

1. **An MCP server** with 16 tools that expose gno operations under a single security spine — untrusted-content envelopes on every external byte, mainnet write-gate on every state-changer, audit log on every call, mnemonics never observed.
2. **A skills pack** — four Claude skills that drive the tools through the workflows users actually have (onboard, explore, read, debug-tx). Skills are first-class shipped artifacts, not afterthoughts; they encode the security posture for the LLM in language the LLM can route on.

## How it is shaped

A few decisions worth knowing if you are reading the code:

- **Wrap `gnopie`'s Go internals as a library, not a subprocess.** Typed errors, no shell-injection surface, faster cold path. The seam is `internal/client.GnopieClient`, so the real client and an in-memory fake are interchangeable. v0.1 ships against the fake; Task 22 swaps in the real one once gnopie tags.
- **Security is a property of the package, not a checklist.** Every tool inherits the envelope (`mcp.UntrustedEnvelope`), the structured-error vocabulary (`onboarding_required`, `confirmation_required`, `mainnet_write_blocked`, `not_implemented`, `invalid_argument`), the audit log (`audit.Log`), and the mainnet-confirm gate by construction. New tools can't accidentally skip them.
- **Output budgeting is mandatory.** Read-shaped tools (`gno_get`, `gno_read`) apply a 4 KB budget by default. Over-budget responses return a `summary` pointing at gnoweb instead of a chopped half-source. Explicit slice requests (symbol/file/lines) bypass the budget.
- **Skills are markdown, not code.** Claude's skill router reads frontmatter; no build step, no DSL. Authoring conventions live in [docs/skills.md](docs/skills.md).
- **Tests use an in-process MCP client, not stdio.** `internal/mcp/testmcp` wires `mcp-go`'s `NewInProcessClient` against a fake `GnopieClient`. Tests exercise the full JSON-RPC layer (initialize → list → call) with no subprocess and no port allocation.
- **No mnemonic, ever.** `gno_keygen` returns `{name, address, pubkey}` only. The key lives in `gnokey`. Backup workflows route through `gnokey export`. The audit log redacts `mnemonic`, `password`, `private_key` even if a tool accidentally accepts them.

## Status

v0.1 implementation complete pending upstream gnopie. 16 MCP tools shipping, `-race` clean across 7 packages, end-to-end smoke green over stdio. See [docs/tools.md](docs/tools.md) for the tool surface.

## Install

    go install github.com/gnolang/gno-mcp/cmd/gno-mcp@latest

Release binaries (linux/darwin × amd64/arm64) are published via [goreleaser](.goreleaser.yaml) once tagged.

## Configure

Copy `.mcp.json.example` into your MCP client config:

```json
{
  "mcpServers": {
    "gno": {
      "command": "gno-mcp",
      "args": []
    }
  }
}
```

Per-user state lives at:

- Audit log: `~/.gno-mcp/audit.jsonl` (mode `0600`)
- Config: `$XDG_CONFIG_HOME/gno-mcp/config.json` (`GNO_MCP_CONFIG` overrides)

## Skills

The `skills/` directory is a Claude plugin package containing four skills:

- **gno-onboarding** — first-time testnet setup, no mnemonic shown
- **gno-explore-realm** — point-paste a realm path, get a structured tour
- **gno-read-contract** — read source in slices, surface invariants and risk
- **gno-debug-tx** — classify a failed tx and propose a concrete fix

See [docs/skills.md](docs/skills.md).

## Tools

16 MCP tools: `gno_network_info`, `gno_get`, `gno_eval`, `gno_read`, `gno_inspect`, `gno_address_info`, `gno_keygen`, `gno_faucet_request`, `gno_call`, `gno_run`, `gno_session_{create,revoke,list}` (stubs), `gno_config_{get,set}`, `gno_audit_tail`. See [docs/tools.md](docs/tools.md).

## Security posture

- **Never displays or asks for a mnemonic.** `gno_keygen` returns only `{name, address, pubkey}`. The mnemonic stays in `gnokey`.
- **Mainnet writes gated.** `gno_call` and `gno_run` always simulate first. On `gno.land` the broadcast is suppressed unless `confirm=true` is set; the response carries `confirmation_required: true` so the caller can echo the security block to the user.
- **Faucet refuses mainnet.** `gno_faucet_request` returns a `mainnet_write_blocked` structured error if pointed at `gno.land`.
- **Untrusted-content envelope.** Every tool that returns external bytes (Render output, source, eval results) wraps them in `<untrusted_content kind="…" source="…">…</untrusted_content>` so the LLM treats them as data, not instructions.
- **Audit log on every write.** JSONL append-only, args redacted (`mnemonic`, `password`, `private_key` stripped). Tail via `gno_audit_tail`.

See [docs/security.md](docs/security.md) for the full posture.

## Development

```sh
make test    # go test -race ./...
make e2e     # build + stdio smoke (asserts every v0.1 tool is registered)
make lint    # vet + gofmt -l -d
make install # go install ./cmd/gno-mcp
```

The repo currently uses an in-memory fake `GnopieClient`. Wiring the real gnopie client (Task 22) requires a local gno checkout and a `replace` directive — instructions will land alongside the upstream release.

## Roadmap

- **v0.1**: read tools + write flow + skills (this milestone)
- **v0.2**: real gnopie wiring, session keys (`gno_session_*`), tx-indexer integration (history/search), skill transcript snapshots
- **v0.3**: streaming Render, multi-network resolution, persisted session scopes
- **post-v1.0**: external security audit before any "stable" claim

## License

Apache-2.0. See [LICENSE](LICENSE).
