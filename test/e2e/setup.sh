#!/usr/bin/env bash
# test/e2e/setup.sh — Boot gnodev with a funded master account and deploy
# the three test realms. Idempotent: safe to re-run if realms are already
# deployed (gnodev will reject duplicate AddPackage txs with an error that
# the script tolerates).
#
# Usage:
#   ./test/e2e/setup.sh [--with-indexer]
#
# Prerequisites:
#   gnodev and gnokey must be on PATH.
#   Run from the repository root.
#
# Deterministic dev-only mnemonic (NEVER use for real funds):
#   source stamp mouse club drift warfare moral hobby jar gravity venture acquire
#   junior unfold vapor lumber balcony wide regular february gravity together fog
#
# This mnemonic is committed in plaintext intentionally — it is a throwaway
# dev seed used only to fund the gnodev test environment.

set -euo pipefail

# ---- constants
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
KEYRING_DIR="${SCRIPT_DIR}/.keyring"
REALMS_DIR="${SCRIPT_DIR}/realms"
MASTER_KEY_NAME="e2e-master"
CHAIN_ID="dev"
FUND_AMOUNT="1000000000ugnot"
GNODEV_PID=""
WITH_INDEXER=false

# Dev-only mnemonic — committed deliberately; throwaway seed for local testing.
E2E_MNEMONIC="source stamp mouse club drift warfare moral hobby jar gravity venture acquire junior unfold vapor lumber balcony wide regular february gravity together fog"

# ---- arg parsing
for arg in "$@"; do
  case "$arg" in
    --with-indexer)
      WITH_INDEXER=true
      ;;
    *)
      echo "Unknown argument: $arg" >&2
      exit 1
      ;;
  esac
done

if [[ "$WITH_INDEXER" == "true" ]]; then
  echo "INFO: --with-indexer passed — tx-indexer integration not wired yet; running without indexer."
fi

# ---- dependency check
if ! command -v gnodev &>/dev/null; then
  echo "ERROR: gnodev not found on PATH." >&2
  echo "       Install: go install github.com/gnolang/gno/contribs/gnodev@latest" >&2
  exit 1
fi
if ! command -v gnokey &>/dev/null; then
  echo "ERROR: gnokey not found on PATH." >&2
  echo "       Install: go install github.com/gnolang/gno/gno.land/cmd/gnokey@latest" >&2
  exit 1
fi

# ---- allocate free port (26657..26680)
GNODEV_PORT=""
for port in $(seq 26657 26680); do
  if ! lsof -iTCP:"${port}" -sTCP:LISTEN -t &>/dev/null 2>&1; then
    GNODEV_PORT="${port}"
    break
  fi
done
if [[ -z "$GNODEV_PORT" ]]; then
  echo "ERROR: no free port found in range 26657-26680." >&2
  exit 1
fi
echo "INFO: using port ${GNODEV_PORT}"

# ---- keyring setup
mkdir -p "${KEYRING_DIR}"

# Generate deterministic master keypair if not already present.
if ! GNOKEYHOME="${KEYRING_DIR}" gnokey list 2>/dev/null | grep -q "^${MASTER_KEY_NAME}"; then
  echo "INFO: generating ${MASTER_KEY_NAME} keypair..."
  printf '%s\n%s\n' "" "" | \
    GNOKEYHOME="${KEYRING_DIR}" gnokey add "${MASTER_KEY_NAME}" \
      --recover \
      --insecure-password-stdin \
      <<< "${E2E_MNEMONIC}"
fi

MASTER_ADDR=$(GNOKEYHOME="${KEYRING_DIR}" gnokey list 2>/dev/null \
  | awk "/^${MASTER_KEY_NAME}/{print \$3}")
if [[ -z "$MASTER_ADDR" ]]; then
  echo "ERROR: could not determine address for ${MASTER_KEY_NAME}." >&2
  exit 1
fi
echo "INFO: master address: ${MASTER_ADDR}"

# ---- cleanup trap
cleanup() {
  if [[ -n "$GNODEV_PID" ]]; then
    echo ""
    echo "INFO: shutting down gnodev (PID ${GNODEV_PID})..."
    kill "${GNODEV_PID}" 2>/dev/null || true
    wait "${GNODEV_PID}" 2>/dev/null || true
    echo "INFO: gnodev stopped."
  fi
}
trap cleanup EXIT INT TERM

# ---- boot gnodev in background
echo "INFO: starting gnodev on port ${GNODEV_PORT}..."
gnodev \
  --add-account "${MASTER_KEY_NAME}:${MASTER_ADDR}:${FUND_AMOUNT}" \
  --chain-id="${CHAIN_ID}" \
  --listen="127.0.0.1:${GNODEV_PORT}" \
  &>/tmp/gnodev-e2e.log &
GNODEV_PID=$!
echo "INFO: gnodev PID ${GNODEV_PID} (log: /tmp/gnodev-e2e.log)"

# ---- wait for gnodev to produce block height > 0
echo "INFO: waiting for gnodev to produce first block (30s timeout)..."
RPC_BASE="http://127.0.0.1:${GNODEV_PORT}"
DEADLINE=$(( $(date +%s) + 30 ))
while true; do
  if [[ $(date +%s) -ge $DEADLINE ]]; then
    echo "ERROR: gnodev did not produce a block within 30s." >&2
    echo "       Check /tmp/gnodev-e2e.log for details." >&2
    exit 1
  fi
  HEIGHT=$(curl -sf "${RPC_BASE}/status" 2>/dev/null \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['result']['sync_info']['latest_block_height'])" \
    2>/dev/null || echo "0")
  if [[ "$HEIGHT" -gt 0 ]] 2>/dev/null; then
    echo "INFO: gnodev ready at block height ${HEIGHT}."
    break
  fi
  sleep 0.2
done

# ---- deploy test realms
deploy_realm() {
  local name="$1"
  local pkgpath="gno.land/r/test/${name}"
  local pkgdir="${REALMS_DIR}/${name}"
  echo "INFO: deploying ${pkgpath}..."
  # Tolerate "already deployed" errors from gnodev (idempotent re-runs).
  GNOKEYHOME="${KEYRING_DIR}" gnokey maketx addpkg \
    --pkgpath "${pkgpath}" \
    --pkgdir "${pkgdir}" \
    --gas-fee 1ugnot \
    --gas-wanted 5000000 \
    --remote "http://127.0.0.1:${GNODEV_PORT}" \
    --chainid "${CHAIN_ID}" \
    --broadcast \
    "${MASTER_KEY_NAME}" \
  || echo "WARN: deploy of ${pkgpath} failed or already deployed — continuing."
}

deploy_realm echo
deploy_realm counter
deploy_realm other

# ---- ready banner
echo ""
echo "========================================================"
echo "  gnomcp e2e environment ready"
echo "========================================================"
echo "  gnodev port : ${GNODEV_PORT}"
echo "  master key  : ${MASTER_KEY_NAME}"
echo "  master addr : ${MASTER_ADDR}"
echo "  mnemonic    : (see script header — dev-only seed)"
echo "  keyring     : ${KEYRING_DIR}"
echo "  realms      : gno.land/r/test/{echo,counter,other}"
echo "  log         : /tmp/gnodev-e2e.log"
echo ""
echo "  Update test/e2e/profiles.toml rpc-url port if it differs from 26657."
echo "  Run: bin/gnomcp --config test/e2e/profiles.toml"
echo "  Then follow test/e2e/PROTOCOL.md"
echo ""
echo "  Press Ctrl+C to stop gnodev and exit."
echo "========================================================"

# ---- foreground wait (operator presses Ctrl+C)
wait "${GNODEV_PID}"
GNODEV_PID=""
