---
name: gno-onboarding
description: Use when gno-mcp returns onboarding_required, or the user says they want to try gno.land for the first time. Creates a testnet key via gnokey, requests faucet funds, and broadcasts a first safe transaction. Never shows the mnemonic — gnokey stores the key, we only display the public address.
---

# Gno Onboarding

## When to use

- gno-mcp returned an `onboarding_required` error.
- User says: "I want to try gno", "how do I start", "set me up on gno.land".

## Guardrails

- **Default network: testnet** (staging.gno.land). Confirm before touching mainnet.
- **Never display or ask for a mnemonic.** The key stays in gnokey. If the user asks for backup, direct them to `gnokey export`.
- **Never broadcast on mainnet without loud confirmation.**

## Flow

1. Ask: testnet (recommended) or mainnet? Default testnet.
2. Call `gno_keygen` with a name the user chose. Surface the `address` and `pubkey` — and only those.
3. Explain: your key lives in gnokey; we do not display the mnemonic; for backup use `gnokey export`.
4. Call `gno_faucet_request` with the network and address.
5. Call `gno_address_info` to confirm the balance is non-zero.
6. Suggest a first CALL, e.g. `gno.land/r/demo/wugnot.Deposit()` on testnet. Echo the `security` block before broadcasting.
7. Call `gno_call` with `confirm=true` once the user says go.
8. Celebrate. Point them at the `gno-explore-realm` skill next.

## If anything fails

- Faucet rate-limited → wait, or switch to a different testnet.
- Key already exists → reuse it, don't overwrite.
- Mainnet intent → stop, confirm explicitly, require the user to spell out the network.
