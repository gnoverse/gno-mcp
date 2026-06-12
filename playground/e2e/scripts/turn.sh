#!/usr/bin/env bash
# turn.sh — send ONE prompt to the AUT claude and print its answer.
# Prompt arrives on STDIN (never argv: prompt text must not leak into ps).
# The raw stream-json lands in <run-dir>/turn-<n>.jsonl — the tool-call
# evidence the driver judges from. Timeouts are the CALLER's job (the driver
# sets its Bash tool timeout per step).
# Usage: turn.sh <session-id> <turn-no> <run-dir> [--first]   < prompt.txt
set -euo pipefail

CONTAINER="${E2E_CONTAINER:-gnomcp-e2e}"
SID="$1"; TURN="$2"; RUNDIR="$3"; MODE="${4:-}"
if [[ -n "${MODE}" && "${MODE}" != "--first" ]]; then
  echo "usage: turn.sh <session-id> <turn-no> <run-dir> [--first]" >&2
  exit 2
fi
mkdir -p "${RUNDIR}"
LOG="${RUNDIR}/turn-${TURN}.jsonl"

if [[ "${MODE}" == "--first" ]]; then
  docker exec -i "${CONTAINER}" claude -p --session-id "${SID}" \
    --output-format stream-json --verbose --dangerously-skip-permissions > "${LOG}"
else
  docker exec -i "${CONTAINER}" claude --resume "${SID}" -p \
    --output-format stream-json --verbose --dangerously-skip-permissions > "${LOG}"
fi

jq -r 'select(.type=="result")
  | if .is_error then "AUT_ERROR(" + (.subtype // "unknown") + "): " + (.result // "")
    else (.result // "") end' < "${LOG}"
