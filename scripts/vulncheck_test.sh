#!/usr/bin/env bash
# Tests for scripts/vulncheck.sh, driven by recorded govulncheck JSON streams.
set -euo pipefail

SCRIPT="$(cd "$(dirname "$0")" && pwd)/vulncheck.sh"
TMPROOT="$(mktemp -d)"
trap 'rm -rf "$TMPROOT"' EXIT
fails=0
total=0

pass() { total=$((total + 1)); echo "ok $total - $1"; }
fail() { total=$((total + 1)); fails=$((fails + 1)); echo "FAIL $total - $1"; }

# Write a fixture stream from stdin, print its path.
stream() {
  local f
  f="$(mktemp "$TMPROOT/stream.XXXXXX")"
  cat >"$f"
  echo "$f"
}

f="$(stream </dev/null)"
if "$SCRIPT" --from-json "$f" >/dev/null 2>&1; then
  pass "empty stream passes"
else
  fail "empty stream passes"
fi

f="$(stream <<'EOF'
{"osv":{"id":"GO-2026-5932","summary":"openpgp is unmaintained"}}
{"finding":{"osv":"GO-2026-5932","trace":[{"module":"golang.org/x/crypto","package":"golang.org/x/crypto/openpgp/armor","function":"Decode"}]}}
EOF
)"
if out="$("$SCRIPT" --from-json "$f" 2>&1)" && [[ "$out" == *GO-2026-5932* && "$out" == *ignored* ]]; then
  pass "symbol-level finding on an ignored advisory passes and is reported as ignored"
else
  fail "symbol-level finding on an ignored advisory passes and is reported as ignored"
fi

f="$(stream <<'EOF'
{"osv":{"id":"GO-9999-0001","summary":"something real"}}
{"finding":{"osv":"GO-9999-0001","trace":[{"module":"example.com/m","package":"example.com/m/p","function":"F"}]}}
EOF
)"
if out="$("$SCRIPT" --from-json "$f" 2>&1)"; then
  fail "symbol-level finding on a non-ignored advisory fails and names it"
elif [[ "$out" == *GO-9999-0001* ]]; then
  pass "symbol-level finding on a non-ignored advisory fails and names it"
else
  fail "symbol-level finding on a non-ignored advisory fails and names it"
fi

# Import- and module-level findings (no called symbol) must not fail the gate:
# govulncheck's own default mode only fails when project code calls the symbol.
f="$(stream <<'EOF'
{"finding":{"osv":"GO-9999-0002","trace":[{"module":"example.com/m","package":"example.com/m/p"}]}}
{"finding":{"osv":"GO-9999-0003","trace":[{"module":"example.com/m"}]}}
EOF
)"
if "$SCRIPT" --from-json "$f" >/dev/null 2>&1; then
  pass "import- and module-level findings do not fail the gate"
else
  fail "import- and module-level findings do not fail the gate"
fi

f="$(stream <<'EOF'
{"finding":{"osv":"GO-2026-5932","trace":[{"module":"golang.org/x/crypto","package":"golang.org/x/crypto/openpgp/armor","function":"Decode"}]}}
{"finding":{"osv":"GO-9999-0001","trace":[{"module":"example.com/m","package":"example.com/m/p","function":"F"}]}}
EOF
)"
if out="$("$SCRIPT" --from-json "$f" 2>&1)"; then
  fail "ignored advisory does not mask a non-ignored one"
elif [[ "$out" == *GO-9999-0001* ]]; then
  pass "ignored advisory does not mask a non-ignored one"
else
  fail "ignored advisory does not mask a non-ignored one"
fi

if ! "$SCRIPT" --from-json "$TMPROOT/does-not-exist" >/dev/null 2>&1; then
  pass "missing stream file is an error"
else
  fail "missing stream file is an error"
fi

if [[ $fails -ne 0 ]]; then
  echo "$fails of $total tests failed" >&2
  exit 1
fi
echo "all $total tests passed"
