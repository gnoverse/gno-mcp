#!/usr/bin/env bash
# up.sh — (re)create the e2e AUT container. Always recreates: "reuse" means
# the driver chooses NOT to call this. Prints the container name when ready.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLAYGROUND_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CONTAINER="${E2E_CONTAINER:-gnomcp-e2e}"
IMAGE="${E2E_IMAGE:-gnomcp-playground:e2e}"
ALIAS="testnet.gnomcp.sim"

if [[ ! -f "${PLAYGROUND_DIR}/.env" ]]; then
  echo "ERROR: missing playground/.env — cp playground/.env.example playground/.env and paste a claude setup-token." >&2
  exit 1
fi

docker build -f "${PLAYGROUND_DIR}/Dockerfile" --target e2e -t "${IMAGE}" "${PLAYGROUND_DIR}/.."
docker rm -f "${CONTAINER}" 2>/dev/null || true
docker run -d --name "${CONTAINER}" \
  --env-file "${PLAYGROUND_DIR}/.env" \
  --add-host "${ALIAS}:127.0.0.1" \
  "${IMAGE}"

DEADLINE=$(( $(date +%s) + 30 ))
while true; do
  if [[ $(date +%s) -ge ${DEADLINE} ]]; then
    echo "ERROR: simnet not ready after 30s — docker logs ${CONTAINER}" >&2
    exit 1
  fi
  HEIGHT=$(docker exec "${CONTAINER}" curl -sf "http://${ALIAS}:26687/status" 2>/dev/null \
    | python3 -c 'import sys,json; print(json.load(sys.stdin)["result"]["sync_info"]["latest_block_height"])' \
    2>/dev/null || echo 0)
  [[ "${HEIGHT}" =~ ^[1-9][0-9]*$ ]] && break
  sleep 0.3
done
echo "READY container=${CONTAINER} rpc=http://${ALIAS}:26687 faucet=http://${ALIAS}:8590 gnoweb=http://${ALIAS}:8688"
