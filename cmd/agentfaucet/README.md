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
| `-grant` | `1000000000` | ugnot dispensed per drip (1,000 GNOT — ugnot is micro-GNOT, 1 GNOT = 1,000,000 ugnot) |
| `-per-addr-cooldown` | `24h` | minimum time between grants to the same address |
| `-per-ip-max` | `5` | max grants per IP per `-per-ip-window` |
| `-per-ip-window` | `1h` | sliding window for the per-IP limit |
| `-daily-cap` | `100000000000` | hard global daily ugnot outflow cap (100,000 GNOT) |

`agentfaucet -help` lists them all. `agentfaucet version` prints the build version.

## Limits & anti-abuse

Three independent limits gate every grant. They are checked in order — **per-address → per-IP → daily cap** — and the first one that trips rejects the request with HTTP `429` and a specific message:

1. **Per-address cooldown** (`-per-addr-cooldown`, default 24h). At most one grant per address per window, as a sliding window. The address is canonicalized from bech32 first, so case variants of the same address share one bucket and a malformed address is rejected (`400`) before any limit is touched. Trips → `faucet: address in cooldown`.
2. **Per-IP rate limit** (`-per-ip-max` / `-per-ip-window`, default 5 per hour). A sliding window over the caller's IP, taken from the TCP remote address. Trips → `faucet: per-IP rate limit`.
3. **Global daily cap** (`-daily-cap`, default 100,000 GNOT). A hard ceiling on total outflow per **UTC calendar day**; the counter resets at UTC midnight. At the default grant this is ~100 grants/day across all callers. Trips → `faucet: global daily cap reached`.

Behaviors worth knowing when you set these:

- **Limits are refunded on dispense failure.** They're applied *before* the on-chain send. If the send fails, the grant is refunded — a chain hiccup doesn't consume the requester's cooldown, their per-IP count, or the daily budget.
- **State is in-memory.** Cooldowns, per-IP windows, and the daily counter live in the process. **Restarting the service clears all of them** — there's no persistence, so a restart resets every cooldown and the day's outflow tally.
- **Per-IP limiting uses the raw connection IP.** There's no `X-Forwarded-For` parsing. If you front the faucet with a reverse proxy or load balancer, every request appears to come from the proxy and shares a single per-IP bucket — so the per-IP limit collapses into a near-global one. Terminate connections accordingly, or rely on the per-address and daily limits.

## HTTP API

A single endpoint:

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
| `429` | a limit tripped (cooldown / per-IP / daily cap) | the limit message above |
| `502` | the on-chain dispense failed | generic `faucet: dispense failed` (details only in the service logs, to avoid leaking probe signal to anonymous callers) |

The request body is capped at 4 KiB.
