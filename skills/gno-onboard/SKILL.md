---
name: gno-onboard
description: Teach gno.land from first contact, adapting to the user's background. Use when someone asks "what is gno", "how do I start with gno.land", "explain gno to me", says they are new to Gno or coming from another stack (Solidity, Solana, web2, anything), or asks beginner questions about realms, gnokey, or testnets — even without the word "onboard".
---

# Onboarding to gno.land

Read `../gno/SKILL.md` first (source index). This flow is interview-first, hands-on, one
concept per step. Never lecture-dump.

## Step 1 — Gauge (mandatory, never skip)

Use AskUserQuestion (or plain-text questions when that tool is unavailable), ONE question at
a time. If the opening message already answers a question, don't re-ask it — acknowledge and
sharpen instead (e.g. a Solidity dev gets asked about Go familiarity, not "do you know
blockchains?").

1. Background — open: "what do you already know?" (options like new-to-blockchain / another
   chain / some Gno are fine as answering shortcuts, but treat the answer as *context to
   adapt with*, never as a category that locks the rest of the flow)
2. Goal — what do they want to build or understand?

<HARD-GATE> No key generation, no faucet call, no transaction, no deploy until the gauge is
done AND the user explicitly agrees to the hands-on part. One explicit yes covers key
generation + faucet + the first call; read-only observation needs no gate. Still announce
the broadcast moment before sending anything for real. </HARD-GATE>

## Step 2 — Adapt (no buckets, no per-background scripts)

Meet the user in their own vocabulary. If they know another stack, build the bridge from THAT
stack — but ground every Gno-side claim by reading the relevant `../gno/references/` file
first (`interrealm.md` for caller identity and crossing, `patterns.md` for idioms,
`stdlib.md` for the API surface, `build.md` for project setup). Analogies are bridges, not
specs: when one breaks (caller identity, storage model, the absence of a bytecode/source
split), say so explicitly rather than letting the analogy carry the claim.

If they know nothing about blockchains, explain realm / account / transaction in plain words
(3–4 concrete sentences each) and skip comparisons entirely.

Depth is continuous: check understanding before moving on; simplify or accelerate based on
their answers, not on the initial label.

## Step 3 — Hands-on (see before do; one new concept per step; checkpoint after each)

1. **Observe:** `gno_render` a live realm (e.g. `gno.land/r/gnoland/home`), then
   `gno_inspect` it — "this is a contract, and you can read all of it". If the client elides
   resource previews, hand over the gnoweb URL instead.
2. **Identity:** `gno_key_address` → explain the agent key; `gno_key_generate` on testnet if
   none exists yet.
3. **Fund:** `gno_faucet_fund` — testnet coins are free and worthless; that is the point.
   Funding precedes even `simulate`: an unfunded account cannot sign a dry run. If the
   profile has no faucet configured, say so, give the manual-funding address, and either
   switch to a local gnodev (`test1` is pre-funded, the whole flow works in seconds) or
   continue in describe-only mode.
4. **First write:** the smallest possible `gno_call` against a test realm (e.g.
   `gno.land/r/demo/counter` — verify it exists with `gno_inspect` first) — `simulate=true`,
   show the gas, then broadcast. Always say which identity signed.
5. **Close:** match their goal — if it was deploying, close the loop with a `gno_addpkg`
   `simulate=true` of a ~10-line counter realm before pointing at docs; writing a realm →
   the gno skill's `build.md`/`patterns.md`; safety review → `/gno-audit`; something
   failed → `/gno-debug`.

## Rules

- Be concrete: real commands, real outputs, real numbers. Never a shill — testnets reset,
  sessions are WIP, mainnet is deliberately out of reach.
- No mnemonic ever appears; keys stay in gnokey or the gnomcp keystore.
