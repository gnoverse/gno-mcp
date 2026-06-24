# gnokey — the gno.land CLI, and the tx machinery gnomcp wraps

> **Category: gno.land tooling.** `gnokey` is gno.land's key manager + transaction tool. This
> reference teaches the **tx model behind every write** — the gas/fee/deposit knobs, the
> sign→broadcast pipeline, the failure modes — so you can reason about what a write actually costs
> and why it fails, and explain or display the equivalent command. It is **not** a license to shell
> out: when a gnomcp server is connected, gnomcp signs for you and raw `gnokey` for a write is a
> wrong turn (see "The discipline"). The flag surface is concrete (state it), but it is
> **version-bound** — the gnokey UX is actively being reworked (gnolang/gno #3703); when in doubt,
> `gnokey <cmd> -help`.

## Why this matters

Every on-chain write — a realm call, a deploy, a send — is a transaction carrying a **fee** and a
**gas limit**. Get the mental model wrong and you either overpay real money on every tx or hit
opaque "out of gas" / "insufficient fee" failures. The single most expensive misconception:
**the chain deducts the *full fee you offer*, not the gas you actually use.** gnomcp pins the fee to
the chain minimum precisely because of this; understand the model and you understand both tools.

## The mapping — gnomcp tool ↔ gnokey command

gnomcp does not call gnokey. It has its own keystore (`~/.gnomcp/keys/`, **separate** from gnokey's
`~/.gnokey/` — a `gno_key_generate` key is invisible to `gnokey list`, and vice-versa) and builds,
signs, and broadcasts each tx atomically in one tool call. gnokey splits the same work into
`maketx` → `sign` → `broadcast`. Same messages underneath:

| gnomcp tool | gnokey command | Message built | What it is |
|---|---|---|---|
| `gno_call` | `gnokey maketx call` | `vm.MsgCall` | Call a realm's crossing function |
| `gno_addpkg` | `gnokey maketx addpkg` | `vm.MsgAddPackage` | Deploy a package/realm |
| `gno_run` | `gnokey maketx run` | `vm.MsgRun` | Run ephemeral `main()` code |
| `gno_key_send` | `gnokey maketx send` | `bank.MsgSend` | Transfer ugnot |
| `gno_session_propose` | `gnokey maketx session create` | `auth.MsgCreateSession` | Open a user-authorized session |
| `gno_session_revoke` | `gnokey maketx session revoke` | `auth.MsgRevokeSession` | Close a session |
| `gno_key_generate` | `gnokey add` / `add --recover` | — | Create / recover a key |
| `gno_key_list` · `gno_key_delete` · `gno_key_address` | `gnokey list` · `delete` · (address of) | — | Manage the gnomcp keystore |
| `gno_eval` · `gno_render` · `gno_packages` · `gno_account` | `gnokey query vm/qeval` · `vm/qrender` · `vm/qfuncs` · `auth/accounts/<addr>` | — | Reads (ABCI queries) |

Reads have no key, fee, or signature — `gnokey query` and `gno_eval`/`gno_render` are the same ABCI
paths (see `mcp.md`, `sysrealms.md`). Everything below is about the **write** path.

## The tx model — `Fee = {GasWanted, GasFee}`, two independent knobs

A signed tx carries `std.Fee{ GasWanted int64; GasFee Coin }`. These are **not** derived from each
other — they are two separate dials, and conflating them is the root of most gas confusion.

- **`GasFee`** (`--gas-fee`, e.g. `10000ugnot`) — a **flat total amount you offer for the whole tx**.
  The ante handler deducts it **in full, regardless of how much gas the tx actually burns.** There is
  **no refund** of the difference. Offering more than the floor is money burned for nothing.
- **`GasWanted`** (`--gas-wanted`, e.g. `10000000`) — the **execution ceiling** in gas units. If the
  tx consumes more than this, it aborts with **out of gas**. It is *not* a cost; it only caps work.
  It must also stay under the chain's **block max gas** (else `invalid gas-wanted`).

The effective per-unit price the chain sees is `GasFee / GasWanted`. A tx is admitted only if that
ratio clears the chain's minimum (next section). So the two knobs interact only through their ratio:
raise `GasWanted` and you must raise `GasFee` to keep the ratio above the floor.

**The floor.** The minimum acceptable fee is `GasWanted × minGasPrice`. On gno testnets the price is
**1 ugnot per 1000 gas**, so for `GasWanted = 10_000_000` the floor is **10,000 ugnot (0.01 GNOT)** —
and that is exactly what gnomcp offers (`DefaultGasFeeUgnot = DefaultGasWanted / 1000`). A realm
call burns ~1–2M gas, a deploy ~5M — all far under the 10M ceiling, so one fixed `(10M, 10_000ugnot)`
pair covers every ordinary write at the cheapest price the chain will accept.

> **The footgun that bit this project:** the old default offered a **10 GNOT** fee on every tx —
> ~1000× the floor — and since the chain takes the full offered fee, every write silently cost 10
> GNOT of real balance for ~0.005 GNOT of actual gas. (gnolang/gno #3805 states this from the
> maintainers: *"`--gas-fee` is the total for the whole tx, deducted in full; overpaying is real
> money lost."*) Fix: offer the floor, never more.

### Where the price actually lives

`minGasPrice` is **node config** (`min_gas_prices`, grammar `"<amount><denom>/<gas>gas"`, e.g.
`1ugnot/1000gas`), not a constant in the binary — so a different target chain can set a different
price, and the 10,000-ugnot floor moves with it. gnomcp mirrors the ratio as `minGasPriceDivisor`
and its comment flags that it must change for a chain with a different `min_gas_prices`. Two price
gates exist: this static config floor (checked in CheckTx) and an adaptive EIP-1559-style block price
(skipped when zero, the common testnet case) — on test13 the static floor is the operative one.

## Failure modes — tell them apart

Both of these still **cost you the fee** (ante deductions are committed even on a failed DeliverTx),
so distinguishing them matters:

| Symptom | Real cause | When | Fix |
|---|---|---|---|
| `insufficient fees; got {Gas-Wanted:N, Gas-Fee:Mugnot}, fee required {Gas:1000 Price:1ugnot}` | `GasFee/GasWanted` below the price floor | **CheckTx** — rejected before execution, no state touched | Raise `GasFee` to ≥ `GasWanted × price` (or lower `GasWanted`) |
| `out of gas in location: …; gasWanted: N, gasUsed: M` | tx burned more gas than `GasWanted` | **DeliverTx** — executed then aborted; fee gone | Raise `GasWanted` **well** above M — see note |
| `invalid gas-wanted; got: N block-max-gas: 3000000000` | `GasWanted` exceeds the block gas cap | CheckTx | Keep `GasWanted` under the chain's block max (~3B) |
| `insufficient funds to pay for fees` | balance < `GasFee` | CheckTx/DeliverTx | Fund the key, or lower `GasFee` toward the floor |
| `not enough deposit to cover the storage usage: requires D … for B bytes` | storage deposit cap < bytes × storage price | DeliverTx | Raise `--max-deposit` (see Storage deposit) |

**Out-of-gas reports gas-used-*so-far*, not the right limit** (gnolang/gno #3704). Bumping
`--gas-wanted` to just above the reported number fails again — bump comfortably above it, or
simulate to get the real figure.

## Storage deposit — `--max-deposit` (a cap, not a charge)

Persisting bytes on-chain **locks** a deposit = `bytes × storage_price` (default **100 ugnot/byte**,
"1 GNOT per 10 KB"). `--max-deposit` / gnomcp's `DefaultMaxDepositUgnot` is the **maximum you'll let
the tx lock**, not an amount spent. Key facts:

- An **empty** `--max-deposit` does *not* mean unbounded — it falls back to the `DefaultDeposit`
  chain param (**600 GNOT** cap), so a deploy can lock far more than you expect if the realm is large.
- The deposit is **refundable**: when a realm frees storage (state deleted), the proportional locked
  amount is returned. It's a deposit, not a fee.
- gnomcp pins `DefaultMaxDepositUgnot = 10_000_000` (10 GNOT) on **`gno_addpkg` only**; `gno_call`
  and `gno_run` leave it unset (the 600 GNOT param cap applies). A deploy rejected for
  `not enough deposit` is the signal to raise the cap.
- This is distinct from the gas fee and goes to a **different collector** (`vm:p:storage_fee_collector`,
  not the gas `auth:p:fee_collector`). The "insufficient coins" you hit on a starved account is the
  deposit failing *after* the gas fee already drained the balance.

## Simulate before you broadcast

Simulation runs the **full ante handler + full message execution** against a throwaway cache: it
charges nothing, commits nothing, **skips signature verification** (so you can simulate *unsigned* —
only the public key is needed, no password; gnolang/gno #4279), and returns the **real `GasUsed`**.
This is how you size `GasWanted` honestly instead of guessing.

- gnokey: `--simulate` is `test` (simulate, then broadcast if it passes — the default), `skip`
  (broadcast directly), or `only` (dry-run; also prints a suggested fee with a `--gas-fee-margin`,
  default 5%). `--broadcast` defaults true. **Estimation advises but does not auto-fill** —
  `maketx` still errors if you don't pass `--gas-wanted` and `--gas-fee` yourself.
- gnomcp: the `simulate` parameter on `gno_call`/`gno_addpkg`/`gno_run` does the same dry-run; the
  write tools also **simulate before broadcasting** so a type error or an unmet deploy gate
  (CLA/namespace) fails at **zero gas** instead of stranding a freshly-funded key.

Best practice (gnokey or gnomcp): **simulate → read `GasUsed` → set `GasWanted` a margin above it →
set `GasFee = GasWanted × min price`, no higher.**

## Flag reference

**tx-build** (`maketx <call|addpkg|run|send>`):

| Flag | Purpose | gnomcp stance |
|---|---|---|
| `--gas-wanted` | execution ceiling (gas units); required, no default | pinned `10_000_000` |
| `--gas-fee` | flat total fee offered (e.g. `10000ugnot`); required | pinned to the floor `10_000ugnot` |
| `--send` | coins attached to the msg (→ realm as `OriginSend`), separate from the fee | user-supplied (`send` arg) |
| `--max-deposit` | cap on storage deposit locked (empty → 600 GNOT param) | pinned `10_000_000` on addpkg only |
| `--memo` | free text in the signed tx (≤ 64 KB; costs gas by size) | not set |
| `--pkgpath` `--func` `--args` (call) · `--pkgdir` (addpkg) · `--to` (send) | the message payload | the tool's own params |

**broadcast / signing** (base flags + `sign`/`broadcast`):

| Flag | Purpose | gnomcp stance |
|---|---|---|
| `--remote` | node RPC URL (default `127.0.0.1:26657`) | fixed at profile/connection (`NewReal(rpcURL,…)`) |
| `--chainid` | chain id mixed into the **signature** (default `dev`); wrong value → opaque sig failure | fixed at profile |
| `--simulate` / `--broadcast` | dry-run vs send (above) | the `simulate` param; gnomcp always broadcasts otherwise |
| `--account-number` / `--account-sequence` | replay/identity counters in the SignDoc | **auto** (queried from `auth/accounts/<addr>`); gnomcp passes 0,0 to force the query |

**keys** (`add` `generate` `list` `delete` `export` `import` `rotate`, sub-modes `add multisig|ledger|bech32`):
create/recover/inspect keys in `~/.gnokey/`. gnomcp manages its own keystore via
`gno_key_generate` / `gno_key_list` / `gno_key_delete` (no export/import/rotate). **Don't mix the
two stores** — they don't see each other's keys.

## Best practices & footguns (from the gnolang/gno issue tracker)

| Footgun | Right way | Source |
|---|---|---|
| Overpaying `--gas-fee` (full fee is taken) | Offer the floor: `GasWanted × min price`, never more | #3805, #5086 |
| `--gas-fee 1gnot` → "insufficient funds" | Use ugnot: `--gas-fee 10000ugnot` (denominations don't mix in the fee field) | #329 |
| `--gas-wanted 4000000000` → "invalid gas-wanted" | Keep under block-max-gas (~3B) | #329 |
| Empty `--send ""` to a payable realm → "payment must not be less than …" | A required deposit must ride in `--send`; empty sends nothing | #329 |
| `out of gas` → bump to just above the reported number, fails again | The number is gas-used-so-far; bump well above, or simulate | #3704 |
| `signature verification failed; verify correct account, sequence, and chain-id` | First suspect a **stale gnokey binary** (a known cause); then check `--chainid`, and `--account-number`/`--sequence` from `gnokey query auth/accounts/<addr>` | #2109 |
| `gnokey add --derivation-path …` prints derived addrs but may save an **un-derived** key | Verify the stored key's `path:` before signing | #5122 |
| Tx "succeeds" at CheckTx then fails at DeliverTx | The real cause is one line buried in a large `Log` stack dump — grep the deliver-tx log; typed realm errors aren't here yet | #203, #416 |
| Mismatched / missing `--remote` + `--chainid` | Both must be set and match the target chain (the UX may collapse to one `-chain` flag later) | #3703 |

## The discipline — when gnomcp is connected, it signs for you

With a gnomcp server connected, an agent does writes through `gno_call` / `gno_addpkg` / `gno_run` /
`gno_key_send` / sessions — **never** raw `gnokey`, and never by reading, asking for, or importing a
key or mnemonic. Reaching for `gnokey` for a write means you took a wrong turn, because:

- **Separate keystore.** gnomcp keys live in `~/.gnomcp/keys/`; `gnokey` can't see or sign with them.
- **Atomic + safe defaults.** gnomcp builds-signs-broadcasts in one step, simulates first, and pins
  the fee to the floor — you can't accidentally overpay or strand a key.
- **It's a hard rule here.** The gno-build skill forbids it, and the e2e flows fail any run where the
  agent shells out to `gnokey`.

gnokey understanding is for three things: **reasoning** about what a write costs and why it fails;
**reading chain params** that gnomcp may not wrap (the `gnokey query params/<module>:p:<name>`
fallback — see `sysrealms.md`); and **showing** a user the equivalent command (next).

## Showing the equivalent gnokey command (transparency)

Any gnomcp write maps mechanically to the `gnokey maketx` command it stands in for — useful to teach
a user what just happened or to hand them a reproducible line. The pieces all come from the tool call
plus the profile (rpc, chain-id) and the pinned gas defaults:

```
# gno_call{realm:"gno.land/r/demo/foo", func:"Bump", args:["1"], send:"", key:"alice"}
gnokey maketx call \
  -pkgpath gno.land/r/demo/foo -func Bump -args 1 \
  -gas-wanted 10000000 -gas-fee 10000ugnot \
  -remote https://rpc.test13.testnets.gno.land:443 -chainid test-13 \
  -broadcast alice
```

Template: `gnokey maketx <call|addpkg|run|send> <msg flags> -gas-wanted <GasWanted> -gas-fee
<GasFee>ugnot [-send <amt>] [-max-deposit <amt>] -remote <rpc> -chainid <id> -broadcast <key>`.
Show it as *equivalent*, not as something to run instead — the agent already did the write via gnomcp.

If the gnomcp write result already carries a `gnokey_command` field (some servers emit it), that is
exactly this string — surface it directly rather than re-deriving. It's a convenience, not a
dependency: this reference renders the same command from first principles when the field is absent.

## See also

- `mcp.md` — the gno-mcp read tools and the no-MCP ABCI fallbacks; the read half of the table above.
- `sysrealms.md` — the deploy gates (`r/sys/names`, `r/sys/cla`) a write must clear, and the
  `gnokey query params/…` recipe for raw chain params; "trust the chain" discipline for live values.
- `build.md` — authoring/testing/deploying realms; "Never touch keys" — the deploy-side companion to
  the discipline here.
- `debug.md` — failure-signature triage; the gas/sequence/deposit errors above feed its table.

## Source

Distilled from `gnokey` (`tm2/pkg/crypto/keys/client`, `gno.land/pkg/keyscli`), the auth ante handler
and fee/gas-price logic (`tm2/pkg/sdk/auth`, `tm2/pkg/std`), and the vm storage-deposit keeper
(`gno.land/pkg/sdk/vm`) in gnolang/gno at the commit pinned in this repo's go.mod; the gnomcp write
path (`internal/chain/real.go`, `internal/tools/write`); and the gnolang/gno issue tracker (#3805,
#5086, #3704, #329, #2109, #4279, #5122, #203, #416, #3703). Mechanics verified against the live
test13 deploy-gates flow. Flag surface is version-bound — confirm with `gnokey <cmd> -help`.
