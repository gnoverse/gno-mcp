#!/usr/bin/env bash
# Tests for scripts/bump-version.sh. Each test runs in a throwaway git sandbox.
set -euo pipefail

SCRIPT="$(cd "$(dirname "$0")" && pwd)/bump-version.sh"
TMPROOT="$(mktemp -d)"
trap 'rm -rf "$TMPROOT"' EXIT
fails=0
total=0

pass() { total=$((total + 1)); echo "ok $total - $1"; }
fail() { total=$((total + 1)); fails=$((fails + 1)); echo "FAIL $total - $1"; }

# Fresh sandbox: two declared manifests (one nested array path), one audit exclude.
sandbox() {
  local dir
  dir="$(mktemp -d "$TMPROOT/sandbox.XXXXXX")"
  cd "$dir"
  git init -q
  cat >.version-bump.json <<'EOF'
{
  "files": [
    { "path": "a.json", "field": "version" },
    { "path": "nested.json", "field": "plugins.0.version" }
  ],
  "audit": { "exclude": ["excluded.txt"] }
}
EOF
  printf '{\n  "name": "a",\n  "version": "1.0.0"\n}\n' >a.json
  printf '{\n  "plugins": [\n    {\n      "version": "1.0.0"\n    }\n  ]\n}\n' >nested.json
  git add .
}

sandbox
"$SCRIPT" 2.0.0 >/dev/null
if [[ "$(jq -r .version a.json)" == "2.0.0" && "$(jq -r '.plugins[0].version' nested.json)" == "2.0.0" ]]; then
  pass "bump rewrites all declared fields, including nested array paths"
else
  fail "bump rewrites all declared fields, including nested array paths"
fi

sandbox
"$SCRIPT" v2.0.0 >/dev/null
if [[ "$(jq -r .version a.json)" == "2.0.0" ]]; then
  pass "bump strips a leading v from the version argument"
else
  fail "bump strips a leading v from the version argument"
fi

sandbox
if ! "$SCRIPT" not-a-version >/dev/null 2>&1; then
  pass "bump rejects a malformed version"
else
  fail "bump rejects a malformed version"
fi

sandbox
if "$SCRIPT" --check >/dev/null 2>&1 && "$SCRIPT" --check 1.0.0 >/dev/null 2>&1 && "$SCRIPT" --check v1.0.0 >/dev/null 2>&1; then
  pass "check passes when manifests are in sync (with and without expected version)"
else
  fail "check passes when manifests are in sync (with and without expected version)"
fi

sandbox
# Drift the SECOND declared file: the first file read becomes the reference,
# so the drift message must name the file that deviates from it.
jq '.plugins[0].version = "9.9.9"' nested.json >nested.json.tmp && mv nested.json.tmp nested.json
if out="$("$SCRIPT" --check 2>&1)"; then
  fail "check fails on drift and names the offending file"
elif [[ "$out" == *nested.json* ]]; then
  pass "check fails on drift and names the offending file"
else
  fail "check fails on drift and names the offending file"
fi

sandbox
if ! "$SCRIPT" --check 2.0.0 >/dev/null 2>&1; then
  pass "check fails when manifests differ from the expected version"
else
  fail "check fails when manifests differ from the expected version"
fi

sandbox
echo "shipped in 1.0.0" >stray.txt
git add stray.txt
if out="$("$SCRIPT" --audit 2>&1)"; then
  fail "audit fails when an undeclared tracked file contains the version"
elif [[ "$out" == *stray.txt* ]]; then
  pass "audit fails when an undeclared tracked file contains the version"
else
  fail "audit fails when an undeclared tracked file contains the version"
fi

sandbox
echo "shipped in 1.0.0" >excluded.txt
git add excluded.txt
if "$SCRIPT" --audit >/dev/null 2>&1; then
  pass "audit ignores files listed in audit.exclude"
else
  fail "audit ignores files listed in audit.exclude"
fi

sandbox
echo "see 2.0.0 notes" >stray.txt
git add stray.txt
if ! "$SCRIPT" 2.0.0 >/dev/null 2>&1; then
  pass "bump runs the audit and fails on undeclared mentions of the new version"
else
  fail "bump runs the audit and fails on undeclared mentions of the new version"
fi

if [[ $fails -ne 0 ]]; then
  echo "$fails of $total tests failed" >&2
  exit 1
fi
echo "all $total tests passed"
