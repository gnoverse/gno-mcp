#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${CLAUDE_CODE_OAUTH_TOKEN:-}" ]]; then
  echo "ERROR: CLAUDE_CODE_OAUTH_TOKEN is not set (or empty)." >&2
  echo "  On the host: run 'claude setup-token', then create playground/.env:" >&2
  echo "    cp playground/.env.example playground/.env" >&2
  echo "    # paste the token after CLAUDE_CODE_OAUTH_TOKEN=" >&2
  exit 1
fi

case "${PLAYGROUND_LAYER:-unknown}" in
  l1-fresh)
    echo "[playground:fresh] Clean Claude Code — no gno plugin/skill/MCP, no gnodev."
    echo "  Run 'claude' to start; inside it install as a user would:"
    echo "    /plugin marketplace add gnoverse/gno-mcp"
    ;;
  l2-gnomcp)
    echo "[playground:gnomcp] gno skill + auditor agent + gnomcp MCP pre-installed (testnet, no local node)."
    echo "  Run 'claude' to start. The gnomcp binary is on PATH."
    ;;
  l3-full)
    echo "[playground:full] gno skill family + agent + MCP + gno + gnodev all available."
    echo "  Test realm source locally:  gno test ./...   (no chain, no keys)"
    echo "  Start a local devnet:  gnodev   (e.g. in a tmux pane; gnomcp 'local' auto-discovers :26657)"
    echo "  Then run 'claude' to start."
    ;;
  sim)
    echo "[playground:sim] simulated testnet serving — node :26687, faucet :8590, gnoweb :8688 (chain test-9999)."
    echo "  The 'testnet' profile is pre-pointed at it. Run 'claude' to start; gnodev stays manual."
    ;;
  sim-cla)
    echo "[playground:sim-cla] simulated testnet with the test13 CLA deploy gate seeded — a deploy must sign r/sys/cla first."
    echo "  Faucet drip ${SIMNET_GRANT:-?} ugnot; at the minimum gas fee this clears the whole Sign+deploy+bump flow."
    echo "  node :26687, faucet :8590, gnoweb :8688 (chain test-9999). Run 'claude' to start."
    ;;
  *)
    echo "[playground] layer: ${PLAYGROUND_LAYER:-unknown}"
    ;;
esac

exec "$@"
