# Networks — per-chain facts and cross-chain drift

Both public testnets are live and **fully writable**: pearl (chain-id `pearl-1`) is the current
chain, sapphire (`sapphire-1`) is its sunset predecessor, retiring but still fully writable
(prefer pearl for new work, but deploys to sapphire must work without friction). Every value below
was verified live on **2026-08-27**; chain state moves, so re-query anything load-bearing
(`gno_status`, `auth/gasprice`, a realm's render) before relying on it. Method and design live in
the topic references — this file is only the per-chain snapshot and the cross-chain differences.

## Per-chain matrix

| Fact | pearl — current | sapphire — sunset, still writable |
|---|---|---|
| chain-id | `pearl-1` | `sapphire-1` |
| RPC | `rpc.pearl.testnets.gno.land:443` | `rpc.sapphire.testnets.gno.land:443` |
| gnoweb | `pearl.testnets.gno.land` | `sapphire.testnets.gno.land` |
| node version | `v1.0.0-rc.0` | `v1.0.0-rc.0` (same) |
| gas price (`auth/gasprice`) | `1ugnot/1000gas` (genesis floor) | `1ugnot/1000gas` (same) |
| minimum fee (price floor) for a 10M-gas write | 10,000 ugnot (0.01 GNOT) — gnomcp offers ×2 over the floor (`gnokey.md`) | same |
| storage deposit (`params/vm:p:storage_price`) | 100 ugnot | 100 ugnot (same) |
| CLA deploy gate (`r/sys/cla`) | **OFF** — "enforcement is currently DISABLED" | **OFF** (same) |
| namespace gate (`r/sys/names.IsEnabled`) | `true`; personal-address path free | same |
| name registration | `r/sys/namereg/v1`, not paused; `registerPrice` **0 today** but GovDAO-settable, and `Register` requires the sent amount to equal it exactly — read it live, don't assume free. Format `nym-<stem><digits>`: stem 5–13 lowercase letters, exactly 3 digits, 12–20 chars total. No names registered yet | same controller, same format and price; names already registered |
| faucet (`faucet-agent.<host>/limits`) | 10 GNOT/grant, 1/addr/24h | identical, still live |
| tx indexer | `indexer.pearl…/graphql/query` — current, sitting at the chain tip, and running a build that serves `getSupply` | `indexer.sapphire…/graphql/query` — current, but an older build with no `getSupply` |
| toolchain tag (local testing) | `chain/pearl` at `c4c72fd` — matches `heads/chain/pearl` | `chain/sapphire` at `9ab5198` — matches its branch tip |
| ecosystem | 85 pkgs — the curated genesis set, no user deploys yet | 397 pkgs |
| notably absent | grc721, GnoSwap, Akkadia, `p/nt/commondao/v0` | grc721 under `p/demo`, `p/nt/commondao/v0` |
| only here | nothing — pearl's tree is a strict subset of sapphire's | 312 user-namespace deploys (GnoSwap, aib IBC, personal-address packages) |

Toolchain tags use the short chain **name**, never the chain-id (`chain/pearl`, not `chain/pearl-1`).
Both chains ship one, both match their branch tips, and neither is a valid `go install @` ref (the
`/`), so both install by commit SHA — see `toolchain.md`. Neither node reports a build sha (`/status`
carries a release `version` and an empty `software`), so the tag is the only anchor the repo offers.

**Pearl carries no user deploys.** It launched fresh on 2026-08-27 with a curated 85-package genesis
and no namespaces claimed, so anything outside that set is absent — including packages that exist on
sapphire. Query the chain before importing.

**Supply figures come from the indexer, on pearl only.** `getSupply(denom)` returns `total`,
`locked` and `spendable` at a given height, where `locked` is the unvested portion held by vesting
accounts. Pearl created vesting accounts at genesis, which is what makes the split meaningful there;
sapphire's indexer runs a build without the query. Read it from the indexer, never from a realm.

## Retired chains

**topaz (`topaz-1`)** is gone: its RPC, gnoweb, indexer and faucet hostnames no longer resolve, so
gnomcp ships no builtin profile for it and `topaz-1` is no longer a writable chain-id. Its tag
`chain/topaz` (`fc40526`) survives in the repo and sits **behind** `heads/chain/topaz` (`63c2673`),
which is why a tag is worth comparing against its branch head before pinning. `p/nt/commondao/v0`
was deployed only here, so it now resolves on no live testnet.

Earlier numbered testnets (`test1`–`test13`) are likewise dead; the `test` name stays writable
because it also covers the e2e simnet's `test-9999`.

## Cross-chain API drift — same import path, different source

Pearl was cut from a later master than sapphire, so shared packages have moved. Comparing the
deployed sources of the 22 packages these references teach, file by file:

| Package | pearl vs sapphire |
|---|---|
| `p/nt/{ufmt,uassert,urequire,seqid,ownable,mux,bptree,testutils}/v0`, `p/moul/{txlink,authz,realmpath}`, `p/jeronimoalbi/pager`, `r/sys/cla`, `r/gnoland/blog`, `r/tests/vm` | byte-identical |
| `p/nt/avl/v0` | a documentation link in `README.md`; API and implementation identical |
| `p/nt/markdown/sanitize/v0` | added test cases only; `sanitize.gno` identical |
| `p/nt/treasury/v0` | `render.gno` puts banker IDs through `md.EscapeText`; signatures unchanged, rendered output differs |
| `r/sys/names`, `r/sys/namereg/v1`, `r/sys/users` | internals only — **exported API identical** (6, 11 and 29 functions, same signatures). Pearl adds `cur.IsCurrent()` guards to crossing entrypoints such as `names.Enable`, so the boundary rule `security.md` teaches is enforced in the genesis realms themselves |
| `p/demo/tokens/grc20` | **breaking** — see below |

**`p/demo/tokens/grc20` breaks source compatibility.** `CallerTeller()` moved off `*Token` and onto
`*PrivateLedger`:

```go
tok, ledger := newTestToken(...)
teller := tok.CallerTeller()     // sapphire
teller := ledger.CallerTeller()  // pearl
```

Pearl also adds a `guardHome` check: a frame-relative teller only works inside the token's own realm
(sub-realms included), so a teller that a realm builds and then exports is inert everywhere else.
Code written against sapphire's grc20 will not compile on pearl, and a design that passed tellers
between realms will not work there. Port deliberately rather than assuming it moves.

One breaking change in 22 packages, and it is the only difference an importer can observe: every
other package either matches byte for byte or moved without touching its exported signatures. Still
read the **target chain's** deployed source (`gno_read` / `vm/qfile`) before relying on any package
this file does not cover.

## Deploying to either chain — the checklist

1. **Confirm the target** — `gno_status` (chain-id) or `gno_profile_list` (name ↔ chain-id map).
2. **Gates** — namespace: personal-address path is free on both. CLA: enforcement is **off on both
   chains today**, so no `Sign` step is needed — but it is a chain setting, so confirm live with
   `gno_cla_info` rather than trusting this line. Both chains currently accept code from anyone
   (no code-submission policy set); a chain may adopt a review gate that parks a deploy instead of
   publishing it, so read the deploy's result rather than assuming a package went live.
3. **Fund** — faucets are identical (10 GNOT, 1/addr/24h) and both live.
4. **Fees** — same price on both (`1ugnot/1000gas`); still query `auth/gasprice` per chain, since
   this is the value most likely to drift next.
5. **Imports** — pearl carries 85 packages to sapphire's 397, and pearl's set is a strict subset.
   "It exists on sapphire" does not mean it exists on pearl — verify on the target chain. Where a
   package exists on both, check the drift table: grc20 in particular is not source-compatible.
6. **Transaction history** — `gno_activity`/`gno_history` work on both chains. `gno_list` does not
   work anywhere: it needs a `realms` query absent from both deployed indexers, so enumerate
   packages with `gno_packages` instead.
7. **Local tests** — use the chain-matched toolchain and vendor on-chain deps from the matching
   source tree; a develop-HEAD toolchain can refuse to compile deps auto-fetched from either chain
   (`toolchain.md`). Both chains install by commit SHA from a tag that matches its branch tip.
