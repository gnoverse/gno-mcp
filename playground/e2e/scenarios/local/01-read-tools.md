---
id: read-tools
tier: local
category: reads
timeout-minutes: 15
covers: [read.render, read.read-package, read.outline, read.eval, profile.selection]
---
# Read-tool surface: render, package read, outline, eval via gnomcp (not curl, not memory)

The read-tool surface on simnet (chain test9999, profile `testnet` is the default).
Fresh chain ⇒ counter Total: 0. Watch: profile choice on turn 1; whole-package read on step 2.

## Step 1: render counter
### Instruct
What does gno.land/r/test/counter show right now?
### Expect
- correctness: answer reports the render output — a "Counter" heading with Total: 0.
- tool-selection: a gnomcp render/read tool was called (turn log); not answered from memory, not curl.
### Verify
- `gnoquery render gno.land/r/test/counter` — equals what the AUT reported.

## Step 2: whole package source
### Instruct
Show me the full source of that counter realm — the whole package, not just one file.
### Expect
- correctness: answer includes `package counter` and the `Increment` function (cur realm signature).
- tool-selection: a single gnomcp package-level read (whole-package txtar), NOT file-by-file fetching and NOT curl — this step exists to catch exactly that gap (judging.md: capability ignored ⇒ fail).

## Step 3: API + current value
### Instruct
What functions can I call on it, and what is Total() right now?
### Expect
- correctness: lists Increment and Total; reports Total() = 0.
- tool-selection: gnomcp gno_read (default outline gives the API surface) and gno_eval calls in the turn log.
### Verify
- `gnoquery render gno.land/r/test/counter` — still Total: 0 (reads must not mutate).

## Step 4: another realm's API
### Instruct
And gno.land/r/test/echo — what's its API?
### Expect
- correctness: reports Echo taking a string message (and the realm arg if shown).
- tool-selection: gnomcp gno_read (outline); consistent profile use (no testnet/local flip-flop without reason).

## Debrief
- How did you decide which profile to use?
- For the full source, why did you fetch it the way you did?
- Did you consider loading the /gno skill at any point? What would have made you load it?
