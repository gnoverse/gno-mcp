#!/usr/bin/env bash
# sim-init.sh — ENTRYPOINT of the `sim` image stage: start simnet (in-memory
# node + faucet + gnoweb) in the background, wait until the node serves, then
# hand over to the regular entrypoint with the image CMD (an interactive shell).
set -euo pipefail

# gnoweb binds all interfaces so the docker port publish reaches it; the node
# RPC and faucet keep the loopback default (in-container use only). The ports
# here and in the banner track simnet's flag defaults (e2e/simnet/main.go).
#
# Optional gate/economics overrides, set by the sim-cla image stage:
#   SIMNET_CLA_ARGS  e.g. "-cla-dir <dir> -cla-hash <hash>" (seeds the CLA gate)
#   SIMNET_GRANT     faucet drip in ugnot (e.g. 10000000 = 10 GNOT)
# Both are deliberately word-split into separate simnet flags; unset → base sim.
# shellcheck disable=SC2086 # intentional word-splitting of controlled args
simnet -web-listen 0.0.0.0:8688 ${SIMNET_CLA_ARGS:-} ${SIMNET_GRANT:+-grant $SIMNET_GRANT} \
  > /tmp/simnet.log 2>&1 &

for _ in $(seq 1 50); do
  if curl -sf http://127.0.0.1:26687/status > /dev/null 2>&1; then
    echo "[playground:sim] simnet up — node :26687, faucet :8590, gnoweb :8688, chain test-9999 (logs: /tmp/simnet.log)"
    exec /usr/local/bin/entrypoint.sh "$@"
  fi
  sleep 0.2
done

echo "[playground:sim] ERROR: simnet not serving after 10s — log tail:" >&2
tail -n 20 /tmp/simnet.log >&2
exit 1
