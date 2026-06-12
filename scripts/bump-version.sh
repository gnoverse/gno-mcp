#!/usr/bin/env bash
# Bump, check, or audit the version fields declared in .version-bump.json.
# Run from the repo root:
#   scripts/bump-version.sh <version>         rewrite all declared fields, then audit
#   scripts/bump-version.sh --check [<ver>]   fail if fields drift (or differ from <ver>)
#   scripts/bump-version.sh --audit           check + flag undeclared files containing the version
set -euo pipefail

CONFIG=.version-bump.json

usage() {
  grep '^#   ' "$0" >&2
  exit 2
}

[[ -f $CONFIG ]] || { echo "error: $CONFIG not found (run from the repo root)" >&2; exit 1; }

files() { jq -r '.files[].path' "$CONFIG"; }
field_of() { jq -r --arg f "$1" '.files[] | select(.path == $f).field' "$CONFIG"; }
excludes() { jq -r '.audit.exclude[]?' "$CONFIG"; }

# dotted field path -> jq path array; numeric segments index arrays
jq_path() {
  jq -cn --arg p "$1" '$p | split(".") | map(if test("^[0-9]+$") then tonumber else . end)'
}

get_field() { jq -r --argjson p "$(jq_path "$2")" 'getpath($p) // empty' "$1"; }

set_field() {
  local tmp
  tmp="$(mktemp)"
  jq --argjson p "$(jq_path "$2")" --arg v "$3" 'setpath($p; $v)' "$1" >"$tmp"
  # Write through the existing file (not mv) so its permissions survive;
  # mktemp creates 0600 files.
  cat "$tmp" >"$1"
  rm -f "$tmp"
}

# Verify every declared field carries the same version (and matches $1 if
# given). Prints the common version on stdout; returns non-zero on any drift.
do_check() {
  local expected="${1-}" status=0 ref="" f v
  expected="${expected#v}"
  while IFS= read -r f; do
    v="$(get_field "$f" "$(field_of "$f")")"
    if [[ -z $v ]]; then
      echo "drift: $f is missing its version field" >&2
      status=1
      continue
    fi
    if [[ -z $ref ]]; then
      ref="$v"
    fi
    if [[ $v != "$ref" ]]; then
      echo "drift: $f has $v, others have $ref" >&2
      status=1
    fi
  done < <(files)
  if [[ -n $expected && $ref != "$expected" ]]; then
    echo "drift: manifests have $ref, expected $expected" >&2
    status=1
  fi
  echo "$ref"
  return "$status"
}

# Flag tracked files that contain the current version string but are neither
# declared in files[] nor covered by audit.exclude — catches a manifest that
# was added without being registered here. Matching is a literal substring
# grep: a hit inside a longer token (e.g. "10.1.0" when the version is
# "0.1.0") is a false positive; add such files to audit.exclude.
do_audit() {
  local current f d e known status=0
  if ! current="$(do_check)"; then
    return 1
  fi
  if [[ -z $current ]]; then
    echo "error: no version resolved from $CONFIG files[]" >&2
    return 1
  fi
  while IFS= read -r f; do
    known=0
    while IFS= read -r d; do
      [[ $f == "$d" ]] && known=1
    done < <(files)
    while IFS= read -r e; do
      [[ $f == "$e" || $f == "$e"/* ]] && known=1
    done < <(excludes)
    if [[ $known -eq 0 ]]; then
      echo "audit: version $current appears in undeclared file: $f" >&2
      status=1
    fi
  done < <(git grep -lF -- "$current" || true)
  if [[ $status -ne 0 ]]; then
    echo "audit: declare these in $CONFIG files[] or add them to audit.exclude" >&2
  fi
  return "$status"
}

case "${1-}" in
"")
  usage
  ;;
--check)
  do_check "${2-}" >/dev/null
  echo "ok: all manifests in sync"
  ;;
--audit)
  do_audit
  echo "ok: audit clean"
  ;;
--*)
  usage
  ;;
*)
  version="${1#v}"
  if ! [[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
    echo "error: invalid version '$1' (want X.Y.Z or X.Y.Z-pre)" >&2
    exit 1
  fi
  while IFS= read -r f; do
    set_field "$f" "$(field_of "$f")" "$version"
    echo "bumped: $f -> $version"
  done < <(files)
  do_audit
  ;;
esac
