# agentfaucet

A standalone HTTP service that funds agent keys on a testnet. An operator runs one per chain and advertises it to gnomcp through a profile's `faucet-service-url`, so `gno_faucet_fund` can top up an agent key automatically. It's independent of the MCP server — the only coupling is HTTP.

It ships as a release binary (`agentfaucet_<os>_<arch>.tar.gz`) and a multi-arch image `ghcr.io/gnoverse/agentfaucet`.

## Run

```bash
# Docker
docker run --rm -p 8590:8590 \
  -e GNOMCP_FAUCET_MNEMONIC="<funding key mnemonic>" \
  ghcr.io/gnoverse/agentfaucet:latest \
  -rpc-url https://rpc.test13.testnets.gno.land:443 -chain-id test-13 -listen 0.0.0.0:8590

# Binary
GNOMCP_FAUCET_MNEMONIC="<funding key mnemonic>" \
  agentfaucet -rpc-url https://rpc.test13.testnets.gno.land:443 -chain-id test-13
```

The funding mnemonic is read from `GNOMCP_FAUCET_MNEMONIC`, never a flag default — a non-empty flag default is printed by `-help` and on any flag error, which would leak the key to stderr/logs (and argv is visible to `ps` and shell history). Only `test<N>` / `test-<N>` chain-ids are accepted; `dev` and everything else are refused. The default `-listen` is `127.0.0.1:8590` for host safety, so in a container you must pass `-listen 0.0.0.0:8590`.

## Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-rpc-url` | *(required)* | gno node RPC URL |
| `-chain-id` | *(required)* | target testnet chain-id; must match `test<N>` or `test-<N>` (e.g. `test-13`) |
| `-mnemonic` | `$GNOMCP_FAUCET_MNEMONIC` | BIP-39 mnemonic for the funding key — prefer the env var |
| `-listen` | `127.0.0.1:8590` | address to listen on |
| `-metrics-addr` | *(empty — off)* | enable the Prometheus `/metrics` listener on this address (e.g. `127.0.0.1:8591`); a separate listener, so metrics stay off the public fund port. Opt-in: empty disables |
| `-trusted-proxies` | `0` | number of trusted reverse-proxy hops for `X-Forwarded-For` client-IP extraction (`0` = ignore XFF, use the direct peer; set to the proxy count, e.g. `1` behind a single ALB) |
| `-grant` | `1000000000` | ugnot dispensed per drip (1,000 GNOT — ugnot is micro-GNOT, 1 GNOT = 1,000,000 ugnot) |
| `-gas-fee` | `1000000ugnot` | gas fee per dispense tx; a faucet send is cheap, so this is far below the gnomcp write default (sized for deploys) |
| `-gas-wanted` | `5000000` | gas limit per dispense tx (a test-13 bank send burns ~1.6M) |
| `-per-addr-cooldown` | `24h` | minimum time between grants to the same address |
| `-per-ip-max` | `60` | max grants per IP per `-per-ip-window` |
| `-per-ip-window` | `1h` | sliding window for the per-IP limit |
| `-daily-cap` | `100000000000` | hard global daily ugnot outflow cap (100,000 GNOT) |
| `-drip-burst` | `0` | global outflow token-bucket capacity in ugnot — the largest burst tolerated; master switch (`0` disables the drip control entirely) |
| `-drip-rate` | `0` | global outflow token-bucket refill in ugnot/sec; inert unless `-drip-burst` is set |
| `-min-funding-balance` | `0` | refuse grants (`503`) while the funding wallet holds fewer ugnot than this (`0` disables) |

`agentfaucet -help` lists them all. `agentfaucet version` prints the build version.

## Limits & anti-abuse

An optional funding-balance floor gates every grant first, then up to four limits gate it in order — **per-address → per-IP → daily cap → global drip** — and the first that trips rejects the request. The limit trips return HTTP `429`; the funding floor returns `503`:

0. **Funding-balance floor** (`-min-funding-balance`, default disabled). Checked before any limit: while the funding wallet holds fewer ugnot than the floor, grants are refused with `503` so the faucet degrades gracefully instead of failing mid-dispense on an empty key. The balance is TTL-cached (30s), so it's a graceful-degradation guard, not a hard solvency invariant. Trips → `faucet: funding wallet below minimum balance`.
1. **Per-address cooldown** (`-per-addr-cooldown`, default 24h). At most one grant per address per window, as a sliding window. The address is canonicalized from bech32 first, so case variants of the same address share one bucket and a malformed address is rejected (`400`) before any limit is touched. Trips → `faucet: address in cooldown`.
2. **Per-IP rate limit** (`-per-ip-max` / `-per-ip-window`, default 60 per hour). A sliding window over the caller's IP (see `-trusted-proxies` for how the IP is resolved behind a proxy). Trips → `faucet: per-IP rate limit`.
3. **Global daily cap** (`-daily-cap`, default 100,000 GNOT). A hard ceiling on total outflow per **UTC calendar day**; the counter resets at UTC midnight. At the default grant this is ~100 grants/day across all callers. Trips → `faucet: global daily cap reached`.
4. **Global drip token-bucket** (`-drip-burst` / `-drip-rate`, default disabled). A token bucket over total ugnot outflow, independent of address or IP: `-drip-burst` is the largest spike tolerated and `-drip-rate` the sustained refill. Disabled when `-drip-burst` is `0`. Trips → `faucet: global drip rate exceeded`.

Behaviors worth knowing when you set these:

- **Limits are refunded on dispense failure.** They're applied *before* the on-chain send. If the send fails, the grant is refunded — a chain hiccup doesn't consume the requester's cooldown, their per-IP count, or the daily budget.
- **State is in-memory.** Cooldowns, per-IP windows, and the daily counter live in the process. **Restarting the service clears all of them** — there's no persistence, so a restart resets every cooldown and the day's outflow tally.
- **Per-IP limiting ignores `X-Forwarded-For` by default.** With `-trusted-proxies 0` (the default) the client IP is the raw connection peer (`r.RemoteAddr`), so a directly-exposed faucet can't be tricked by a forged `X-Forwarded-For`. Behind a reverse proxy, set `-trusted-proxies` to the number of hops you control (e.g. `1` for a single ALB): the client IP is then taken that many entries in from the right of `X-Forwarded-For` — the side proxies append to — so a client still can't forge a left-hand entry to dodge the per-IP limit. Set it too high and everyone collapses into the proxy's bucket; leave it `0` behind a proxy and all traffic shares the proxy IP.

## HTTP API

The funding endpoint:

```
POST /fund
Content-Type: application/json
{ "address": "g1...", "chain_id": "test-13" }
```

```bash
curl -sX POST http://127.0.0.1:8590/fund \
  -d '{"address":"g1...","chain_id":"test-13"}'
```

| Status | When | Body |
|--------|------|------|
| `200` | granted | `{"tx_hash": "...", "amount_ugnot": 1000000000}` |
| `400` | malformed JSON, empty address, or invalid recipient address | plain text |
| `403` | `chain_id` is not a testnet, or doesn't match this faucet's chain | plain text (`faucet: chain-id …`) |
| `429` | a limit tripped (cooldown / per-IP / daily cap / drip) | the limit message above |
| `502` | the on-chain dispense failed | generic `faucet: dispense failed` (details only in the service logs, to avoid leaking probe signal to anonymous callers) |
| `503` | the funding wallet is below `-min-funding-balance` | `faucet: funding wallet below minimum balance` |

The request body is capped at 4 KiB.

A liveness probe:

```
GET /health   ->  200 "ok"
```

Returns `200` whenever the process is serving; it does not probe the chain, so a transient RPC blip won't fail the check (suited to a load-balancer health check that should only remove a genuinely down instance).

## Metrics & logs

Metrics are **opt-in**: pass `-metrics-addr` to enable Prometheus pull-scraping on a **separate listener** (e.g. `127.0.0.1:8591`), kept off the public fund port. They're off by default; with `-metrics-addr` unset there is no `/metrics` endpoint. A bind failure on the chosen address is fatal at startup (you asked for metrics, so a misconfigured port fails loudly rather than running blind). In a container, bind it explicitly (`-p 8591:8591 … -metrics-addr 0.0.0.0:8591`).

```
GET /metrics   ->  Prometheus text exposition
```

| Metric | Type | Labels | Meaning |
|--------|------|--------|---------|
| `faucet_fund_requests_total` | counter | `outcome` | fund requests by outcome (`success`, `bad_request`, `chain_refused`, `chain_mismatch`, `cooldown`, `rate_limited`, `daily_cap`, `drip_limited`, `funding_low`, `dispense_failed`) |
| `faucet_funding_balance_ugnot` | gauge | — | funding wallet balance, polled every 30s (so a scrape never blocks on an RPC) — alert before it runs dry |
| `faucet_drip_tokens_available` | gauge | — | global drip token-bucket headroom in ugnot (absent when the drip control is disabled) |
| `faucet_daily_cap_remaining_ugnot` | gauge | — | remaining daily outflow budget for the current UTC day |
| `http_server_request_duration_seconds` | histogram | method, status, scheme | request latency, from `otelhttp` |
| `http_server_request_body_size_bytes` | histogram | method, status, scheme | request body size, from `otelhttp` |

Labels are deliberately low-cardinality (a bounded `outcome` enum, HTTP method/status); the recipient address and client IP are **never** metric labels, to avoid unbounded series.

Logs are structured JSON on stdout (one `http_request` line per request):

```json
{"time":"…","level":"INFO","msg":"http_request","method":"POST","route":"POST /fund","status":200,"latency_ms":12,"client_ip":"…","outcome":"success","address":"g1…","chain_id":"test-13"}
```

`/health` lines carry only `method/route/status/latency_ms/client_ip`; the fund-specific `outcome/address/chain_id` appear on `/fund`. Recipient addresses, chain-ids, tx hashes, and the client IP are logged (public on-chain or internal infra). The funding mnemonic and raw request bodies are never logged. Internal dispense errors are logged server-side (for operators) but returned to anonymous callers as a generic `502`.
