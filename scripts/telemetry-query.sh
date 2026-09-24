#!/usr/bin/env bash
set -euo pipefail

config_file="${CLIPHUB_TELEMETRY_AGENT_ENV:-$HOME/.config/cliphub/telemetry-agent.env}"
if [[ -f "$config_file" ]]; then
  # Values are generated locally, mode 0600, and contain only an HTTPS URL and hex token.
  # shellcheck disable=SC1090
  source "$config_file"
fi

: "${CLIPHUB_TELEMETRY_ADMIN_URL:?set CLIPHUB_TELEMETRY_ADMIN_URL or create $config_file}"
: "${CLIPHUB_TELEMETRY_ADMIN_TOKEN:?set CLIPHUB_TELEMETRY_ADMIN_TOKEN or create $config_file}"
# Plain HTTP is accepted only for a collector on this machine's loopback.
[[ "$CLIPHUB_TELEMETRY_ADMIN_URL" == https://* || "$CLIPHUB_TELEMETRY_ADMIN_URL" =~ ^http://(127(\.[0-9]{1,3}){3}|localhost):[0-9]+$ ]] || {
  printf 'telemetry admin URL must use HTTPS\n' >&2
  exit 2
}
[[ "$CLIPHUB_TELEMETRY_ADMIN_TOKEN" =~ ^[A-Fa-f0-9]{64,}$ ]] || {
  printf 'telemetry admin token has an invalid format\n' >&2
  exit 2
}

usage() {
  printf 'usage: %s incident CH-XXXX-XXXX-XXXX-XXXX-XXXX [limit]\n' "$0" >&2
  printf '       %s stats [hours]\n' "$0" >&2
  printf '       %s errors [--after <received_ms>:<event_id>] [--limit 1-200]\n' "$0" >&2
  printf '       %s logs (--job ID | --support CODE | --session ID | --event NAME)... [--after CURSOR] [--limit 1-500]\n' "$0" >&2
  printf '       %s health\n' "$0" >&2
  exit 2
}

uuid_pattern='^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$'
support_pattern='^CH(-[A-F0-9]{4}){5}$'

request_path=''
case "${1:-}" in
  incident)
    code="${2:-}"
    limit="${3:-50}"
    [[ "$code" =~ $support_pattern ]] || usage
    [[ "$limit" =~ ^[0-9]+$ ]] && (( limit >= 1 && limit <= 200 )) || usage
    request_path="/v1/incidents?support_code=$code&limit=$limit"
    ;;
  stats)
    hours="${2:-24}"
    [[ "$hours" =~ ^[0-9]+$ ]] && (( hours >= 1 && hours <= 720 )) || usage
    request_path="/v1/stats?hours=$hours"
    ;;
  errors)
    # Error events of every installation in collector receipt order; pass the
    # previous page's next_after as --after to continue.
    shift
    after=''
    limit=100
    while (( $# )); do
      case "$1" in
        --after)
          after="${2:-}"
          [[ "$after" =~ ^[0-9]{1,15}:[0-9A-Fa-f-]{36}$ ]] || usage
          ;;
        --limit)
          limit="${2:-}"
          [[ "$limit" =~ ^[0-9]+$ ]] && (( limit >= 1 && limit <= 200 )) || usage
          ;;
        *) usage ;;
      esac
      shift 2
    done
    request_path="/v1/errors?limit=$limit"
    if [[ -n "$after" ]]; then
      request_path+="&after=$after"
    fi
    ;;
  logs)
    # Durable trace records; filters combine. Pass next_cursor as --after.
    shift
    filters=''
    after=''
    limit=200
    while (( $# )); do
      value="${2:-}"
      case "$1" in
        --job)
          [[ "$value" =~ $uuid_pattern ]] || usage
          filters+="&job_id=$value"
          ;;
        --support)
          [[ "$value" =~ $support_pattern ]] || usage
          filters+="&support_code=$value"
          ;;
        --session)
          [[ "$value" =~ $uuid_pattern ]] || usage
          filters+="&session_id=$value"
          ;;
        --event)
          [[ "$value" =~ ^[a-zA-Z0-9][a-zA-Z0-9_.:-]{0,95}$ ]] || usage
          filters+="&event=$value"
          ;;
        --after)
          [[ "$value" =~ ^[0-9]{1,18}$ ]] || usage
          after="$value"
          ;;
        --limit)
          [[ "$value" =~ ^[0-9]+$ ]] && (( value >= 1 && value <= 500 )) || usage
          limit="$value"
          ;;
        *) usage ;;
      esac
      shift 2
    done
    [[ -n "$filters" ]] || usage
    request_path="/v1/logs?limit=$limit$filters"
    if [[ -n "$after" ]]; then
      request_path+="&after=$after"
    fi
    ;;
  health)
    # Admin health: version, database state, last receipt per channel,
    # ingest rejection counters since start and storage ratios.
    request_path='/healthz'
    ;;
  *) usage ;;
esac

# Feed the bearer header over curl's stdin config so the token never appears in
# the process argv visible to other same-host users.
response=$(
  printf '%s\n' \
    'fail' \
    'silent' \
    'show-error' \
    'max-time = 15' \
    "header = \"Authorization: Bearer $CLIPHUB_TELEMETRY_ADMIN_TOKEN\"" \
    "url = \"$CLIPHUB_TELEMETRY_ADMIN_URL$request_path\"" \
  | curl --config -
)

if command -v python3 >/dev/null 2>&1; then
  python3 -m json.tool <<<"$response"
else
  printf '%s\n' "$response"
fi
