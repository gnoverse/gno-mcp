# Chain-matched gno toolchain

> **Category: author tooling.** Update when the `gnolang/gno` release/tag conventions, `go install` resolution behavior, or the dep-cache layout change.

## Why this reference exists

Gno language semantics move between chain releases (interrealm spec, stdlibs, type-checking). Testing local code with a `gno` binary that does not match the target chain's source gives false results — code that passes locally and fails on-chain, or the reverse. This reference covers obtaining a binary built from the target chain's pinned source without touching the user's `PATH`, and testing against the target's actual on-chain dependencies.

Scope: live chain targets. For a **local gnodev** target, use the `gno` already on `PATH` — the operator's own toolchain runs that node, so it is the match by definition.

## Resolve the target's source ref

1. Get the chain-id from `gno_status` (if a Gno MCP is connected) or from the user.
2. List release tags without cloning:
   ```sh
   git ls-remote --tags https://github.com/gnolang/gno "chain/*" "v*"
   ```
3. Pick the **latest `chain/<chain-id>*` tag**. The store key is the tag name minus the `chain/` prefix (e.g. `gnoland1.1`). Two install-ref shapes:
   - **Semver twin** — a `v*` tag listing the same sha (e.g. `chain/gnoland1.1` = `v1.1.0`): use the semver tag as the ref.
   - **Commit-only** (e.g. `chain/test12`): tags containing `/` are not valid `go install @` refs — use the commit sha. In `ls-remote` output that is the tag's `^{}` (peeled) line when one exists, else the tag's own line.
4. No matching chain tag (unreleased or dev chain): ask the user which ref tracks their chain; if they operate the node themselves, their local `gno` is the answer.

## Install into the store

One directory per release; the binary keeps its name; releases coexist. **Never add the store to `PATH`** — always invoke by full path, so which version ran is explicit in every command.

```sh
release="gnoland1.1"   # store key: the chain release name
ref="v1.1.0"           # install ref: semver twin, or peeled commit sha
store="${XDG_CACHE_HOME:-$HOME/.cache}/gno-toolchains"
[ -x "$store/$release/gno" ] ||
  GOBIN="$store/$release" go install "github.com/gnolang/gno/gnovm/cmd/gno@$ref"
```

- **Prerequisite: a Go toolchain.** Check `go version` first; any modern Go works (`GOTOOLCHAIN=auto` fetches whatever the ref's `go.mod` requires). If missing, stop and explain: building a chain-matched `gno` requires Go (https://go.dev/dl/) — the releases ship no prebuilt binaries.
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
- `gno test` auto-fetches missing deps **only from the default `rpc.gno.land`** — it has no remote flag. Run `mod download -remote-overrides` first so every dep comes from the target chain (the RPC URL the connected profile points at). The override fully controls the destination; there is no silent fallback.
- **Keep the dep cache outside the workspace** (the recipe's per-target `gnohome`): a cache inside the workspace gets picked up by `./...`, which then runs the dependencies' own test suites. Sharing one cache per target across workspaces is correct — on-chain package paths are immutable. Never reuse a cache across different chains; if two chains run the same release, give each its own `gnohome` dir.
- Set `GNOHOME` per command rather than exporting it, so later commands in the same shell keep the user's normal environment.
- The same store binary serves `lint`, `fmt`, `run`, `doc` — see `build.md` for the subcommand surface and test flavors.
