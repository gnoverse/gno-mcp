# agentfaucet

A standalone HTTP service that funds agent keys on a testnet. It's a per-chain piece an operator runs and then advertises to gnomcp via a profile's `faucet-service-url` (so `gno_faucet_fund` can top up an agent key automatically). It's independent of the MCP server — the only coupling is HTTP.

It ships as a release binary (`agentfaucet_<os>_<arch>.tar.gz`) and a multi-arch image `ghcr.io/gnoverse/agentfaucet`.

```bash
docker run --rm -p 8590:8590 \
  -e GNOMCP_FAUCET_MNEMONIC="<funding key mnemonic>" \
  ghcr.io/gnoverse/agentfaucet:latest \
  -rpc-url https://rpc.testN.gno.land:443 -chain-id testN -listen 0.0.0.0:8590
```

The funding mnemonic is read from `GNOMCP_FAUCET_MNEMONIC` (never a flag — a flag default leaks to `-help`/logs). The default `-listen` is `127.0.0.1:8590` for host safety, so in a container you must pass `-listen 0.0.0.0:8590`.

Anti-abuse is built in: per-address cooldown, per-IP rate limit, and a hard global daily outflow cap (`-per-addr-cooldown`, `-per-ip-max`, `-per-ip-window`, `-daily-cap`, `-grant`). Only `test*` chain-ids are accepted. `agentfaucet -help` lists every flag.
