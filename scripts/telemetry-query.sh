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
[[ "$CLIPHUB_TELEMETRY_ADMIN_URL" == https://* ]] || {
  printf 'telemetry admin URL must use HTTPS\n' >&2
  exit 2
}
[[ "$CLIPHUB_TELEMETRY_ADMIN_TOKEN" =~ ^[A-Fa-f0-9]{64,}$ ]] || {
  printf 'telemetry admin token has an invalid format\n' >&2
  exit 2
}

usage() {
  printf 'usage: %s incident CH-XXXX-XXXX-XXXX-XXXX-XXXX [limit] | stats [hours] | health\n' "$0" >&2
  exit 2
}

request_path=''
case "${1:-}" in
  incident)
    code="${2:-}"
    limit="${3:-50}"
    [[ "$code" =~ ^CH(-[A-F0-9]{4}){5}$ ]] || usage
    [[ "$limit" =~ ^[0-9]+$ ]] && (( limit >= 1 && limit <= 200 )) || usage
    request_path="/v1/incidents?support_code=$code&limit=$limit"
    ;;
  stats)
    hours="${2:-24}"
    [[ "$hours" =~ ^[0-9]+$ ]] && (( hours >= 1 && hours <= 720 )) || usage
    request_path="/v1/stats?hours=$hours"
    ;;
  health)
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
