#!/usr/bin/env bash
# down.sh — remove the e2e AUT container. Idempotent.
set -euo pipefail
docker rm -f "${E2E_CONTAINER:-gnomcp-e2e}" 2>/dev/null || true
echo "DOWN"
