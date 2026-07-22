# Networks — per-chain facts and cross-chain drift

Both public testnets are live and **fully writable**: topaz (test14) is the current chain,
test13 is its sunset predecessor (retiring — prefer topaz for new work, but deploys to test13
must work without friction). Every value below was verified live on **2026-07-22**; chain state
moves, so re-query anything load-bearing (`gno_status`, `auth/gasprice`, a realm's render) before
relying on it. Method and design live in the topic references — this file is only the per-chain
snapshot and the cross-chain differences.

## Per-chain matrix

| Fact | topaz (test14) — current | test13 — sunset, still writable |
|---|---|---|
| chain-id | `topaz-1` | `test-13` |
| RPC | `rpc.topaz.testnets.gno.land:443` | `rpc.test13.testnets.gno.land:443` |
| gnoweb | `topaz.testnets.gno.land` | `test13.testnets.gno.land` |
| gas price (`auth/gasprice`) | `1ugnot/1000gas` (genesis floor) | `10ugnot/1000gas` (10× drifted) |
| minimum fee (price floor) for a 10M-gas write | 10,000 ugnot (0.01 GNOT) | 100,000 ugnot (0.1 GNOT) — gnomcp offers ×2 over the floor (`gnokey.md`) |
| storage deposit (`params/vm:p:storage_price`) | 100 ugnot | 100 ugnot (same) |
| CLA deploy gate (`r/sys/cla`) | **OFF** — no `Sign` step needed | **ON** — `Sign(hash)` once per key first |
| namespace gate (`r/sys/names.IsEnabled`) | on; personal-address path free | same |
| name registration | `r/sys/namereg/v1`, free (0 ugnot) | same flow, same controller |
| faucet (`faucet-agent.<host>/limits`) | 10 GNOT/grant, 1/addr/24h | identical, still live |
| tx indexer | `indexer.topaz…/graphql/query` | `indexer.test13…/graphql/query` (live) |
| toolchain tag (local testing) | `chain/topaz` (commit-only) | `chain/test13` (commit-only) |
| ecosystem | 137 pkgs (fresh start) | 618 pkgs (bulk is test/audit spam) |
| notably absent | grc721, GnoSwap, Akkadia | grc721 |

Toolchain tags use the short chain **name**, never the chain-id (`chain/topaz`, not
`chain/topaz-1`); both are commit-only — install by commit SHA (`toolchain.md` has the recipe).

## Cross-chain API drift — same import path, different source

The chains were cut from different gno commits, so shared packages drifted. Code copied from a
deployment on one chain can fail to compile or behave differently on the other. Write against the
**target chain's** deployed source (fetch it with `gno_read` / `vm/qfile` on that chain):

| Package | test13 form | topaz (test14) form |
|---|---|---|
| `p/nt/avl/v0` `Tree.Get` | `Get(key string) (any, bool)` — `v, ok := tree.Get(k)` | `Get(key string) any` — no bool; use `Has(k)` or nil-check |
| `p/demo/tokens/grc20` `NewToken` | `NewToken(0, cur, "wrapped GNOT", "wugnot", 0)` (int, realm, name, symbol, decimals) | `NewToken("wrapped GNOT", "wugnot", 0, 0, cur)` (name, symbol, decimals, seqid.ID, realm) |
| `grc20` `Token.ID()` format | `origRealm + "." + symbol` | `pkgPath + "." + symbol + "." + seqid` — **not comparable across chains** |
| `p/nt/uassert/v0` `ErrorIs` | string-compares `err.Error()` — `%w`-wrapped errors do NOT match | real `errors.Is` — wrapped errors match |
| `p/nt/markdown/sanitize/v0` mailto | rejects only literal `?body=`/`&body=` | rejects any `?` or `&` in the URL |

No drift (byte-identical, safe to treat as portable): `p/nt/ufmt/v0`, `p/nt/seqid/v0`,
`p/nt/ownable/v0`. Both chains' genesis code uses the current `cross(fn)(...)` interrealm calling
generation — the VM syntax an agent writes is the same on both; only package APIs differ.

## Deploying to either chain — the checklist

1. **Confirm the target** — `gno_status` (chain-id) or `gno_profile_list` (name ↔ chain-id map).
2. **Gates** — namespace: personal-address path is free on both. CLA: required on test13
   (`gno_cla_info` → show URL → `gno_cla_sign`), not on topaz today — always confirm live.
3. **Fund** — faucets are identical (10 GNOT, 1/addr/24h) and both live.
4. **Fees** — query `auth/gasprice` per chain; the same write costs 10× more ugnot on test13.
5. **Imports** — check the drift table above; when in doubt, read the package's deployed source
   on the target chain before calling it.
6. **Local tests** — use the chain-matched toolchain (`chain/topaz` / `chain/test13` commit) and
   vendor on-chain deps from the matching source tree; a develop-HEAD toolchain can refuse to
   compile deps auto-fetched from either chain (`toolchain.md`).
