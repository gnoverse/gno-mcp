#!/usr/bin/env bash
# Tests for scripts/install.sh — re-running it must pick up a newer release.
# Hermetic: stub `curl` serves a fake release, stub harness CLIs model the real
# ones (their exit codes and their staleness), so no network and no real
# harness state is touched.
set -euo pipefail

SCRIPT="$(cd "$(dirname "$0")" && pwd)/install.sh"
readonly SCRIPT
TMPROOT="$(mktemp -d)"
readonly TMPROOT
trap 'rm -rf "$TMPROOT"' EXIT
fails=0
total=0

pass() { total=$((total + 1)); echo "ok $total - $1"; }
fail() {
  total=$((total + 1))
  fails=$((fails + 1))
  echo "FAIL $total - $1"
  [[ ! -s "$RUNLOG" ]] || { echo "--- last install.sh run:"; cat "$RUNLOG"; }
}

check() {
  local want="$1" got="$2" desc="$3"
  if [[ "$want" == "$got" ]]; then
    pass "$desc"
  else
    fail "$desc (want '$want', got '$got')"
  fi
}

export STUB_STATE="$TMPROOT/state"
mkdir -p "$STUB_STATE"
readonly STUBS="$TMPROOT/stubs"
mkdir -p "$STUBS"
readonly BIN_DIR="$TMPROOT/bin"
readonly RUNLOG="$TMPROOT/run.log"

# The version the release/marketplace publishes. Bump it between runs to model
# a new release landing upstream.
publish() { echo "$1" >"$STUB_STATE/available"; }

# ---- Stubs

# curl: build the release archive and its checksum file on demand.
cat >"$STUBS/curl" <<'EOF'
#!/bin/sh
set -eu
dest=""; url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) shift; dest="$1" ;;
    https://*) url="$1" ;;
  esac
  shift
done
[ -n "$dest" ] || exit 1
sha_of() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1"
  else shasum -a 256 "$1"
  fi
}
# A pinned URL carries its tag (/releases/download/vX/), latest does not.
want="$(cat "$STUB_STATE/available")"
case "$url" in
  */releases/download/*) t="${url#*/releases/download/}"; want="${t%%/*}"; want="${want#v}" ;;
esac
case "$url" in
  *.tar.gz)
    # Stage inside the caller's temp dir so its EXIT trap reaps this too.
    d="${dest%/*}/stage"
    mkdir -p "$d"
    printf '#!/bin/sh\necho "gnomcp %s"\n' "$want" >"$d/gnomcp"
    chmod +x "$d/gnomcp"
    tar -czf "$dest" -C "$d" gnomcp
    rm -rf "$d"
    ;;
  *checksums.txt)
    : >"$dest"
    for a in "${dest%/*}"/gno-mcp_*.tar.gz; do
      [ -f "$a" ] || continue
      printf '%s  %s\n' "$(sha_of "$a" | cut -d' ' -f1)" "${a##*/}" >>"$dest"
    done
    ;;
  *) exit 22 ;;
esac
EOF

# claude: models the two staleness traps measured against the real CLI —
# `marketplace add` does not refetch an existing marketplace, and `plugin
# update` reads only the marketplace cache. Both report success either way.
# `marketplace list` reproduces the real multi-line rendering, whose per-entry
# `Source:` line is why matching the org name against it is unsafe.
cat >"$STUBS/claude" <<'EOF'
#!/bin/sh
set -eu
mkt="$STUB_STATE/claude.marketplace"   # cached (possibly stale) version
plg="$STUB_STATE/claude.plugin"        # installed version
avail="$(cat "$STUB_STATE/available")"
case "${1:-} ${2:-} ${3:-}" in
  "plugin marketplace add")
    # An existing marketplace is reported as success without any refetch, so
    # `claude.offline` cannot affect that path.
    [ ! -f "$mkt" ] || { echo "Marketplace 'gnoverse' already on disk"; exit 0; }
    [ ! -f "$STUB_STATE/claude.offline" ] || { echo "clone failed" >&2; exit 1; }
    echo "$avail" >"$mkt"
    echo "Successfully added marketplace: gnoverse" ;;
  "plugin marketplace list")
    echo "Configured marketplaces:"
    [ ! -f "$STUB_STATE/claude.foreign" ] || {
      echo "  > othermkt"
      echo "    Source: Directory (/home/u/code/gnoverse/other)"
    }
    [ ! -f "$mkt" ] || {
      echo "  > gnoverse"
      echo "    Source: GitHub (gnoverse/gno-mcp)"
    } ;;
  "plugin marketplace update")
    [ -f "$mkt" ] || { echo "Marketplace 'gnoverse' not found" >&2; exit 1; }
    [ ! -f "$STUB_STATE/claude.offline" ] || { echo "clone failed" >&2; exit 1; }
    echo "$avail" >"$mkt" ;;
  "plugin install "*)
    [ -f "$mkt" ] || { echo "no such marketplace" >&2; exit 1; }
    [ -f "$plg" ] || cp "$mkt" "$plg"
    echo "Plugin installed or already installed" ;;
  "plugin update "*)
    [ -f "$plg" ] || { echo "Plugin is not installed" >&2; exit 1; }
    cp "$mkt" "$plg" ;;
  "mcp remove "*) rm -f "$STUB_STATE/claude.mcp" ;;
  "mcp add "*) shift 2; echo "$*" >"$STUB_STATE/claude.mcp" ;;
  *) echo "unexpected: $*" >&2; exit 1 ;;
esac
EOF

# gemini: `extensions install` refuses an already-installed extension and
# exits non-zero (gemini-cli extension-manager.ts: "is already installed.
# Please uninstall it first."). Only `extensions update` moves it forward.
cat >"$STUBS/gemini" <<'EOF'
#!/bin/sh
set -eu
ext="$STUB_STATE/gemini.extension"
avail="$(cat "$STUB_STATE/available")"
case "${1:-} ${2:-}" in
  "extensions list") [ ! -f "$ext" ] || echo "gnomcp"  ;;
  "extensions install")
    [ ! -f "$ext" ] || {
      echo 'Extension "gnomcp" is already installed. Please uninstall it first.' >&2
      exit 1
    }
    echo "$avail" >"$ext" ;;
  "extensions update")
    [ ! -f "$STUB_STATE/gemini.offline" ] || { echo "network error" >&2; exit 1; }
    [ -f "$ext" ] || { echo 'Extension "gnomcp" not found.' >&2; exit 1; }
    echo "$avail" >"$ext" ;;
  *) echo "unexpected: $*" >&2; exit 1 ;;
esac
EOF

chmod +x "$STUBS/curl" "$STUBS/claude" "$STUBS/gemini"
export PATH="$STUBS:$PATH"

# The hermeticity claim above is only true while every command install.sh can
# reach resolves to a stub; assert it rather than trusting PATH order.
for cmd in curl claude gemini; do
  [[ "$(command -v "$cmd")" == "$STUBS/$cmd" ]] ||
    { echo "not hermetic: $cmd resolves outside the stub dir" >&2; exit 1; }
done

# Run install.sh for one harness, with any extra flags; print its exit code.
run_install() {
  local harness="$1" rc=0
  shift
  "$SCRIPT" --bin-dir "$BIN_DIR" --harness "$harness" "$@" >"$RUNLOG" 2>&1 || rc=$?
  echo "$rc"
}

installed_binary() { "$BIN_DIR/gnomcp" version | cut -d' ' -f2; }
present() { [[ -f "$1" ]] && echo yes || echo no; }
mcp_registered() { present "$STUB_STATE/claude.mcp"; }
mcp_command() { cat "$STUB_STATE/claude.mcp" 2>/dev/null; }

# ---- Claude Code: install, then two re-runs across a new release

publish 0.9.0
check 0 "$(run_install claude)" "claude: first run succeeds"
check 0.9.0 "$(cat "$STUB_STATE/claude.plugin")" "claude: first run installs the plugin"
check "gnomcp --scope user -- $BIN_DIR/gnomcp" "$(mcp_command)" \
  "claude: registers the installed binary at user scope"

publish 0.10.0
check 0 "$(run_install claude)" "claude: second run succeeds"
check 0.10.0 "$(installed_binary)" "claude: second run updates the binary"
check 0.10.0 "$(cat "$STUB_STATE/claude.plugin")" "claude: second run updates the plugin"
check yes "$(mcp_registered)" "claude: second run keeps the MCP server registered"

check 0 "$(run_install claude)" "claude: third run succeeds"
check 0.10.0 "$(installed_binary)" "claude: third run keeps the binary current"
check 0.10.0 "$(cat "$STUB_STATE/claude.plugin")" "claude: third run keeps the plugin current"
check yes "$(mcp_registered)" "claude: third run keeps the MCP server registered"

# A marketplace the user already had, whose rendered listing mentions the
# gnoverse org, must not be mistaken for ours — ours is absent here, so this
# is a first install and every step has to run.
rm -f "$STUB_STATE"/claude.* "$BIN_DIR/gnomcp"
touch "$STUB_STATE/claude.foreign"
check 0 "$(run_install claude)" "claude: a confusable marketplace does not divert the install"
check 0.10.0 "$(cat "$STUB_STATE/claude.plugin")" "claude: confusable marketplace still installs the plugin"
check yes "$(mcp_registered)" "claude: confusable marketplace still registers the MCP server"
rm -f "$STUB_STATE/claude.foreign"

# A marketplace that cannot be refreshed must fail loudly rather than report a
# successful upgrade that silently left the plugin on its cached version — and
# the MCP registration, which happens first, has to survive it.
publish 0.11.0
touch "$STUB_STATE/claude.offline"
rm -f "$STUB_STATE/claude.mcp"   # so the check below is about *this* run
check 1 "$(run_install claude)" "claude: an unrefreshable marketplace exits non-zero"
check 0.10.0 "$(cat "$STUB_STATE/claude.plugin")" "claude: a failed refresh does not claim the new version"
check yes "$(mcp_registered)" "claude: a failed refresh keeps the MCP server registered"
rm -f "$STUB_STATE/claude.offline"
publish 0.10.0

# ---- Gemini CLI: same cycle, where a repeat `install` is a hard error

rm -rf "$BIN_DIR"
publish 0.9.0
check 0 "$(run_install gemini)" "gemini: first run succeeds"
check 0.9.0 "$(cat "$STUB_STATE/gemini.extension")" "gemini: first run installs the extension"

publish 0.10.0
check 0 "$(run_install gemini)" "gemini: second run succeeds"
check 0.10.0 "$(cat "$STUB_STATE/gemini.extension")" "gemini: second run updates the extension"

check 0 "$(run_install gemini)" "gemini: third run succeeds"
check 0.10.0 "$(cat "$STUB_STATE/gemini.extension")" "gemini: third run keeps the extension current"

# A harness that cannot be wired has to reach the caller as a non-zero exit,
# not just a warning.
touch "$STUB_STATE/gemini.offline"
check 1 "$(run_install gemini)" "gemini: a failed wiring exits non-zero"
rm -f "$STUB_STATE/gemini.offline"

# ---- Pinning: `--version` selects a binary, never a plugin
#
# No harness plugin manager can select a plugin version, so the plugin is left
# alone rather than dragged to the newest release beside a deliberately old
# binary. Runs last: it clears both harnesses' state.
rm -rf "$BIN_DIR"
rm -f "$STUB_STATE"/claude.* "$STUB_STATE"/gemini.*
publish 0.10.0
check 0 "$(run_install claude --version v0.9.0)" "claude: a pinned install succeeds"
check 0.9.0 "$(installed_binary)" "claude: --version pins the binary"
check no "$(present "$STUB_STATE/claude.plugin")" "claude: --version leaves the plugin alone"
check yes "$(mcp_registered)" "claude: --version still registers the MCP server"
check 0 "$(run_install gemini --version v0.9.0)" "gemini: a pinned install succeeds"
check no "$(present "$STUB_STATE/gemini.extension")" "gemini: --version leaves the extension alone"

if [[ $fails -ne 0 ]]; then
  echo "$fails of $total tests failed" >&2
  exit 1
fi
echo "all $total tests passed"
