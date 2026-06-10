#!/usr/bin/env bash
# test/e2e/teardown.sh — Stop gnodev and clean all e2e state.
# Run from the repository root after finishing a protocol run.
# Safe to run multiple times (tolerates already-gone processes/dirs).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KEYRING_DIR="${SCRIPT_DIR}/.keyring"
SESSION_BASE="${HOME}/.local/share/gnomcp/sessions"
AUDIT_LOG="${HOME}/.local/share/gnomcp/audit.jsonl"

echo "INFO: stopping gnodev..."
# Match setup.sh's launch: `gnodev local ... -node-rpc-listener 127.0.0.1:<port>`.
if pids=$(pgrep -f 'gnodev local.*-node-rpc-listener' 2>/dev/null); then
  for pid in $pids; do
    kill "$pid" 2>/dev/null && echo "INFO: killed gnodev PID ${pid}." || true
  done
else
  echo "INFO: no running gnodev process found."
fi

echo "INFO: removing keyring..."
rm -rf "${KEYRING_DIR}"

echo "INFO: removing session state for 'local' and 'local-safe' profiles..."
rm -rf "${SESSION_BASE}/local"
rm -rf "${SESSION_BASE}/local-safe"

echo "INFO: removing audit log..."
rm -f "${AUDIT_LOG}"

echo "INFO: removing gnodev log..."
rm -f /tmp/gnodev-e2e.log

echo "Teardown complete."
