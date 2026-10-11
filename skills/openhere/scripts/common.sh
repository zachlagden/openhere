#!/usr/bin/env bash

EXIT_USAGE=1
EXIT_AUTH=2
EXIT_CONFLICT=3
EXIT_REFUSED=4
EXIT_NETWORK=5
EXIT_UNSAFE=6
EXIT_NO_EMAIL=7

MAX_ATTEMPTS=5
BACKOFF_BASE_SECONDS=1
MAX_RETRY_AFTER_SECONDS=120
CONNECT_TIMEOUT_SECONDS=15
API_TIMEOUT_SECONDS=60
UPLOAD_TIMEOUT_SECONDS=600

CREDENTIALS_DIR="${HOME}/.openhere"
CREDENTIALS_FILE="${CREDENTIALS_DIR}/credentials"

INSTANCE_URL=""
API_KEY=""
ALLOW_INSECURE=0
WORK_DIR=""
AUTH_CONFIG=""
HTTP_STATUS=""
RESPONSE_FILE=""

die() {
  local code="$1"
  shift
  printf 'error: %s\n' "$*" >&2
  exit "$code"
}

warn() {
  printf 'warning: %s\n' "$*" >&2
}

info() {
  printf '%s\n' "$*" >&2
}

require_tool() {
  local tool="$1" hint="$2"
  command -v "$tool" >/dev/null 2>&1 || die "$EXIT_AUTH" "missing required tool '$tool'. $hint"
}

require_core_tools() {
  require_tool curl "Install curl with your package manager."
  require_tool jq "Install jq with your package manager (https://jqlang.org)."
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

require_sha256_tool() {
  command -v sha256sum >/dev/null 2>&1 && return 0
  command -v shasum >/dev/null 2>&1 && return 0
  die "$EXIT_AUTH" "missing required tool 'sha256sum' or 'shasum'. Install coreutils or perl's shasum."
}

make_work_dir() {
  umask 077
  WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/openhere.XXXXXX")" || die "$EXIT_USAGE" "cannot create a temporary directory"
  trap 'rm -rf "$WORK_DIR"' EXIT
}

normalize_url() {
  local url="$1"
  while [ "${url%/}" != "$url" ]; do
    url="${url%/}"
  done
  printf '%s' "$url"
}

is_local_host() {
  local url="$1" host
  host="${url#*://}"
  host="${host%%/*}"
  host="${host%%@*}"
  case "$host" in
    \[*\]*) host="${host%%\]*}]" ;;
    *) host="${host%%:*}" ;;
  esac
  case "$host" in
    localhost | 127.0.0.1 | "[::1]" | *.localhost) return 0 ;;
  esac
  return 1
}

check_instance_url() {
  local url="$1"
  case "$url" in
    https://*) return 0 ;;
    http://*)
      if is_local_host "$url" || [ "$ALLOW_INSECURE" = 1 ]; then
        return 0
      fi
      die "$EXIT_USAGE" "refusing to send credentials over plain http to $url. Use https, or pass --allow-insecure if the user accepts the risk."
      ;;
  esac
  die "$EXIT_USAGE" "the instance URL must start with https:// (got '$url')"
}

read_credentials_field() {
  local field="$1"
  [ -f "$CREDENTIALS_FILE" ] || return 0
  jq -r --arg f "$field" '.[$f] // empty' "$CREDENTIALS_FILE" 2>/dev/null || true
}

resolve_instance_url() {
  local flag_url="$1" stored_url
  stored_url="$(read_credentials_field url)"
  if [ -n "$flag_url" ]; then
    INSTANCE_URL="$flag_url"
  elif [ -n "${OPENHERE_URL:-}" ]; then
    INSTANCE_URL="$OPENHERE_URL"
  else
    INSTANCE_URL="$stored_url"
  fi
  INSTANCE_URL="$(normalize_url "$INSTANCE_URL")"
  [ -n "$INSTANCE_URL" ] || return 1
  check_instance_url "$INSTANCE_URL"
}

resolve_api_key() {
  local stored_url stored_key
  if [ -n "${OPENHERE_API_KEY:-}" ]; then
    API_KEY="$OPENHERE_API_KEY"
    return 0
  fi
  stored_url="$(normalize_url "$(read_credentials_field url)")"
  stored_key="$(read_credentials_field key)"
  if [ -n "$stored_key" ] && [ "$stored_url" = "$INSTANCE_URL" ]; then
    API_KEY="$stored_key"
    return 0
  fi
  return 1
}

load_credentials() {
  local flag_url="$1"
  if ! resolve_instance_url "$flag_url"; then
    die "$EXIT_AUTH" "no instance URL. Set OPENHERE_URL, pass --instance, or run scripts/auth.sh to create ${CREDENTIALS_FILE}. Ask the user for their instance URL if you do not have it."
  fi
  if ! resolve_api_key; then
    die "$EXIT_AUTH" "no API key for $INSTANCE_URL. Set OPENHERE_API_KEY, or sign in with scripts/auth.sh (email code) or 'scripts/auth.sh save-key' (dashboard key)."
  fi
  write_auth_config
}

write_auth_config() {
  AUTH_CONFIG="${WORK_DIR}/auth.conf"
  printf 'header = "Authorization: Bearer %s"\n' "$API_KEY" >"$AUTH_CONFIG"
}

error_summary() {
  local file="$1"
  if jq -e '.code' "$file" >/dev/null 2>&1; then
    jq -r '"\(.code): \(.message // "no message")"' "$file"
  else
    head -c 200 "$file" | tr '\n' ' '
  fi
}

response_code() {
  jq -r '.code // empty' "$1" 2>/dev/null || true
}

retry_after_from() {
  local value
  value="$(jq -r '.retry_after // empty' "$1" 2>/dev/null || true)"
  case "$value" in
    '' | *[!0-9]*) return 0 ;;
  esac
  if [ "$value" -gt "$MAX_RETRY_AFTER_SECONDS" ]; then
    value="$MAX_RETRY_AFTER_SECONDS"
  fi
  printf '%s' "$value"
}

is_transient() {
  local status="$1" file="$2"
  case "$status" in
    000 | 408 | 500 | 502 | 503 | 504) return 0 ;;
    429) [ "$(response_code "$file")" = "rate_limited" ] && return 0 ;;
  esac
  return 1
}

backoff_seconds() {
  local attempt="$1" delay="$BACKOFF_BASE_SECONDS" i=1
  while [ "$i" -lt "$attempt" ]; do
    delay=$((delay * 2))
    i=$((i + 1))
  done
  printf '%s' "$delay"
}

run_curl() {
  local out="$1"
  shift
  local status
  status="$(curl -sS -o "$out" -w '%{http_code}' --connect-timeout "$CONNECT_TIMEOUT_SECONDS" "$@" 2>"${out}.err")" || status="000"
  printf '%s' "$status"
}

request_with_retry() {
  local label="$1" out="$2"
  shift 2
  local attempt=1 status wait_for suggested
  while :; do
    status="$(run_curl "$out" "$@")"
    if ! is_transient "$status" "$out" || [ "$attempt" -ge "$MAX_ATTEMPTS" ]; then
      break
    fi
    wait_for="$(backoff_seconds "$attempt")"
    suggested="$(retry_after_from "$out")"
    if [ -n "$suggested" ] && [ "$suggested" -gt "$wait_for" ]; then
      wait_for="$suggested"
    fi
    info "${label}: attempt ${attempt} failed (HTTP ${status}), retrying in ${wait_for}s"
    sleep "$wait_for"
    attempt=$((attempt + 1))
  done
  HTTP_STATUS="$status"
}

api_call() {
  local method="$1" path="$2" body="${3:-}"
  RESPONSE_FILE="$(mktemp "${WORK_DIR}/response.XXXXXX")"
  local args=(-X "$method" --max-time "$API_TIMEOUT_SECONDS" -H "Accept: application/json")
  [ -n "$AUTH_CONFIG" ] && args+=(-K "$AUTH_CONFIG")
  if [ -n "$body" ]; then
    args+=(-H "Content-Type: application/json" --data-binary "$body")
  fi
  request_with_retry "$method $path" "$RESPONSE_FILE" "${args[@]}" "${INSTANCE_URL}${path}"
  if [ "$HTTP_STATUS" = "000" ]; then
    die "$EXIT_NETWORK" "cannot reach ${INSTANCE_URL} after ${MAX_ATTEMPTS} attempts: $(tr '\n' ' ' <"${RESPONSE_FILE}.err")"
  fi
}

fail_on_error_status() {
  local context="$1"
  case "$HTTP_STATUS" in
    2??) return 0 ;;
    401) die "$EXIT_AUTH" "${context}: the instance rejected the API key (401). The key may be revoked. Ask the user for access again." ;;
    5?? | 408) die "$EXIT_NETWORK" "${context}: the instance kept failing (HTTP ${HTTP_STATUS}): $(error_summary "$RESPONSE_FILE")" ;;
  esac
  die "$EXIT_REFUSED" "${context}: HTTP ${HTTP_STATUS} $(error_summary "$RESPONSE_FILE")"
}

write_credentials() {
  local url="$1" key="$2" tmp
  mkdir -p "$CREDENTIALS_DIR" || die "$EXIT_USAGE" "cannot create ${CREDENTIALS_DIR}"
  chmod 700 "$CREDENTIALS_DIR"
  tmp="$(umask 077 && mktemp "${CREDENTIALS_DIR}/credentials.XXXXXX")" || die "$EXIT_USAGE" "cannot write in ${CREDENTIALS_DIR}"
  if ! jq -n --arg url "$url" --arg key "$key" '{url: $url, key: $key}' >"$tmp"; then
    rm -f "$tmp"
    die "$EXIT_USAGE" "cannot write credentials"
  fi
  chmod 600 "$tmp"
  mv -f "$tmp" "$CREDENTIALS_FILE" || die "$EXIT_USAGE" "cannot replace ${CREDENTIALS_FILE}"
}
