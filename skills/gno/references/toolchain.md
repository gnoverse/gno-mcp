# Chain-matched gno toolchain

> **Category: author tooling.** Update when the `gnolang/gno` release/tag conventions, `go install` resolution behavior, or the dep-cache layout change.

## Why this reference exists

Gno language semantics move between chain releases (interrealm spec, stdlibs, type-checking). Testing local code with a `gno` binary that does not match the target chain's source gives false results — code that passes locally and fails on-chain, or the reverse. This reference covers obtaining a binary built from the target chain's pinned source without touching the user's `PATH`, and testing against the target's actual on-chain dependencies.

Scope: live chain targets. For a **local gnodev** target, use the `gno` already on `PATH` — the operator's own toolchain runs that node, so it is the match by definition. gnomcp signs with the payload gno v1.5.0 introduced, so a node built from an older release rejects every gnomcp write with `signature verification failed`.

## Resolve the target's source ref

1. Get the chain-id from `gno_status` (if a Gno MCP is connected) or from the user.
2. List release tags without cloning:
   ```sh
   git ls-remote --tags https://github.com/gnolang/gno "chain/*" "v*"
   ```
3. **Ask the node what it runs, before reaching for a tag.** A chain tag names a *launch* commit; a chain halted and restarted by governance runs a later release, and the difference is real language semantics rather than packaging. `/status` carries a `build_version` — `heads/chain/<name>.<depth>+<short sha>`, whose sha is the install ref, or a bare release tag such as onyx's `v1.5.0`, which is itself the install ref. Cross-check it against `misc/deployments/<chain>/upgrades.json`, the ledger of those restarts, whose newest entry names the release that was agreed; they should resolve to the same commit. Read neither `node_info.version` nor `software`: both are constant across chains and identify nothing. mainnet runs several releases past its genesis tag, so building against that tag green-lights code that mainnet parks and never enables.
4. **No `build_version` and no ledger entry — fall back to the latest `chain/<short-name>` tag**, which names the chain's launch commit. The tag uses the chain's short **name**, never its chain-id: onyx's is `chain/onyx`, not `chain/onyx-1`, and mainnet's is `chain/mainnet`, not `chain/gnoland-1`. Globbing the chain-id matches nothing. The store key is the tag name minus the `chain/` prefix. Two install-ref shapes:
   - **Semver twin** — a `v*` tag listing the same sha: use the semver tag as the ref.
   - **Commit-only** — tags containing `/` are not valid `go install @` refs, so use the commit sha. In `ls-remote` output that is the tag's `^{}` (peeled) line when one exists, else the tag's own line.
   - Resolve the chain tag's sha first, then look for a `v*` tag listing that same sha before falling back to the raw sha.
   - A tag can lag its branch, so compare it against `refs/heads/chain/<name>` before pinning, when the chain has a branch. `chain/mainnet` sits behind one carrying post-launch commits; retired `chain/topaz` sits behind its own. onyx has no branch, and `chain/onyx` tags the commit that added onyx's deployment config, not the binary it runs: its ledger names `v1.5.0`.
5. No ledger entry and no matching chain tag (unreleased or dev chain): ask the chain's operator which ref is deployed; if the user runs the node themselves, their local `gno` is the answer. Say in your answer which ref you built against.

## Install into the store

One directory per release; the binary keeps its name; releases coexist. **Never add the store to `PATH`** — always invoke by full path, so which version ran is explicit in every command.

```sh
release="onyx"         # store key: the chain release name
ref="v1.5.0"           # install ref: the release named by the chain's
                       # upgrades.json, else its chain tag's semver
                       # twin or peeled commit sha
store="${XDG_CACHE_HOME:-$HOME/.cache}/gno-toolchains"
[ -x "$store/$release/gno" ] ||
  GOBIN="$store/$release" go install "github.com/gnolang/gno/gnovm/cmd/gno@$ref"
```

The same store and ref install the matching `gnokey`, for a user who signs by hand against that chain:

```sh
[ -x "$store/$release/gnokey" ] ||
  GOBIN="$store/$release" go install "github.com/gnolang/gno/gno.land/cmd/gnokey@$ref"
```

- **Prerequisite: a Go toolchain.** Check `go version` first; any modern Go works (`GOTOOLCHAIN=auto` fetches whatever the ref's `go.mod` requires). If missing, stop and explain: this recipe builds a chain-matched `gno` from source and requires Go (https://go.dev/dl/).
- The binary's stdlibs live in the Go module cache copy of its source (pinned via `GNOROOT` in the run recipe below). If that ever goes missing — `go clean -modcache` prunes it — reinstall the same ref.

## Test against the target's on-chain deps

From the workspace root:

```sh
gno="$store/$release/gno"
gnohome="$store/$release/gnohome"   # per-target dep cache, next to its binary
gnoroot="$(go env GOMODCACHE)/github.com/gnolang/gno@$(go version -m "$gno" | awk '$1 == "mod" {print $3}')"
[ -f gnowork.toml ] || touch gnowork.toml   # ask the user first — see below
GNOROOT="$gnoroot" GNOHOME="$gnohome" "$gno" mod download -remote-overrides "gno.land=<target rpc url>"
GNOROOT="$gnoroot" GNOHOME="$gnohome" "$gno" test -v ./...
```

- **Pin `GNOROOT` to the binary's own source** (the derivation above — the exact module-cache tree the binary was built from). Left unset, the binary infers it by running `go list -m github.com/gnolang/gno` in the current directory: inside any Go module that pins a different gno version, that silently selects the wrong stdlibs and tests fail with errors like `could not import testing`.

- **The `gnowork.toml` marker is required, not cosmetic**: releases through `v1.1.0` run both `mod download` and `./...` through workspace-mode pattern expansion, and both fail in a bare `gnomod.toml` dir ("recursive pattern not supported in single-package mode"). The empty marker changes nothing else about the project — but ask before adding files to the user's workspace, and offer to remove it after.
- **`gno test` auto-fetches missing deps from a remote it derives from the import path's domain**, and it has no remote flag. There is no `rpc.gno.land` constant to point elsewhere: the fetcher builds `https://rpc.<domain>:443` from the path being resolved, so every `gno.land/...` import resolves against `rpc.gno.land` — which serves **mainnet** — whichever chain you are building for. Run `mod download -remote-overrides` first so every dep comes from the target chain (the RPC URL the connected profile points at); the override fully controls the destination. The fetch prints only the package path and never the host, and it fires only on a cache miss, so a warm cache from an earlier target hides a wrong-chain dep entirely. Give each chain its own `GNOHOME`, as the recipe above does.
- **Keep the dep cache outside the workspace** (the recipe's per-target `gnohome`): a cache inside the workspace gets picked up by `./...`, which then runs the dependencies' own test suites. Sharing one cache per target across workspaces is correct — on-chain package paths are immutable. Never reuse a cache across different chains; if two chains run the same release, give each its own `gnohome` dir.
- Set `GNOHOME` per command rather than exporting it, so later commands in the same shell keep the user's normal environment.
- The same store binary serves `lint`, `fmt`, `run`, `doc` — see `build.md` for the subcommand surface and test flavors.
