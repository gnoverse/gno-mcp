---
name: gno-build
description: Author, build, test, and deploy a Gno realm or pure package. Use whenever the user wants to write a new realm or `/p/` package, add or change a function in one, scaffold a Gno project, or deploy/publish `.gno` code to a chain — even when they only describe the on-chain behavior they want ("make a realm that stores notes", "deploy a counter I can bump", "add a withdraw function") without ever saying "build". This skill owns writing Gno source; reasoning about existing code stays with gno, security audits with gno-audit, failed transactions with gno-debug.
---

# Building a Gno realm or package

Gno reads like Go but its semantics and API are its own — author from the references, not from Go memory. "This function is simple enough to write as plain Go" is exactly the thought that ships a realm that only works by luck.

The shape of the work is: **author and test locally first, decide where it runs only when it's ready to go on-chain.** Don't ask "local devnet or testnet?" up front — that is friction before there is anything to deploy. A realm can be written and made correct with zero chain and zero keys.

1. **Load the knowledge.** Read `../gno/SKILL.md` (the source index), then `../gno/references/build.md` (project layout, `gnomod.toml`, the `gno` binary, test flavors) and `../gno/references/patterns.md` (realm idioms — globals, `init`, crossing-function discipline, state shape). Pull `interrealm.md` for `cur realm` / crossing / caller-identity semantics, `stdlib.md` for the chain API, `render.md` if it exposes `Render()`, `security.md` to avoid known footguns. Load what the realm needs, not all of them.

2. **Pin what to build** — not where. If the on-chain behavior is ambiguous (who may call it, what persists across transactions, what it returns), ask one sharp question now. A wrong state shape is expensive to migrate later. The deploy target is a separate question for step 5; raising it here is premature.

3. **Ground the code in real examples.** Before inventing structure, `gno_read` an existing on-chain realm of a similar shape and match its crossing-function signatures and state layout (outline mode surveys the API; `full=true` reads a file verbatim). Then write it.

4. **Test locally — no chain, no keys.** Run `gno test` and filetests against the source on disk. This loop is friction-free and is where a realm becomes correct; stay in it until tests pass. A realm that compiles is not a realm that works. (If `gno` is not on PATH, fall back to a cheap on-chain check with `gno_eval` / `simulate=true` once a target exists.)

5. **Now pick the target.** Only when the realm is ready to go on-chain, ask where it should run: a local devnet (`gnodev` — fast, throwaway) or a testnet (via a gnomcp profile). This is the right moment for that question; before this point there was nothing to place.

6. **Never touch keys.** Do not run `gnokey` directly, and never read, ask for, or import a keystore or private key. gnomcp signs for you — agent-key writes via `gno_addpkg` / `gno_call` / `gno_run`, user-attributed writes via a session (`gno_session_propose`, which the user authorizes with their own key). Reaching for raw `gnokey` or a key file means you took a wrong turn. Prefer gnomcp for deploying and calling throughout — it is strongly encouraged here.

7. **Security pass before deploy.** Self-check against `security.md`'s bug classes — designation-forgery (caller-identity guards), payment guards, impl-substitution. For anything non-trivial or fund-handling, run the `gno-audit` skill rather than trusting the self-check.

8. **Deploy and prove.** Deploy via gnomcp, then prove it with a real call that returns the expected state, and report which identity signed. Done means deployed, proven, and signer named.
