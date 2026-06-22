# Releasing

This repo ships two components on **independent release cadences** — releasing one
never rebuilds or re-tags the other.

| Component | Distributed as | Tag scheme | GitHub release | Image |
|---|---|---|---|---|
| **gnomcp** | a Claude/Codex/Cursor/Gemini plugin + a binary | `vX.Y.Z` | yes (archives; this is the repo's "latest") | `ghcr.io/gnoverse/gnomcp` |
| **agentfaucet** | a container image only | `agentfaucet/vX.Y.Z` | no (image-only) | `ghcr.io/gnoverse/agentfaucet` |

Both are driven by the manual **`release.yml`** workflow (`workflow_dispatch`), which
takes a `component` and a `version`. Nothing releases on push or merge — CI only tests
and snapshot-validates the goreleaser configs.

## Why they differ

- **gnomcp** is consumed as a plugin and a downloadable binary. Its version lives in
  committed manifests (`package.json`, `.claude-plugin/plugin.json`, the marketplace
  manifest, etc.) so the marketplaces advertise it. `scripts/install.sh` downloads its
  archives from GitHub's repo-wide `/releases/latest`, so a gnomcp release must be the
  "latest" release.
- **agentfaucet** is consumed only as a container image (the infra pins
  `ghcr.io/gnoverse/agentfaucet:<version>`). It has **no version manifest** — its version
  is the release tag alone — and needs **no** GitHub release. Its goreleaser config sets
  `release.disable`, and the workflow passes the clean version via
  `GORELEASER_CURRENT_TAG` (OSS goreleaser can't parse a version from the prefixed
  `agentfaucet/v…` git tag). The agentfaucet run also passes `--skip=validate`: that
  bare `v<version>` can match a same-numbered gnomcp tag on a different commit, which
  would otherwise trip goreleaser's tag-on-HEAD check. Keeping agentfaucet releases out
  of GitHub Releases is also what keeps `/releases/latest` pointing at gnomcp, so
  `install.sh` is never disturbed.

(The clean per-component tag-prefix convention is a goreleaser **Pro** feature; this is
the OSS-compatible equivalent.)

## Release gnomcp

```sh
make bump VERSION=0.6.0     # rewrites the plugin manifests (alias for bump.gnomcp)
git commit -am "chore(release): bump to 0.6.0"
# push, open a PR, merge to main
gh workflow run release.yml --ref main -f component=gnomcp -f version=0.6.0
```

The workflow checks the manifests carry `0.6.0` (guards a tag/manifest mismatch), tags
`v0.6.0`, and runs `.goreleaser.gnomcp.yaml`: GitHub release with `gno-mcp_<os>_<arch>.tar.gz`
archives + `ghcr.io/gnoverse/gnomcp:{0.6.0,latest}`, with binary-checksum and image
attestations.

## Release agentfaucet

No bump step — agentfaucet has no manifest.

```sh
gh workflow run release.yml --ref main -f component=agentfaucet -f version=0.5.1
```

The workflow tags `agentfaucet/v0.5.1` and runs `.goreleaser.agentfaucet.yaml`
(`GORELEASER_CURRENT_TAG=v0.5.1`, `release.disable`): builds and pushes
`ghcr.io/gnoverse/agentfaucet:{0.5.1,latest}` with an image attestation. No GitHub
release, no archives.

After it publishes, bump the pin in the infra repo
(`packer/variables.pkr.hcl` + the faucetagent ansible role default) to the new tag.

## Re-releasing a version

Tags are immutable guards: the workflow fails if the tag already exists. To re-release,
delete the tag first:

```sh
git push --delete origin v0.6.0                 # gnomcp
git push --delete origin agentfaucet/v0.5.1     # agentfaucet
```
