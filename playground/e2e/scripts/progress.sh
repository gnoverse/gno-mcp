#!/usr/bin/env bash
# progress.sh — print the batch driver's narration lines as they stream.
# Pipe `claude -p --agent playground-driver ... --output-format stream-json --verbose`
# through this; assistant text events ARE the progress feed (one line per
# step/verdict by the agent's own contract).
set -euo pipefail
jq --unbuffered -r 'select(.type=="assistant")
  | .message.content[]? | select(.type=="text") | .text'
