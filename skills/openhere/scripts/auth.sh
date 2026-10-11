#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${SCRIPT_DIR}/common.sh"

usage() {
  cat <<'USAGE'
Usage:
  auth.sh check [--instance URL]
  auth.sh request-code <email> [--instance URL]
  auth.sh verify-code <email> <code> [label] [--instance URL]
  auth.sh save-key [--instance URL]

Commands:
  check          Test the saved credentials against the instance. Prints the instance URL and version.
  request-code   Ask the instance to email a sign-in code to the account owner.
  verify-code    Trade the emailed 8-digit code for an API key and save it.
  save-key       Read an API key from stdin (or $OPENHERE_API_KEY), test it and save it.

Credentials are saved to ~/.openhere/credentials as JSON with mode 0600.
The instance URL comes from --instance, then $OPENHERE_URL, then the credentials file.
The key is never printed.

Options:
  --instance URL    Instance address, for example https://sites.example.com
  --allow-insecure  Allow plain http to a host that is not localhost
  -h, --help        Show this text

Exit codes:
  0 success, 1 usage error, 2 missing tool, credentials or rejected key,
  4 refused by the instance (wrong code, rate limit), 5 network failure,
  7 the instance cannot send email (use save-key with a dashboard key)
USAGE
}

parse_args() {
  COMMAND="${1:-}"
  [ -n "$COMMAND" ] || { usage >&2; exit "$EXIT_USAGE"; }
  shift
  POSITIONAL=()
  INSTANCE_FLAG=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --instance) [ $# -ge 2 ] || die "$EXIT_USAGE" "--instance needs a value"; INSTANCE_FLAG="$2"; shift 2 ;;
      --allow-insecure) ALLOW_INSECURE=1; shift ;;
      -h | --help) usage; exit 0 ;;
      -*) die "$EXIT_USAGE" "unknown option: $1" ;;
      *) POSITIONAL+=("$1"); shift ;;
    esac
  done
}

require_instance() {
  resolve_instance_url "$INSTANCE_FLAG" || die "$EXIT_AUTH" "no instance URL. Pass --instance or set OPENHERE_URL. Ask the user for their instance URL."
}

post_json() {
  local path="$1" body="$2"
  api_call POST "$path" "$body"
}

command_check() {
  load_credentials "$INSTANCE_FLAG"
  api_call GET /api/v1/instance
  fail_on_error_status "check"
  info "credentials work"
  printf 'url=%s\n' "$(jq -r '.publicUrl' "$RESPONSE_FILE")"
  printf 'version=%s\n' "$(jq -r '.version' "$RESPONSE_FILE")"
  printf 'email_available=%s\n' "$(jq -r '.emailAvailable' "$RESPONSE_FILE")"
}

command_request_code() {
  [ "${#POSITIONAL[@]}" -eq 1 ] || die "$EXIT_USAGE" "usage: auth.sh request-code <email>"
  local email="${POSITIONAL[0]}"
  require_instance
  post_json /api/v1/auth/agent/request-code "$(jq -n --arg email "$email" '{email: $email}')"
  case "$HTTP_STATUS" in
    2??) ;;
    409)
      if [ "$(response_code "$RESPONSE_FILE")" = "email_not_configured" ]; then
        info "this instance cannot send email"
        info "ask the user to create an API key in the dashboard under Settings, API keys, then run: auth.sh save-key"
        exit "$EXIT_NO_EMAIL"
      fi
      fail_on_error_status "request-code"
      ;;
    429) die "$EXIT_REFUSED" "request-code is rate limited: $(error_summary "$RESPONSE_FILE"). Wait before asking again." ;;
    *) fail_on_error_status "request-code" ;;
  esac
  info "a sign-in code was requested for ${email}"
  info "if an account exists for that address, an 8-digit code arrives by email within a minute and works for 10 minutes"
  info "ask the user to paste the code, then run: auth.sh verify-code ${email} <code>"
}

command_verify_code() {
  [ "${#POSITIONAL[@]}" -ge 2 ] && [ "${#POSITIONAL[@]}" -le 3 ] || die "$EXIT_USAGE" "usage: auth.sh verify-code <email> <code> [label]"
  local email="${POSITIONAL[0]}" code="${POSITIONAL[1]}" label="${POSITIONAL[2]:-agent}"
  require_instance
  post_json /api/v1/auth/agent/verify-code "$(jq -n --arg e "$email" --arg c "$code" --arg l "$label" '{email: $e, code: $c, label: $l}')"
  case "$HTTP_STATUS" in
    2??) ;;
    400)
      if [ "$(response_code "$RESPONSE_FILE")" = "invalid_code" ]; then
        die "$EXIT_REFUSED" "the code is wrong or expired. A code works once, for 10 minutes and 5 attempts. Run request-code again for a fresh one."
      fi
      fail_on_error_status "verify-code"
      ;;
    *) fail_on_error_status "verify-code" ;;
  esac
  local key
  key="$(jq -r '.key // empty' "$RESPONSE_FILE")"
  [ -n "$key" ] || die "$EXIT_REFUSED" "the instance returned no key"
  write_credentials "$INSTANCE_URL" "$key"
  info "signed in. API key saved to ${CREDENTIALS_FILE} (mode 0600) for ${INSTANCE_URL}"
}

command_save_key() {
  require_instance
  local key="${OPENHERE_API_KEY:-}"
  if [ -z "$key" ] && [ ! -t 0 ]; then
    IFS= read -r key || true
  fi
  key="$(printf '%s' "$key" | tr -d '[:space:]')"
  [ -n "$key" ] || die "$EXIT_USAGE" "no key given. Pipe it in: printf '%s' \"\$KEY\" | auth.sh save-key --instance URL"
  API_KEY="$key"
  write_auth_config
  api_call GET /api/v1/instance
  fail_on_error_status "save-key"
  write_credentials "$INSTANCE_URL" "$key"
  info "key accepted. Saved to ${CREDENTIALS_FILE} (mode 0600) for ${INSTANCE_URL}"
}

main() {
  parse_args "$@"
  require_core_tools
  make_work_dir
  case "$COMMAND" in
    check) command_check ;;
    request-code) command_request_code ;;
    verify-code) command_verify_code ;;
    save-key) command_save_key ;;
    -h | --help | help) usage ;;
    *) die "$EXIT_USAGE" "unknown command '$COMMAND'. Run auth.sh --help." ;;
  esac
}

main "$@"
