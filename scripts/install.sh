#!/bin/sh
# install.sh — install the gnomcp binary and wire it into coding-agent harnesses.
# Keep this script simple and auditable: read it before piping it to sh.
#
#   curl -fsSL https://raw.githubusercontent.com/gnoverse/gno-mcp/main/scripts/install.sh | sh
#
# Flags (also: --flag=value):
#   --bin-dir DIR     binary install dir            (default: ~/.local/bin)
#   --version TAG     release tag, e.g. v0.1.0      (default: latest)
#   --harness NAME    wire one harness: claude | gemini | codex | opencode | none
#                     (repeatable; default: auto-detect installed harnesses)
#   -h, --help        this text
set -eu

REPO="gnoverse/gno-mcp"
BIN_DIR="${GNOMCP_BIN_DIR:-$HOME/.local/bin}"
VERSION="latest"
HARNESSES=""

info() { printf '%s\n' "$*"; }
warn() { printf 'warning: %s\n' "$*" >&2; }
die()  { printf 'error: %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
install.sh — install the gnomcp binary and wire it into coding-agent harnesses.
Flags (also: --flag=value):
  --bin-dir DIR     binary install dir            (default: ~/.local/bin)
  --version TAG     release tag, e.g. v0.1.0      (default: latest)
  --harness NAME    wire one harness: claude | gemini | codex | opencode | none
                    (repeatable; default: auto-detect installed harnesses)
  -h, --help        this text
EOF
}

need_val() { case "${1:-}" in ''|-*) die "$2 needs a value" ;; esac; }

while [ $# -gt 0 ]; do
  case "$1" in
    --bin-dir=*) BIN_DIR="${1#*=}" ;;
    --bin-dir)   shift; need_val "${1:-}" --bin-dir; BIN_DIR="$1" ;;
    --version=*) VERSION="${1#*=}" ;;
    --version)   shift; need_val "${1:-}" --version; VERSION="$1" ;;
    --harness=*) HARNESSES="$HARNESSES ${1#*=}" ;;
    --harness)   shift; need_val "${1:-}" --harness; HARNESSES="$HARNESSES $1" ;;
    -h|--help)   usage; exit 0 ;;
    *) die "unknown flag: $1 (try --help)" ;;
  esac
  shift
done

for h in $HARNESSES; do
  case "$h" in
    claude|gemini|codex|opencode|none) ;;
    *) die "unknown harness: $h (want claude|gemini|codex|opencode|none)" ;;
  esac
done

# ---- Platform → release asset
case "$(uname -s) $(uname -m)" in
  "Linux x86_64")   target="linux_amd64" ;;
  "Linux aarch64")  target="linux_arm64" ;;
  "Linux arm64")    target="linux_arm64" ;;
  "Darwin x86_64")  target="darwin_amd64" ;;
  "Darwin arm64")   target="darwin_arm64" ;;
  *) die "unsupported platform '$(uname -sm)' — prebuilt binaries: https://github.com/${REPO}/releases" ;;
esac
asset="gno-mcp_${target}.tar.gz"
if [ "$VERSION" = "latest" ]; then
  base="https://github.com/${REPO}/releases/latest/download"
else
  base="https://github.com/${REPO}/releases/download/${VERSION}"
fi

# ---- Download + checksum + install
fetch() { curl --proto '=https' --tlsv1.2 -fsSL "$1" -o "$2" || die "download failed: $1"; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

info "Downloading ${asset} (${VERSION})..."
fetch "${base}/${asset}" "${tmp}/${asset}"
fetch "${base}/checksums.txt" "${tmp}/checksums.txt"

if command -v sha256sum >/dev/null 2>&1; then
  got="$(sha256sum "${tmp}/${asset}" | cut -d' ' -f1)"
elif command -v shasum >/dev/null 2>&1; then
  got="$(shasum -a 256 "${tmp}/${asset}" | cut -d' ' -f1)"
else
  got=""
  warn "no sha256sum/shasum found — skipping checksum verification"
fi
if [ -n "$got" ]; then
  want="$(grep " ${asset}\$" "${tmp}/checksums.txt" | cut -d' ' -f1)"
  [ -n "$want" ] || die "no checksum for ${asset} in checksums.txt"
  [ "$got" = "$want" ] || die "checksum mismatch for ${asset} (got ${got}, want ${want})"
  info "Checksum OK."
fi

# goreleaser archives carry the binary at archive root
tar -xzf "${tmp}/${asset}" -C "${tmp}" gnomcp
mkdir -p "${BIN_DIR}"
mv "${tmp}/gnomcp" "${BIN_DIR}/gnomcp"
GNOMCP="${BIN_DIR}/gnomcp"
ver="$("${GNOMCP}" version)" || die "installed binary failed to run: ${GNOMCP}"
info "Installed ${GNOMCP} (${ver})"
case ":${PATH}:" in
  *":${BIN_DIR}:"*) ;;
  *) warn "${BIN_DIR} is not on your PATH (harness configs below use the absolute path, so this only affects running 'gnomcp' yourself)" ;;
esac

# ---- Harness wiring (skills install via each harness's own plugin manager)
# Fallible commands carry explicit `|| return 1`: POSIX suspends `set -e`
# inside a function whose call is ||-tested, so errors must be routed by hand.
wire_claude() {
  info "Wiring Claude Code (plugin + MCP server)..."
  if ! claude plugin marketplace add "${REPO}" 2>/dev/null; then
    claude plugin marketplace list 2>/dev/null | grep -q gnoverse || return 1
  fi
  claude plugin install gnomcp@gnoverse --scope user || return 1
  claude mcp remove gnomcp --scope user >/dev/null 2>&1 || true
  claude mcp add gnomcp --scope user -- "${GNOMCP}" || return 1
  info "Claude Code done — start (or restart) claude so the plugin loads."
}

wire_gemini() {
  info "Wiring Gemini CLI (extension)..."
  gemini extensions install "https://github.com/${REPO}" || return 1
  info "Gemini CLI done — restart gemini so the extension loads."
}

wire_codex() {
  info "Codex: install the plugin via Codex's plugin flow (manifest: .codex-plugin/plugin.json in"
  info "  https://github.com/${REPO}), then register ${GNOMCP} as an MCP server in your Codex config."
}

wire_opencode() {
  info "OpenCode: add \"gnomcp@git+https://github.com/${REPO}.git\" to the \"plugin\" array in"
  info "  your opencode.json, then restart OpenCode. Details: .opencode/INSTALL.md in the repo."
}

if [ -z "$HARNESSES" ]; then
  command -v claude   >/dev/null 2>&1 && HARNESSES="$HARNESSES claude"
  command -v gemini   >/dev/null 2>&1 && HARNESSES="$HARNESSES gemini"
  command -v codex    >/dev/null 2>&1 && HARNESSES="$HARNESSES codex"
  command -v opencode >/dev/null 2>&1 && HARNESSES="$HARNESSES opencode"
  [ -n "$HARNESSES" ] || warn "no coding-agent harness detected — installed the binary only"
fi

rc=0
for h in $HARNESSES; do
  case "$h" in
    claude)   wire_claude   || { warn "Claude Code wiring failed — the binary is installed at ${GNOMCP}"; rc=1; } ;;
    gemini)   wire_gemini   || { warn "Gemini CLI wiring failed — the binary is installed at ${GNOMCP}"; rc=1; } ;;
    codex)    wire_codex ;;
    opencode) wire_opencode ;;
    none)     ;;
  esac
done

[ "$rc" -eq 0 ] && info "Done."
exit "$rc"
