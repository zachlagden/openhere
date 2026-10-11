#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${SCRIPT_DIR}/common.sh"

DEFAULT_JOBS=4
POLL_SECONDS=0.1
MAX_FINALIZE_ROUNDS=3
SECRET_CONTENT_PATTERN='-----BEGIN [A-Z ]*PRIVATE KEY-----|AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{36}|xox[baprs]-[A-Za-z0-9-]{10,}|AIza[0-9A-Za-z_-]{35}|sk_(live|test)_[0-9A-Za-z]{20,}|sk_[A-Za-z0-9]{32,}'

usage() {
  cat <<'USAGE'
Usage: publish.sh <directory> [options]

Publishes a folder of built static files to an OpenHere instance and prints
the site address. A second run from the same project updates the same site.

Options:
  --slug SLUG         Site name. Updates the site when it exists, creates it otherwise.
  --name NAME         Display name of the site (up to 100 characters).
  --spa               Serve index.html for unknown paths without a file extension.
  --folder NAME       Dashboard folder to file a new site in (name or id).
  --workspace SLUG    Workspace for a new site (default: the personal workspace).
  --instance URL      Instance address. Overrides $OPENHERE_URL and the credentials file.
  --jobs N            Parallel uploads (default 4).
  --force             Publish without a base version. Replaces whatever is live.
  --state FILE        State file (default ./.openhere/state.json).
  --include-maps      Publish *.map source map files, which are skipped by default.
  --allow-secrets     Publish even when files look like secrets. Only after the user confirmed.
  --allow-insecure    Allow plain http to a host that is not localhost.
  --dry-run           List what would be published and stop. Needs no credentials.
  -h, --help          Show this text.

Credentials: $OPENHERE_URL and $OPENHERE_API_KEY, else ~/.openhere/credentials.
The key goes only into the Authorization header of requests to the instance.

Skipped without a message per file: .git, node_modules, hidden files and
folders (except .well-known), lock files, symlinks, source maps, the state file.

Output on stdout, one line each:
  url=<site address>
  slug=<site slug>
  version=<live version id>
A summary goes to stderr.

Exit codes:
  0 published
  1 usage error
  2 missing tool, missing credentials or rejected key
  3 version conflict: the live version changed since the last publish from here.
    The live version id is printed. Rerun with --force to replace it.
  4 refused by the instance (validation, limits, permissions, slug taken)
  5 network failure or the instance kept failing after retries
  6 nothing to publish, or files look like secrets
USAGE
}

DIRECTORY=""
SLUG_FLAG=""
NAME=""
SPA=0
FOLDER=""
WORKSPACE=""
INSTANCE_FLAG=""
JOBS="$DEFAULT_JOBS"
FORCE=0
STATE_FILE=""
INCLUDE_MAPS=0
ALLOW_SECRETS=0
DRY_RUN=0

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --slug) need_value "$@"; SLUG_FLAG="$2"; shift 2 ;;
      --name) need_value "$@"; NAME="$2"; shift 2 ;;
      --spa) SPA=1; shift ;;
      --folder) need_value "$@"; FOLDER="$2"; shift 2 ;;
      --workspace) need_value "$@"; WORKSPACE="$2"; shift 2 ;;
      --instance) need_value "$@"; INSTANCE_FLAG="$2"; shift 2 ;;
      --jobs) need_value "$@"; JOBS="$2"; shift 2 ;;
      --force) FORCE=1; shift ;;
      --state) need_value "$@"; STATE_FILE="$2"; shift 2 ;;
      --include-maps) INCLUDE_MAPS=1; shift ;;
      --allow-secrets) ALLOW_SECRETS=1; shift ;;
      --allow-insecure) ALLOW_INSECURE=1; shift ;;
      --dry-run) DRY_RUN=1; shift ;;
      -h | --help) usage; exit 0 ;;
      --) shift; break ;;
      -*) die "$EXIT_USAGE" "unknown option: $1" ;;
      *)
        [ -z "$DIRECTORY" ] || die "$EXIT_USAGE" "unexpected argument: $1"
        DIRECTORY="$1"
        shift
        ;;
    esac
  done
  [ -n "$DIRECTORY" ] || { usage >&2; exit "$EXIT_USAGE"; }
  [ -d "$DIRECTORY" ] || die "$EXIT_USAGE" "not a directory: $DIRECTORY"
  case "$JOBS" in
    '' | *[!0-9]* | 0) die "$EXIT_USAGE" "--jobs needs a positive number" ;;
  esac
}

need_value() {
  [ $# -ge 2 ] || die "$EXIT_USAGE" "$1 needs a value"
}

has_hidden_segment() {
  local rest="$1" segment
  while [ -n "$rest" ]; do
    segment="${rest%%/*}"
    case "$rest" in
      */*) rest="${rest#*/}" ;;
      *) rest="" ;;
    esac
    case "$segment" in
      .well-known) ;;
      .*) return 0 ;;
    esac
  done
  return 1
}

is_lock_file() {
  case "$1" in
    package-lock.json | npm-shrinkwrap.json | pnpm-lock.yaml | yarn.lock | bun.lockb | *.lock) return 0 ;;
  esac
  return 1
}

is_source_map() {
  case "$1" in
    *.js.map | *.mjs.map | *.css.map) return 0 ;;
  esac
  return 1
}

is_secret_file_name() {
  case "$1" in
    *.pem | *.key | *.p12 | *.pfx | *.keystore | id_rsa | id_ed25519 | id_ecdsa | credentials | credentials.json) return 0 ;;
  esac
  return 1
}

content_type_for() {
  local file="$1" name extension detected
  name="${file##*/}"
  extension="$(printf '%s' "${name##*.}" | tr '[:upper:]' '[:lower:]')"
  [ "$extension" = "$name" ] && extension=""
  case "$extension" in
    html | htm) echo "text/html; charset=utf-8" ;;
    css) echo "text/css; charset=utf-8" ;;
    js | mjs) echo "text/javascript; charset=utf-8" ;;
    json | map) echo "application/json; charset=utf-8" ;;
    webmanifest) echo "application/manifest+json" ;;
    txt | md) echo "text/plain; charset=utf-8" ;;
    csv) echo "text/csv; charset=utf-8" ;;
    xml) echo "application/xml" ;;
    svg) echo "image/svg+xml" ;;
    png) echo "image/png" ;;
    jpg | jpeg) echo "image/jpeg" ;;
    gif) echo "image/gif" ;;
    webp) echo "image/webp" ;;
    avif) echo "image/avif" ;;
    ico) echo "image/x-icon" ;;
    pdf) echo "application/pdf" ;;
    woff) echo "font/woff" ;;
    woff2) echo "font/woff2" ;;
    ttf) echo "font/ttf" ;;
    otf) echo "font/otf" ;;
    mp4) echo "video/mp4" ;;
    webm) echo "video/webm" ;;
    mp3) echo "audio/mpeg" ;;
    wav) echo "audio/wav" ;;
    ogg) echo "audio/ogg" ;;
    wasm) echo "application/wasm" ;;
    *)
      detected=""
      if command -v file >/dev/null 2>&1; then
        detected="$(file --brief --mime-type -- "$file" 2>/dev/null || true)"
      fi
      echo "${detected:-application/octet-stream}"
      ;;
  esac
}

SKIPPED_HIDDEN=0
SKIPPED_LOCKS=0
SKIPPED_MAPS=0
SKIPPED_SYMLINKS=0
FILE_COUNT=0
TOTAL_BYTES=0

collect_files() {
  local relative absolute name size digest type
  : >"${WORK_DIR}/files.tsv"
  : >"${WORK_DIR}/absolute.nul"
  : >"${WORK_DIR}/secret-names.txt"
  while IFS= read -r -d '' relative; do
    relative="${relative#./}"
    name="${relative##*/}"
    absolute="${DIRECTORY_ABS}/${relative}"
    if [ -L "$absolute" ]; then
      SKIPPED_SYMLINKS=$((SKIPPED_SYMLINKS + 1))
      continue
    fi
    if has_hidden_segment "$relative" || [ "$absolute" = "$STATE_FILE_ABS" ]; then
      SKIPPED_HIDDEN=$((SKIPPED_HIDDEN + 1))
      continue
    fi
    if is_lock_file "$name"; then
      SKIPPED_LOCKS=$((SKIPPED_LOCKS + 1))
      continue
    fi
    if [ "$INCLUDE_MAPS" = 0 ] && is_source_map "$name"; then
      SKIPPED_MAPS=$((SKIPPED_MAPS + 1))
      continue
    fi
    case "$relative" in
      *$'\t'* | *$'\n'*) die "$EXIT_REFUSED" "a file name contains a tab or newline and the instance rejects it: $(printf '%q' "$relative")" ;;
    esac
    if is_secret_file_name "$name"; then
      printf '%s\n' "$relative" >>"${WORK_DIR}/secret-names.txt"
    fi
    size="$(wc -c <"$absolute" | tr -d ' ')"
    digest="$(sha256_of "$absolute")"
    type="$(content_type_for "$absolute")"
    printf '%s\t%s\t%s\t%s\n' "$relative" "$size" "$digest" "$type" >>"${WORK_DIR}/files.tsv"
    printf '%s\0' "$absolute" >>"${WORK_DIR}/absolute.nul"
    FILE_COUNT=$((FILE_COUNT + 1))
    TOTAL_BYTES=$((TOTAL_BYTES + size))
  done < <(cd "$DIRECTORY_ABS" && find . -mindepth 1 \( -name .git -o -name node_modules \) -prune -o \( -type f -o -type l \) -print0 | LC_ALL=C sort -z)
}

refuse_secrets() {
  local content_hits
  [ "$ALLOW_SECRETS" = 1 ] && return 0
  content_hits="${WORK_DIR}/secret-content.txt"
  : >"$content_hits"
  if [ -s "${WORK_DIR}/absolute.nul" ]; then
    xargs -0 grep -IlE -e "$SECRET_CONTENT_PATTERN" -- <"${WORK_DIR}/absolute.nul" >"$content_hits" 2>/dev/null || true
  fi
  if [ ! -s "$content_hits" ] && [ ! -s "${WORK_DIR}/secret-names.txt" ]; then
    return 0
  fi
  info "refusing to publish: some files look like secrets"
  sed 's/^/  name: /' "${WORK_DIR}/secret-names.txt" >&2
  sed "s|^${DIRECTORY_ABS}/|  content: |" "$content_hits" >&2
  info "published files are served to anyone with the link. Remove them from the folder, or ask the user and rerun with --allow-secrets"
  exit "$EXIT_UNSAFE"
}

print_skipped() {
  local parts=()
  [ "$SKIPPED_HIDDEN" -gt 0 ] && parts+=("${SKIPPED_HIDDEN} hidden or state")
  [ "$SKIPPED_LOCKS" -gt 0 ] && parts+=("${SKIPPED_LOCKS} lock")
  [ "$SKIPPED_MAPS" -gt 0 ] && parts+=("${SKIPPED_MAPS} source map")
  [ "$SKIPPED_SYMLINKS" -gt 0 ] && parts+=("${SKIPPED_SYMLINKS} symlink")
  [ "${#parts[@]}" -gt 0 ] || return 0
  local joined
  joined="$(IFS=,; printf '%s' "${parts[*]}")"
  info "skipped files: ${joined//,/, }"
}

build_manifest() {
  jq -Rn '[inputs | split("\t") | {path: .[0], size: (.[1] | tonumber), sha256: .[2], contentType: .[3]}]' \
    <"${WORK_DIR}/files.tsv" >"${WORK_DIR}/manifest.json"
}

resolve_state_location() {
  CWD_ABS="$(pwd -P)"
  if [ -z "$STATE_FILE" ]; then
    STATE_FILE="${CWD_ABS}/.openhere/state.json"
  fi
  case "$STATE_FILE" in
    /*) ;;
    *) STATE_FILE="${CWD_ABS}/${STATE_FILE}" ;;
  esac
  STATE_FILE_ABS="$STATE_FILE"
  if [ "$DIRECTORY_ABS" = "$CWD_ABS" ]; then
    DIRECTORY_KEY="."
  else
    case "$DIRECTORY_ABS" in
      "$CWD_ABS"/*) DIRECTORY_KEY="${DIRECTORY_ABS#"$CWD_ABS"/}" ;;
      *) DIRECTORY_KEY="$DIRECTORY_ABS" ;;
    esac
  fi
}

state_slug_for_directory() {
  [ -f "$STATE_FILE" ] || return 0
  jq -r --arg dir "$DIRECTORY_KEY" --arg url "$INSTANCE_URL" \
    '[.sites // {} | to_entries[] | select(.value.directory == $dir and .value.instanceUrl == $url)] | first | .key // empty' \
    "$STATE_FILE" 2>/dev/null || true
}

state_version_for_slug() {
  [ -f "$STATE_FILE" ] || return 0
  jq -r --arg slug "$1" --arg url "$INSTANCE_URL" \
    '.sites[$slug] // empty | select(.instanceUrl == $url) | .versionId // empty' "$STATE_FILE" 2>/dev/null || true
}

save_state() {
  local slug="$1" version="$2" url="$3" current tmp
  mkdir -p "$(dirname "$STATE_FILE")"
  current='{"sites":{}}'
  if [ -f "$STATE_FILE" ] && jq -e . "$STATE_FILE" >/dev/null 2>&1; then
    current="$(cat "$STATE_FILE")"
  fi
  tmp="$(mktemp "$(dirname "$STATE_FILE")/state.XXXXXX")"
  printf '%s' "$current" | jq --arg slug "$slug" --arg inst "$INSTANCE_URL" --arg dir "$DIRECTORY_KEY" --arg ver "$version" --arg url "$url" \
    '.sites = (.sites // {}) | .sites[$slug] = {instanceUrl: $inst, directory: $dir, versionId: $ver, url: $url}' >"$tmp"
  mv -f "$tmp" "$STATE_FILE"
  ignore_state_directory
}

ignore_state_directory() {
  local state_dir gitignore
  state_dir="$(dirname "$STATE_FILE")"
  [ "$state_dir" = "${CWD_ABS}/.openhere" ] || return 0
  gitignore="${CWD_ABS}/.gitignore"
  [ -f "$gitignore" ] || return 0
  if ! grep -qxE '/?\.openhere/?' "$gitignore"; then
    [ -s "$gitignore" ] && [ -n "$(tail -c 1 "$gitignore")" ] && printf '\n' >>"$gitignore"
    printf '.openhere/\n' >>"$gitignore"
    info "added .openhere/ to .gitignore"
  fi
}

report_conflict() {
  local live
  live="$(jq -r '.currentVersionId // empty' "$RESPONSE_FILE")"
  info "version conflict: the live version of this site changed since the last publish from here"
  [ -n "$live" ] && info "live version: ${live}"
  info "ask the user whether to replace it. To replace it, rerun with --force"
  [ -n "$live" ] && printf 'live_version=%s\n' "$live"
  exit "$EXIT_CONFLICT"
}

check_publish_status() {
  local context="$1" code
  case "$HTTP_STATUS" in
    2??) return 0 ;;
    409)
      code="$(response_code "$RESPONSE_FILE")"
      [ "$code" = "version_conflict" ] && report_conflict
      ;;
    429)
      code="$(response_code "$RESPONSE_FILE")"
      if [ "$code" = "too_many_pending" ]; then
        die "$EXIT_REFUSED" "${context}: too many unfinished versions on this site or instance. Delete pending versions or wait up to 24 hours for them to expire."
      fi
      ;;
  esac
  fail_on_error_status "$context"
}

SITE_EXISTS=0
TARGET_SLUG=""
BASE_VERSION=""

resolve_target() {
  local live_version
  TARGET_SLUG="$SLUG_FLAG"
  if [ -z "$TARGET_SLUG" ]; then
    TARGET_SLUG="$(state_slug_for_directory)"
  fi
  [ -n "$TARGET_SLUG" ] || return 0
  api_call GET "/api/v1/sites/${TARGET_SLUG}"
  case "$HTTP_STATUS" in
    2??) SITE_EXISTS=1 ;;
    404) SITE_EXISTS=0 ;;
    *) fail_on_error_status "look up site ${TARGET_SLUG}" ;;
  esac
  [ "$SITE_EXISTS" = 1 ] || return 0
  live_version="$(jq -r '.site.currentVersionId // empty' "$RESPONSE_FILE")"
  if [ "$FORCE" = 0 ]; then
    BASE_VERSION="$(state_version_for_slug "$TARGET_SLUG")"
    [ -n "$BASE_VERSION" ] || BASE_VERSION="$live_version"
  fi
}

explain_missing_folder() {
  [ "$HTTP_STATUS" = "404" ] && [ "$SITE_EXISTS" = 0 ] || return 0
  [ -n "$FOLDER" ] || [ -n "$WORKSPACE" ] || return 0
  die "$EXIT_REFUSED" "the workspace or folder was not found ($(error_summary "$RESPONSE_FILE")). Folders must exist before publishing: create it in the dashboard, then rerun."
}

create_pending_version() {
  local request="${WORK_DIR}/request.json"
  if [ "$SITE_EXISTS" = 1 ]; then
    jq -n --slurpfile files "${WORK_DIR}/manifest.json" --arg base "$BASE_VERSION" \
      '{files: $files[0]} + (if $base != "" then {baseVersionId: $base} else {} end)' >"$request"
    api_call PUT "/api/v1/publish/${TARGET_SLUG}" "@${request}"
  else
    jq -n --slurpfile files "${WORK_DIR}/manifest.json" --arg slug "$TARGET_SLUG" --arg name "$NAME" \
      --arg folder "$FOLDER" --arg workspace "$WORKSPACE" --argjson spa "$([ "$SPA" = 1 ] && echo true || echo false)" \
      '{files: $files[0], spaMode: $spa}
       + (if $slug != "" then {slug: $slug} else {} end)
       + (if $name != "" then {displayName: $name} else {} end)
       + (if $folder != "" then {folder: $folder} else {} end)
       + (if $workspace != "" then {workspace: $workspace} else {} end)' >"$request"
    api_call POST /api/v1/publish "@${request}"
  fi
  explain_missing_folder
  check_publish_status "create version"
  SLUG="$(jq -r '.slug' "$RESPONSE_FILE")"
  FINALIZE_PATH="$(jq -r '.finalizeUrl' "$RESPONSE_FILE")"
  jq -r '.uploads[] | [.path, .url] + (.headers | to_entries | map("\(.key): \(.value)")) | join("\t")' "$RESPONSE_FILE" >"${WORK_DIR}/uploads.tsv"
  case "$FINALIZE_PATH" in
    /*) ;;
    *) die "$EXIT_REFUSED" "the instance returned an unexpected finalize address" ;;
  esac
}

upload_one() {
  local index="$1" path="$2" url_path="$3"
  shift 3
  local absolute="${DIRECTORY_ABS}/${path}" out="${WORK_DIR}/upload.${index}" args=() header
  for header in "$@"; do
    args+=(-H "$header")
  done
  request_with_retry "upload ${path}" "$out" -X PUT --max-time "$UPLOAD_TIMEOUT_SECONDS" -K "$AUTH_CONFIG" \
    -H "Expect:" ${args[@]+"${args[@]}"} -T "$absolute" "${INSTANCE_URL}${url_path}"
  case "$HTTP_STATUS" in
    2??) ;;
    000) printf '%s\t%s\t%s\n' "$path" "$HTTP_STATUS" "$(tr '\n' ' ' <"${out}.err")" >"${WORK_DIR}/failed.${index}" ;;
    *) printf '%s\t%s\t%s\n' "$path" "$HTTP_STATUS" "$(error_summary "$out")" >"${WORK_DIR}/failed.${index}" ;;
  esac
}

wait_for_slot() {
  while [ "$(jobs -rp | wc -l | tr -d ' ')" -ge "$JOBS" ]; do
    sleep "$POLL_SECONDS"
  done
}

run_uploads() {
  local only_paths="$1" index=0 fields path url_path
  rm -f "${WORK_DIR}"/failed.*
  while IFS= read -r line; do
    IFS=$'\t' read -r -a fields <<<"$line"
    path="${fields[0]}"
    url_path="${fields[1]}"
    index=$((index + 1))
    if [ -n "$only_paths" ] && ! grep -qxF -- "$path" "$only_paths"; then
      continue
    fi
    case "$url_path" in
      /*) ;;
      *) die "$EXIT_REFUSED" "the instance returned an upload address outside itself for ${path}" ;;
    esac
    wait_for_slot
    upload_one "$index" "$path" "$url_path" "${fields[@]:2}" &
  done <"${WORK_DIR}/uploads.tsv"
  wait
  report_upload_failures
}

report_upload_failures() {
  local failures=("${WORK_DIR}"/failed.*) status worst=0 line count=0
  [ -e "${failures[0]}" ] || return 0
  for line in "${failures[@]}"; do
    status="$(cut -f2 "$line")"
    count=$((count + 1))
    [ "$count" -le 10 ] && info "upload failed: $(cut -f1 "$line") (HTTP $(cut -f2 "$line")) $(cut -f3 "$line")"
    case "$status" in
      401) worst="$EXIT_AUTH" ;;
      000 | 5?? | 408) [ "$worst" = 0 ] && worst="$EXIT_NETWORK" ;;
      *) [ "$worst" != "$EXIT_AUTH" ] && worst="$EXIT_REFUSED" ;;
    esac
  done
  info "${count} file(s) failed to upload. The unfinished version expires after 24 hours."
  exit "$worst"
}

finalize_version() {
  local round=1 missing_file="${WORK_DIR}/missing.txt"
  while :; do
    api_call POST "$FINALIZE_PATH"
    if [ "$HTTP_STATUS" = "409" ] && [ "$(response_code "$RESPONSE_FILE")" = "missing_files" ] && [ "$round" -lt "$MAX_FINALIZE_ROUNDS" ]; then
      jq -r '.missing[]?' "$RESPONSE_FILE" >"$missing_file"
      info "finalize reported $(wc -l <"$missing_file" | tr -d ' ') missing file(s), uploading them again"
      run_uploads "$missing_file"
      round=$((round + 1))
      continue
    fi
    break
  done
  check_publish_status "finalize"
  SITE_URL="$(jq -r '.url // empty' "$RESPONSE_FILE")"
  LIVE_VERSION="$(jq -r '.version.id // empty' "$RESPONSE_FILE")"
  jq -r '.hosting.notices[]? | "notice: " + (. | if type == "string" then . else (.message // tostring) end)' "$RESPONSE_FILE" >&2 || true
}

patch_site_settings() {
  local body
  [ "$SITE_EXISTS" = 1 ] || return 0
  body="$(jq -n --arg name "$NAME" --argjson spa "$([ "$SPA" = 1 ] && echo true || echo false)" \
    '(if $name != "" then {displayName: $name} else {} end) + (if $spa then {spaMode: true} else {} end)')"
  [ "$body" = "{}" ] && return 0
  api_call PATCH "/api/v1/sites/${SLUG}" "$body"
  case "$HTTP_STATUS" in
    2??) ;;
    *) warn "the version is live, but updating the site settings failed: $(error_summary "$RESPONSE_FILE")" ;;
  esac
}

warn_ignored_create_options() {
  [ "$SITE_EXISTS" = 1 ] || return 0
  [ -z "$FOLDER" ] || warn "--folder applies only when a site is created. Move the site in the dashboard."
  [ -z "$WORKSPACE" ] || warn "--workspace applies only when a site is created."
}

print_dry_run() {
  info "dry run: ${FILE_COUNT} file(s), ${TOTAL_BYTES} bytes from ${DIRECTORY_ABS}"
  cut -f1,2,4 "${WORK_DIR}/files.tsv" | awk -F '\t' '{printf "  %s (%s bytes, %s)\n", $1, $2, $3}' >&2
  print_skipped
}

summary() {
  local mode="created"
  [ "$SITE_EXISTS" = 1 ] && mode="updated"
  info "${mode} site ${SLUG}: ${FILE_COUNT} file(s), ${TOTAL_BYTES} bytes"
  print_skipped
  if [ -z "$SITE_URL" ]; then
    warn "the instance has no address for this site yet. Check hosting in the dashboard."
  fi
}

main() {
  parse_args "$@"
  require_core_tools
  require_sha256_tool
  make_work_dir
  DIRECTORY_ABS="$(cd "$DIRECTORY" && pwd -P)"
  resolve_state_location
  collect_files
  [ "$FILE_COUNT" -gt 0 ] || die "$EXIT_UNSAFE" "nothing to publish in ${DIRECTORY}. Build the site first and publish the output folder."
  refuse_secrets
  [ -f "${DIRECTORY_ABS}/index.html" ] || [ "$SPA" = 1 ] || warn "no index.html at the root of ${DIRECTORY}. The site address will answer 404 unless the files are reached by path."
  if [ "$DRY_RUN" = 1 ]; then
    print_dry_run
    exit 0
  fi
  load_credentials "$INSTANCE_FLAG"
  build_manifest
  resolve_target
  warn_ignored_create_options
  create_pending_version
  info "uploading ${FILE_COUNT} file(s) with ${JOBS} parallel job(s)"
  run_uploads ""
  finalize_version
  patch_site_settings
  save_state "$SLUG" "$LIVE_VERSION" "$SITE_URL"
  summary
  printf 'url=%s\n' "$SITE_URL"
  printf 'slug=%s\n' "$SLUG"
  printf 'version=%s\n' "$LIVE_VERSION"
}

main "$@"
