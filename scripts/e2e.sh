#!/usr/bin/env bash
set -euo pipefail

# Build and install gno-mcp
cd "$(git rev-parse --show-toplevel)"
go install ./cmd/gno-mcp

# Send MCP initialize + tools/list requests over stdio and verify tool names
INIT_REQ='{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"e2e","version":"0"}}}'
LIST_REQ='{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'

OUT=$( (echo "$INIT_REQ"; echo "$LIST_REQ") | gno-mcp )

for t in gno_network_info gno_get gno_inspect gno_call gno_audit_tail; do
  echo "$OUT" | grep -q "\"$t\"" || { echo "FAIL: missing tool $t"; exit 1; }
done

echo "e2e ok"
