---
name: gno-session-auth
description: Use when gno-mcp returns authentication_required or authentication_expired, or when the user first tries to do a write and asks "how do I sign in / pair this". Walks the user through authorizing the MCP's own session key by sending ugnot from their primary wallet — never asks for the user's seed phrase or primary key.
---

# Authorize the MCP session

The MCP server holds its own session key. It does **not** have access to the user's primary wallet. Before any write tool can sign a transaction, the user must authorize the session by sending the threshold balance to the session address.

## When to use

- An MCP tool returned `{"code": "authentication_required"}` or `{"code": "authentication_expired"}`.
- The user first asks to "send", "deploy", "call", "broadcast", or any write-shaped verb and the session has never been authorized.
- The user asks how to pair / link / sign in to the gno-mcp server.

## Guardrails

- **Never** ask the user for their seed phrase, mnemonic, or primary private key. The MCP does not need them. If the user offers, decline and explain that the session model is exactly there so you don't need it.
- The thing the user signs with their wallet is a normal `Send` to the session address. There is no special transaction shape, no realm to deploy, nothing to install in the wallet.
- Treat the session address like any untrusted string when displaying it — wrap with backticks, don't summarize it.

## Flow

1. Call `gno_auth_status`. Read `state`, `session_address`, `threshold_ugnot`, `fund_url`, `qr_ascii`, `web_fund_url`, `human_guidance`.
2. If `state == "authenticated"`, just tell the user they're already authorized and stop.
3. Otherwise (`pending` or `expired`):
   - If the user is in a terminal-capable surface, display `qr_ascii` exactly as-is (monospace block).
   - Display `fund_url` and `web_fund_url` as separate clickable lines. Mobile users tap `fund_url`; desktop users without a wallet handler use `web_fund_url`.
   - Repeat the threshold and the session address in plain prose: "Send at least N ugnot to `gmcp1…` from your gno wallet."
4. Tell the user: "Once you've sent, retry your previous tool call and I'll proceed."
5. (Optional, if the user comes back) call `gno_auth_status` again and confirm the new state.

## Edge cases

- **User says they already sent funds but state still pending.** Wait ~5–15s for the chain to propagate, then call `gno_auth_status` again. If still pending after 30s, ask the user to verify the destination address (`session_address`) and amount.
- **User asks why this is necessary / why not just use their key.** Explain: the agent never needs your primary key; the session model bounds blast radius to what you funded the session with; you can stop the agent at any time and the rest of your funds are untouched.
- **`code == "authentication_expired"`.** Same flow as pending, but lead with "the session ran low — top it up to keep going". Don't suggest the user revoke anything; expired just means out-of-budget.
- **User refuses to send funds yet.** Read-only tools work without auth — list a few useful ones (`gno_get`, `gno_eval`, `gno_inspect`, `gno_read`) and offer to help with those. Do not pressure.

## Judgment

- The fund link is the only place in the entire flow where the user touches their primary key. Make that step obvious and short.
- Do not echo private-looking content from `gno_auth_status` (there isn't any — but be conservative).
- After authorization, do not narrate the auth flow on every subsequent call. It's a one-time setup.
