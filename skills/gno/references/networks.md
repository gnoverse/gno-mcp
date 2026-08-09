# Networks — per-chain facts and cross-chain drift

Both public testnets are live and **fully writable**: sapphire (chain-id `sapphire-1`) is the
current chain, topaz (`topaz-1`) is its sunset predecessor (retiring — prefer sapphire for new
work, but deploys to topaz must work without friction). Every value below was verified live on
**2026-08-09**; chain state moves, so re-query anything load-bearing (`gno_status`,
`auth/gasprice`, a realm's render) before relying on it. Method and design live in the topic
references — this file is only the per-chain snapshot and the cross-chain differences.

## Per-chain matrix

| Fact | sapphire — current | topaz — sunset, still writable |
|---|---|---|
| chain-id | `sapphire-1` | `topaz-1` |
| RPC | `rpc.sapphire.testnets.gno.land:443` | `rpc.topaz.testnets.gno.land:443` |
| gnoweb | `sapphire.testnets.gno.land` | `topaz.testnets.gno.land` |
| node version | `v1.0.0-rc.0` | `v1.0.0-rc.0` (same) |
| gas price (`auth/gasprice`) | `1ugnot/1000gas` (genesis floor) | `1ugnot/1000gas` (same) |
| minimum fee (price floor) for a 10M-gas write | 10,000 ugnot (0.01 GNOT) — gnomcp offers ×2 over the floor (`gnokey.md`) | same |
| storage deposit (`params/vm:p:storage_price`) | 100 ugnot | 100 ugnot (same) |
| CLA deploy gate (`r/sys/cla`) | **OFF** — "enforcement is currently DISABLED" | **OFF** (same) |
| namespace gate (`r/sys/names.IsEnabled`) | `true`; personal-address path free | same |
| name registration | `r/sys/namereg/v1`, not paused; `registerPrice` is **0 today** but GovDAO-settable (`ProposeNewRegisterPrice`), and `Register` requires the sent amount to equal it exactly — read it live, don't assume free | same flow, same controller |
| faucet (`faucet-agent.<host>/limits`) | 10 GNOT/grant, 1/addr/24h | identical, still live |
| tx indexer | `indexer.sapphire…/graphql/query` | `indexer.topaz…/graphql/query` (live) |
| toolchain tag (local testing) | `chain/sapphire` at `9ab5198` — matches its branch tip; node reports `build_version: chain/sapphire` | `chain/topaz` at `fc40526` — **lags** `heads/chain/topaz` (`63c2673`); node reports a branch build, so pin that sha |
| ecosystem | 140 pkgs (fresh chain, 11 in user namespaces) | 397 pkgs |
| notably absent | grc721, GnoSwap, Akkadia, `p/nt/commondao/v0`, `r/gnoland/boards2/v1/hub` | grc721 |
| only here | `p/nt/groups/v0`, `p/gnoland/boards/exts/hub` | `p/nt/commondao/v0`, `boards2` hub |

Toolchain tags use the short chain **name**, never the chain-id (`chain/sapphire`, not
`chain/sapphire-1`). Both chains ship one, and neither is a valid `go install @` ref (the `/`),
so both install by commit SHA — see `toolchain.md`. Watch the tag-vs-branch gap: sapphire's tag
is its branch tip, topaz's is not, so for topaz pin the sha its node reports rather than the tag.

Namespaces are registered on sapphire for aib, akkadia, gnoswap, onbloc and samcrew, but those
teams have **not deployed yet** — a registered namespace is not a deployed package. Query the
chain before importing anything from them.

## Cross-chain API drift — same import path, different source

Sapphire and topaz were cut close together, so the shared packages barely moved. Diffing the
deployed sources of `p/nt/avl/v0`, `p/nt/uassert/v0` and `p/nt/markdown/sanitize/v0` on both
chains: **byte-identical**. The one difference is additive:

| Package | topaz form | sapphire form |
|---|---|---|
| `p/demo/tokens/grc20` | no creation event | `NewToken` now emits a `NewToken` event (`NewTokenEvent`) carrying `Token.ID()`, name, symbol, decimals |

`NewToken`'s signature (`NewToken(name, symbol string, decimals int, id seqid.ID, rlm realm)`) and
`Token.ID()`'s format are **unchanged** between the two chains, so grc20 code ports either way; only
an indexer watching for the new event sees a difference.

That is the whole drift surface for these packages — do not assume the larger test13-era divergence
still applies. It does not: code written against topaz compiles and behaves the same on sapphire.
Still read the **target chain's** deployed source (`gno_read` / `vm/qfile`) before relying on any
package this file does not cover.

## Deploying to either chain — the checklist

1. **Confirm the target** — `gno_status` (chain-id) or `gno_profile_list` (name ↔ chain-id map).
2. **Gates** — namespace: personal-address path is free on both. CLA: enforcement is **off on both
   chains today**, so no `Sign` step is needed — but it is a chain setting, so confirm live with
   `gno_cla_info` rather than trusting this line.
3. **Fund** — faucets are identical (10 GNOT, 1/addr/24h) and both live.
4. **Fees** — same price on both (`1ugnot/1000gas`); still query `auth/gasprice` per chain, since
   this is the value most likely to drift next.
5. **Imports** — the drift table above is short, but sapphire carries far fewer packages (140 vs
   397). "It exists on topaz" does not mean it exists on sapphire — verify on the target chain.
6. **Local tests** — use the chain-matched toolchain and vendor on-chain deps from the matching
   source tree; a develop-HEAD toolchain can refuse to compile deps auto-fetched from either chain
   (`toolchain.md`). Both chains install by commit SHA; for topaz take the sha from `/status`,
   since `chain/topaz` lags its branch.
