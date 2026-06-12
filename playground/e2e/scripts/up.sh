#!/usr/bin/env bash
# up.sh [target] — (re)create the e2e AUT container from a playground image
# target (default: e2e — the simnet-backed harness image). Always recreates:
# "reuse" means the driver chooses NOT to call this. Prints the container name
# when ready. Non-e2e targets (l1-fresh, l2-gnomcp, l3-full) have no simnet
# main process — the container idles so the driver can `docker exec` AUT turns.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLAYGROUND_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
TARGET="${1:-e2e}"
case "${TARGET}" in
  e2e|l1-fresh|l2-gnomcp|l3-full) ;;
  *) echo "usage: up.sh [e2e|l1-fresh|l2-gnomcp|l3-full]" >&2; exit 2 ;;
esac
CONTAINER="${E2E_CONTAINER:-gnomcp-e2e}"
IMAGE="${E2E_IMAGE:-gnomcp-playground:${TARGET}}"
ALIAS="testnet.gnomcp.sim"

if [[ ! -f "${PLAYGROUND_DIR}/.env" ]]; then
  echo "ERROR: missing playground/.env — cp playground/.env.example playground/.env and paste a claude setup-token." >&2
  exit 1
fi

docker build -f "${PLAYGROUND_DIR}/Dockerfile" --target "${TARGET}" -t "${IMAGE}" "${PLAYGROUND_DIR}/.."
docker rm -f "${CONTAINER}" 2>/dev/null || true

if [[ "${TARGET}" != "e2e" ]]; then
  docker run -d --name "${CONTAINER}" \
    --env-file "${PLAYGROUND_DIR}/.env" \
    "${IMAGE}" sleep infinity
  echo "READY container=${CONTAINER} layer=${TARGET}"
  exit 0
fi

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
