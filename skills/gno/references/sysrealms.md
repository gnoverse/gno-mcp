# gno.land system realms

> **Category: gno.land platform.** The system realms that ship at genesis and the chain-level
> wiring around them (names, the validator set, chain params, the CLA gate). This reference teaches
> the *durable design*. It deliberately states almost no concrete values — for "what's actually true
> on chain X right now," you query the live chain. Reciting per-network specifics from memory is the
> one way to be confidently wrong here.

## The model — four invariants

These hold across every gno.land network and don't go stale. Everything concrete sits behind a query.

**I1 · Source ≠ master ≠ live.** Three states that genuinely disagree. The realm source on a branch,
the merged `master`, and what's *deployed and running* on a given chain are three different things.
A sys realm on a long-lived testnet often runs code that predates current `master` — a function
`master` removed may still be live, and a default `master` changed may still hold the old value. For
"what is true on chain X now," there is exactly one authority: chain X. Never answer a chain-state
question from source.

**I2 · Genesis activation.** The sys realm *code* is identical on every network. Behavior diverges
entirely from which **genesis transactions** were injected at chain birth — realms ship *dormant* and
a genesis tx flips them on (namespace enforcement enabled, a username set seeded, a concrete validator
set injected, chain params set). So "is namespace enforcement on?", "which usernames exist?", "who are
the validators?" are **per-network, answerable only live** — never from source. The same `r/sys` code
behaves differently on local gnodev, on a public testnet, and on betanet, by design.

**I3 · Controller architecture (names/users).** Name registration is layered. `r/sys/users` is the
durable canonical store (name↔address + a confusable-collision index). Policy lives in *controller*
realms (e.g. `r/sys/namereg/v1`) that enforce format/price/blacklist and then write through the store.
`r/sys/names` is a read-only verifier the chain consults to gate deploys (it reads the store). Policy
is swappable and versioned; the store is durable; **GovDAO can bypass any controller.** Which
controller (if any) is live, and what policy it enforces, is per-network — query it.

**I4 · Single-writer gates + GovDAO trust root.** Privileged sys state is guarded by exactly one
writer each: `r/sys/params` is the only realm allowed to set native chain params; `r/sys/validators/v3`
is the only writer of the validator set; the committed valset record is chain-only. Above all of them,
**GovDAO is the trust root** — it can pause, override, or bypass. When you reason about "who can change
this," the answer is the single privileged writer, then GovDAO.

## The discipline — trust the chain

For any concrete value — a name format, a price, whether enforcement is on, the validator set, a
param — **do not recite it from memory or source. Query the chain.** Confirm *which* chain first
(state read against the wrong chain is just a mislabeled guess — same failure as an audit reading the
wrong deployment).

| What you need | gno-mcp (recommended) | Fallback (no MCP) |
|---|---|---|
| **Confirm which chain you're on (do this first)** | `gno_status` (flags chain-id mismatch) | RPC `/status` → `result.node_info.network` |
| Discover a realm's live API + whether it's even deployed | `gno_packages` (namespace/prefix); `gno_eval` of a func | raw ABCI `vm/qfuncs` (data = pkgpath) |
| Read live state / evaluate an expression | `gno_eval` | `gnokey query vm/qeval --data "<pkgpath>.<expr>"`; raw ABCI `vm/qeval` |
| See `Render()` output (human view of state) | `gno_render` | gnoweb `/<pkgpath>`; raw ABCI `vm/qrender` (data = `pkgpath:renderpath`) |

Never block on the MCP — it's an accelerator, not a dependency (see `mcp.md`). Raw ABCI recipe:

```bash
RPC=https://rpc.test13.testnets.gno.land:443
hex=$(printf '%s' 'gno.land/r/sys/names.IsEnabled()' | xxd -p | tr -d '\n')
curl -s "$RPC/abci_query?path=%22vm/qeval%22&data=0x$hex"   # value = base64 at result.response.ResponseBase.Data
# paths: vm/qeval (eval expr) · vm/qfuncs (list exported funcs, data=pkgpath) · vm/qrender (data=pkgpath:renderpath)
```

If a realm isn't deployed on the target chain, `qfuncs` returns nothing useful and `qeval` errors — that
absence is itself an answer (the feature isn't active on that network), not a tooling failure. But an
error on *one* path is not proof of absence: see versioning, next.

**Versioned realms — the path is part of the identity.** Several sys realms carry a version segment
(`r/sys/namereg/v1`, `r/sys/validators/v2` *and* `/v3`). Querying the unversioned path (`r/sys/validators`)
returns "not found" even when the feature is very much live at `/v3`. When versions coexist, **query the
specific version, and confirm which one the chain actually uses — don't assume the newest from source,
and don't conclude a feature is gone because the bare or older path is empty.** A wrong-version query
that returns nothing is the most common way to answer "doesn't exist" when the truth is "exists at /vN."

## The system realms — role (durable) + what to query for the live truth

`gno_packages` the `gno.land/r/sys/` prefix on the target chain to see which are actually deployed.

| Realm | Durable role | Query live for… |
|---|---|---|
| `r/sys/users` | Canonical name↔address store + collision index. Source of truth; holds no policy. | `IsNameTaken(n)`, `ResolveName(n)`, `ResolveAddress(a)`, `Controllers()` (which controllers it trusts) |
| `r/sys/namereg/v1` | A *controller*: public registration policy (format/price/blacklist) writing through `users`. May or may not be loaded per network; may be superseded by a later `/vN`. | `qfuncs` first (is it deployed?); then `ValidateNymFormat(s)`, `IsPaused()`, and `Render("")` for the price + rules |
| `r/sys/names` | Read-only verifier the chain consults to gate package deploys; reads `users`. | `IsEnabled()` (is namespace enforcement on at all?), `IsPaused()`, `IsAuthorizedAddressForNamespace(addr, ns)` |
| `r/sys/params` | GovDAO-facing **writer** of native chain params (sole privileged caller of native `sys/params`); exposes getters for only a handful (`GetValoperRegisterFee()`, valset getters). **Not** the read surface for arbitrary params. | its own getters for the few it exposes; **raw param values live in the keeper, not this realm** — read them via the param path (see "Reading a chain param value" below) |
| `sys/params` (native stdlib) | The Go-side params keeper `r/sys/params` writes through; frame-gated to that one realm. | Not a realm — observed indirectly via `r/sys/params` and the params query surface |
| `r/sys/validators/v3` | Current params-backed validator-set design (operator/signing model, trust-level + cooldown limits). | `GetValidators()`, `IsValidator(a)`, `GetTrustLevel()`, `GetCooldown()` — **report the live value; `master` may have changed since this chain deployed** |
| `r/sys/validators/v2` | Earlier version (PoA-based). Coexists with `/v3` on some chains. | `qfuncs`/`GetValidators()` — but **don't assume which version drives a given chain's set from source; confirm live which one holds the active valset** |
| `p/sys/validators` | Pure types/interface shared by the validator realms. No state. | — |
| `r/sys/cla` | Contributor License Agreement gate the chain consults before deploys. | `Render("")` (enabled? required hash? URL?), `HasValidSignature(addr)` |
| `r/sys/txfees` | Reserved fee-bucket realm; a stub today — real fee collectors are `auth`/`vm` params, not this realm. | `Render(cur)` (its balance); don't infer fee routing from it |
| `r/sys/rewards` | Reserved namespace for a future proof-of-contributions system; currently an empty stub. | `qfuncs` (expect ~no exports) — confirms it's still a placeholder |

## Deploying through the sys gates (namespace + CLA)

A package deploy (`addpkg`) must clear **two genesis-activated gates**, in this order. Both are
per-network (I2): off on a fresh local chain, on where genesis switched them on — so **query, don't
assume**, and prefer checking both *before* deploying rather than reading it off a failed tx.

1. **Namespace** (`r/sys/names`). Enforced when `IsEnabled()` is true. The signer must be authorized
   for the namespace segment of the path. Two ways to be authorized: deploy under your **own address**
   namespace (`r/<your-g1address>/*` is always authorized — no registration), or hold a registered name
   whose current owner is you (register via the live controller, e.g. `r/sys/namereg/v1`, if it's
   deployed). Check: `IsAuthorizedAddressForNamespace(addr, ns)`.
2. **CLA** (`r/sys/cla`). Enforced when a required hash is set. The signer must have signed the current
   agreement. Check: `HasValidSignature(addr)`. To clear it, **sign once from the same key**: read the
   required hash from `r/sys/cla`'s render (the `Required Hash` field), then call `Sign(hash)` with that
   value. `Sign` is an ordinary crossing call — the agent key signs it directly; no human or session needed.

Both gates reject with the **same error type** but distinct messages — tell them apart by the text:
`…not authorized to deploy packages to namespace` (fix: own-address path or register a name) vs
`…has not signed the required CLA` (fix: `Sign` the hash). Don't read "unauthorized" as one specific
cause; read the message. On a fresh local chain both gates are typically off and neither applies.

## Agent guidance — namespace defaults

Agents should deploy under their own address namespace (`r/<agent-g1address>/*`) by default. It
clears the namespace gate on every chain with no registration, no controller queries, no pricing (the
CLA gate, if enforced, still applies). Only register a name when the user explicitly requests it; names
are semi-permanent and agent behavior should not burn them without consent.

## Reading a chain param value

Raw chain params (fee collectors, storage price, halt height, restricted denoms) are **not** read
from `r/sys/params` — that realm only *writes* them (via GovDAO) and exposes getters for a handful.
The values live in the **params keeper**, keyed `<module>:<submodule>:<name>` where `<module>` is a
registered keeper (`vm`, `auth`, `bank`, `node`) and the submodule is usually `p`. Read one via the
params query path:

```bash
gnokey query params/auth:p:fee_collector -remote "$RPC"   # cleanest output
# raw ABCI: GET $RPC/abci_query?path="params/<module>:p:<name>"  → base64 value in the response
```

gno-mcp wraps realm getters via `gno_eval`, but may not expose a raw-param tool — use the
`params/<key>` path (gnokey/raw) for keeper params. Example keys (query inputs — get the value from
the chain, don't recite it): `auth:p:fee_collector` (gas fee collector), `vm:p:storage_fee_collector`
(storage-deposit collector — distinct from the gas one), `vm:p:storage_price`, `node:p:halt_height`,
`bank:p:restricted_denoms`.

## Worked example — "how do I register a name on test13?"

The model in action. Every concrete value comes from a live query; you explain the steps, the user
runs the funded tx.

1. **Confirm the chain.** `gno_status` (or RPC `/status`) → chain-id is `test-13`. Now reads are about
   the chain the user actually means.
2. **Is enforcement even on?** `gno_eval gno.land/r/sys/names.IsEnabled()`. If `false`, namespace
   enforcement is off on this network — anyone can already deploy under any `r/<name>/*` and registering
   a username isn't required to deploy (explain that, don't invent a registration flow). If `true`, continue.
3. **Which registration path is live?** `gno_packages gno.land/r/sys/` (or `qfuncs gno.land/r/sys/namereg/v1`).
   If `namereg/v1` isn't deployed on this chain, there's no open self-service tier here — names come from
   genesis seeding / GovDAO, and personal-address namespaces (`r/<your-g1address>/*`) still work. Say so.
4. **Read the real rules — live, not from memory.** If `namereg/v1` is present: `gno_render gno.land/r/sys/namereg/v1`
   (shows the format and price) and `gno_eval ...ValidateNymFormat("nym-alice123")` / `...IsPaused()`. Report the
   format and price you actually read — do not state `nym-<stem><digits>` or any price as fact without this step.
5. **Explain the steps the user runs.** With the real package path, func, and price in hand: the user signs a
   `maketx call -pkgpath gno.land/r/sys/namereg/v1 -func Register -args "<username>" -send "<price>ugnot"`
   from their own funded key (or via a gno-mcp session they authorize). Execution is theirs; you don't broadcast it.

The point: whether `namereg/v1` exists, whether enforcement is on, the exact format and price — all of it
came from querying test13, not from this file.

## See also

- `mcp.md` — the gno-mcp read tools and the no-MCP fallbacks (this reference uses them).
- `interrealm.md` — `cur`/crossing/`IsUserCall` semantics, to read the realm calls and payment guards correctly.
- `security.md` / `audit.md` — when auditing a sys realm; `audit.md`'s provenance rule (name the chain you read) is the same discipline as I1 here.
- `render.md` — the `Render()` / `vm/qrender` surface and gnoweb markdown; go there to read or audit a sys realm's rendered output (the `gno_render` calls above land in its `qrender` surface).

## Source

Distilled from `examples/gno.land/r/sys/*` + `gnovm/stdlibs/sys/params` in gnolang/gno, the gnolang/gno
issue/PR roadmap, per-network genesis configs, and verified against live test13 (ABCI `vm/qfuncs`/`qeval`/`qrender`).
The design above is durable; concrete values are intentionally absent — query the live chain.
