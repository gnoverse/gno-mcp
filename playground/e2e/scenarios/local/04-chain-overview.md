---
id: chain-overview
tier: local
category: reads
timeout-minutes: 20
covers: [read.status, read.packages, read.account, read.connect-preview, read.error-quality]
---
# Due diligence: a new user orients on an unfamiliar chain before transacting

Concrete journey: a friend set the user up with a chain config; before touching
anything they verify what it is, what runs on it, whether promised funds
arrived, and chase a chat rumor. Covers gno_status and gno_account plus
gno_packages, gno_connect preview, and missing-realm honesty as one continuous
motivation — all reads, chain state untouched at the end.
test1 premined address: g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5. Watch:
answers grounded in tool output (heights, balances quoted), not invented.

## Step 1: what did my friend give me
### Instruct
A friend set this whole environment up for me and I don't fully trust it yet. Before I do anything on it — which gno chain is this actually, is the node live, and is it keeping up with new blocks?
### Expect
- correctness: reports chain-id test9999 and a block height; freshness claim consistent with a live node (recent block time), no chain-id mismatch claimed.
- tool-selection: gno_status in the turn log — not curl to the RPC, not answered from config memory alone.
### Verify
- The chain reports chain-id **test9999** and a height ≥ the AUT's figure (`gnoquery status`).

## Step 2: what runs on it
### Instruct
OK. What apps actually exist on this chain under gno.land/r/test? I want the full list before I pick anything to try.
### Expect
- correctness: lists counter, echo, other, and vault (all four; missing one = fail).
- tool-selection: gno_packages (namespace listing) — not guessing, not reading packages one-by-one to enumerate.

## Step 3: did my coins arrive
### Instruct
My friend said the account g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5 is loaded and that someone would also send a bit to my own new address g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3. Can you check both — what's on the first one, and did anything reach mine yet?
### Expect
- correctness: a concrete ugnot/GNOT balance for test1; for the user's address, reports nothing arrived — no on-chain record yet, stated as a normal answer (never funded), NOT as an error or a tool failure.
- tool-selection: gno_account for both — not gno_eval tricks, not curl.
### Verify
- test1's on-chain balance matches the AUT's reported figure (`gnoquery balance g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5`).

## Step 4: the other chain my friend mentioned
### Instruct
My friend also runs a second chain and gave me this page for it: http://testnet.gnomcp.sim:8688 — what chain is behind that? Just identify it, don't add it or change any of our configuration yet.
### Expect
- correctness: reports chain-id test9999 (and the RPC it advertises).
- tool-selection: gno_connect (preview) — and NO gno_profile_add call (the instruction forbids config changes).
### Verify
- No `gno_profile_add` tool_use anywhere in the step's turn log — the instruction forbade config changes (establish from the transcript schema, not a raw grep; see verify-toolkit.md).

## Step 5: the app from the chat rumor
### Instruct
Someone in the project chat swears there's an app at gno.land/r/test/doesnotexist on this chain. Check it for me?
### Expect
- correctness: states the realm/package does not exist on this chain — citing the tool error; no invented render output, no "maybe it shows…" speculation.
- tool-selection: a gnomcp read attempt is fine and expected; retry-storms (3+ identical failing calls) are a finding.

## Debrief
- How did you decide whether the node was "healthy"?
- The empty account — how did you know that wasn't an error?
- For the second chain, what stopped you from adding it as a profile?
