#!/usr/bin/env bash
# Gate govulncheck with an advisory ignore list. govulncheck has no native
# suppression (golang/go#59507) and always exits 0 in -format json mode, so
# this wrapper re-implements its default gate — fail when project code calls
# a vulnerable symbol; import- and module-level findings never fail — minus
# the advisories listed in IGNORED_ADVISORIES.
#   scripts/vulncheck.sh                   scan the module (govulncheck ./...)
#   scripts/vulncheck.sh --from-json <f>   gate a recorded JSON stream (tests)
set -euo pipefail

# GO-2026-5932: x/crypto/openpgp is deprecated with no fixed release ("Fixed
# in: N/A"). Reached via gno's tm2 key armoring (tm2/pkg/crypto/keys/armor),
# which uses only the ASCII armor codec, not openpgp's crypto. Drop this
# entry once gnolang/gno migrates off openpgp/armor.
IGNORED_ADVISORIES=(GO-2026-5932)

usage() {
  grep '^#   ' "$0" >&2
  exit 2
}

case "${1-}" in
"")
  # govulncheck is intentionally @latest: it is a first-party Go-team tool and
  # the vuln database is fetched at run time regardless, so @latest keeps the
  # scanner and its advisory tooling current.
  stream="$(go run golang.org/x/vuln/cmd/govulncheck@latest -format json ./...)"
  ;;
--from-json)
  [[ -f ${2-} ]] || { echo "error: missing stream file '${2-}'" >&2; exit 1; }
  stream="$(cat "$2")"
  ;;
*)
  usage
  ;;
esac

# Advisories whose vulnerable symbols the project's code actually calls:
# findings whose deepest trace frame names a function.
called="$(jq -r 'select(.finding != null) | .finding | select(.trace[0].function != null) | .osv' <<<"$stream" | sort -u)"

status=0
while IFS= read -r id; do
  [[ -z $id ]] && continue
  summary="$(jq -r --arg id "$id" '.osv? | select(type == "object" and .id == $id) | .summary' <<<"$stream" | head -n1)"
  ignored=0
  for entry in "${IGNORED_ADVISORIES[@]}"; do
    [[ $id == "$entry" ]] && ignored=1
  done
  if [[ $ignored -eq 1 ]]; then
    echo "ignored: $id${summary:+ — $summary}"
  else
    echo "vulnerable: $id${summary:+ — $summary} (https://pkg.go.dev/vuln/$id)" >&2
    status=1
  fi
done <<<"$called"

if [[ $status -ne 0 ]]; then
  echo "error: code calls vulnerabilities not on the ignore list" >&2
  exit 1
fi
echo "ok: no actionable vulnerabilities"
