#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${SCRIPT_DIR}/common.sh"

PAGE_LIMIT=200

usage() {
  cat <<'USAGE'
Usage: status.sh [--workspace SLUG] [--query TEXT] [--instance URL] [--allow-insecure]

Lists the sites the saved API key can reach, one per line, as: slug url updated
A site without an address shows - as its url. The key is never printed.

Exit codes: 0 success, 1 usage error, 2 missing tool, credentials or rejected key,
4 refused by the instance, 5 network failure.
USAGE
}

WORKSPACE=""
QUERY=""
INSTANCE_FLAG=""

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --workspace) [ $# -ge 2 ] || die "$EXIT_USAGE" "--workspace needs a value"; WORKSPACE="$2"; shift 2 ;;
      --query) [ $# -ge 2 ] || die "$EXIT_USAGE" "--query needs a value"; QUERY="$2"; shift 2 ;;
      --instance) [ $# -ge 2 ] || die "$EXIT_USAGE" "--instance needs a value"; INSTANCE_FLAG="$2"; shift 2 ;;
      --allow-insecure) ALLOW_INSECURE=1; shift ;;
      -h | --help) usage; exit 0 ;;
      *) die "$EXIT_USAGE" "unknown argument: $1" ;;
    esac
  done
}

list_page() {
  local cursor="$1" query_string
  query_string="$(jq -rn --arg w "$WORKSPACE" --arg q "$QUERY" --arg c "$cursor" --argjson l "$PAGE_LIMIT" \
    '[("limit=\($l)"), (if $w != "" then "workspace=\($w|@uri)" else empty end), (if $q != "" then "q=\($q|@uri)" else empty end), (if $c != "" then "cursor=\($c|@uri)" else empty end)] | join("&")')"
  api_call GET "/api/v1/sites?${query_string}"
  fail_on_error_status "list sites"
  jq -r '.sites[] | "\(.slug) \(.url // "-") \(.updatedAt)"' "$RESPONSE_FILE"
}

main() {
  parse_args "$@"
  require_core_tools
  make_work_dir
  load_credentials "$INSTANCE_FLAG"
  local cursor=""
  while :; do
    list_page "$cursor"
    cursor="$(jq -r '.nextCursor // empty' "$RESPONSE_FILE")"
    [ -n "$cursor" ] || break
  done
}

main "$@"
