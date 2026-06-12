#!/usr/bin/env bash
# sim-init.sh — ENTRYPOINT of the `sim` image stage: start simnet (in-memory
# node + faucet + gnoweb) in the background, wait until the node serves, then
# hand over to the regular entrypoint with the image CMD (an interactive shell).
set -euo pipefail

# gnoweb binds all interfaces so the docker port publish reaches it; the node
# RPC and faucet keep the loopback default (in-container use only). The ports
# here and in the banner track simnet's flag defaults (e2e/simnet/main.go).
simnet -web-listen 0.0.0.0:8688 > /tmp/simnet.log 2>&1 &

for _ in $(seq 1 50); do
  if curl -sf http://127.0.0.1:26687/status > /dev/null 2>&1; then
    echo "[playground:sim] simnet up — node :26687, faucet :8590, gnoweb :8688, chain test9999 (logs: /tmp/simnet.log)"
    exec /usr/local/bin/entrypoint.sh "$@"
  fi
  sleep 0.2
done

echo "[playground:sim] ERROR: simnet not serving after 10s — log tail:" >&2
tail -n 20 /tmp/simnet.log >&2
exit 1
