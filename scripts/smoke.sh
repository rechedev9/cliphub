#!/usr/bin/env bash
set -euo pipefail

DEMO="${1:-testdata/lavked-vs-tnc-m2-nuke.dem}"
TARGET="${2:-76561198148986856}"
BASE="${ZV_BASE_URL:-}"
CAPABILITY=""
OWNED_PID=""
OWNED_PID_STARTED_TICKS=""
OWNED_LAUNCH_NOT_BEFORE_TICKS=""
OWNED_SERVER_PID=""
OWNED_SERVER_STARTED_TICKS=""
OWNED_PROCESS_GROUP=""
OWNED_DATA=""

listener_pid() {
  local port="$1"
  local pid=""
  if command -v taskkill.exe >/dev/null 2>&1; then
    pid="$(
      MSYS2_ARG_CONV_EXCL='*' netstat.exe -ano -p tcp 2>/dev/null |
        awk -v suffix=":$port" \
          '$1 == "TCP" && $2 ~ (suffix "$") && $4 == "LISTENING" && $5 ~ /^[0-9]+$/ { print $5; exit }' ||
        true
    )"
    printf '%s\n' "$pid"
    return 0
  fi
  if command -v lsof >/dev/null 2>&1; then
    pid="$(lsof -nP -t -iTCP:"$port" -sTCP:LISTEN 2>/dev/null | head -1 || true)"
    printf '%s\n' "$pid"
    return 0
  fi
  if command -v ss >/dev/null 2>&1; then
    pid="$(
      ss -H -ltnp "sport = :$port" 2>/dev/null |
        sed -n 's/.*pid=\([0-9][0-9]*\).*/\1/p' |
        head -1 ||
        true
    )"
    printf '%s\n' "$pid"
    return 0
  fi
  return 1
}

windows_process_started_ticks() {
  local candidate="$1"
  local ticks=""

  [[ "$candidate" =~ ^[0-9]+$ ]] || return 1
  command -v powershell.exe >/dev/null 2>&1 || return 1
  ticks="$(
    MSYS2_ARG_CONV_EXCL='*' \
      FF_PROCESS_PID="$candidate" \
      powershell.exe -NoProfile -NonInteractive -Command '
        try {
          $process = Get-Process -Id ([int]$env:FF_PROCESS_PID) -ErrorAction Stop
          [void]$process.Handle
          [Console]::Out.WriteLine(
            $process.StartTime.ToUniversalTime().Ticks.ToString(
              [Globalization.CultureInfo]::InvariantCulture
            )
          )
        } catch {
          exit 1
        }
      ' 2>/dev/null |
      tr -d '[:space:]'
  )" || return 1
  [[ "$ticks" =~ ^[0-9]+$ ]] || return 1
  printf '%s\n' "$ticks"
}

windows_utc_now_ticks() {
  local ticks=""

  command -v powershell.exe >/dev/null 2>&1 || return 1
  ticks="$(
    powershell.exe -NoProfile -NonInteractive -Command '
      [Console]::Out.WriteLine(
        [DateTime]::UtcNow.Ticks.ToString(
          [Globalization.CultureInfo]::InvariantCulture
        )
      )
    ' 2>/dev/null |
      tr -d '[:space:]'
  )" || return 1
  [[ "$ticks" =~ ^[0-9]+$ ]] || return 1
  printf '%s\n' "$ticks"
}

windows_process_matches_identity() {
  local candidate="$1"
  local expected_ticks="$2"
  local actual_ticks=""

  [[ "$expected_ticks" =~ ^[0-9]+$ ]] || return 1
  actual_ticks="$(windows_process_started_ticks "$candidate")" || return 1
  [ "$actual_ticks" = "$expected_ticks" ]
}

windows_taskkill_if_same() {
  local candidate="$1"
  local expected_ticks="$2"

  [[ "$candidate" =~ ^[0-9]+$ ]] || return 1
  [[ "$expected_ticks" =~ ^[0-9]+$ ]] || return 1
  command -v powershell.exe >/dev/null 2>&1 || return 1
  MSYS2_ARG_CONV_EXCL='*' \
    FF_PROCESS_PID="$candidate" \
    FF_PROCESS_STARTED_TICKS="$expected_ticks" \
    powershell.exe -NoProfile -NonInteractive -Command '
      try {
        $process = Get-Process -Id ([int]$env:FF_PROCESS_PID) -ErrorAction Stop
        [void]$process.Handle
        $actualTicks = $process.StartTime.ToUniversalTime().Ticks.ToString(
          [Globalization.CultureInfo]::InvariantCulture
        )
        if ($actualTicks -ne $env:FF_PROCESS_STARTED_TICKS) {
          exit 3
        }
        $taskkill = Start-Process `
          -FilePath taskkill.exe `
          -ArgumentList @("/PID", [string]$process.Id, "/T", "/F") `
          -Wait `
          -PassThru `
          -WindowStyle Hidden
        exit $taskkill.ExitCode
      } catch {
        exit 3
      }
    ' >/dev/null 2>&1
}

process_belongs_to_launch() {
  local candidate="$1"
  local root="$2"
  local root_started_ticks="${3:-}"
  local launch_not_before_ticks="${4:-}"
  local parent=""
  local listener_group=""
  local depth=0

  [[ "$candidate" =~ ^[0-9]+$ ]] || return 1
  [[ "$root" =~ ^[0-9]+$ ]] || return 1

  if command -v taskkill.exe >/dev/null 2>&1; then
    command -v powershell.exe >/dev/null 2>&1 || return 1
    [[ "$launch_not_before_ticks" =~ ^[0-9]+$ ]] || return 1
    MSYS2_ARG_CONV_EXCL='*' \
      FF_LISTENER_PID="$candidate" \
      FF_LAUNCH_PID="$root" \
      FF_LAUNCH_STARTED_TICKS="$root_started_ticks" \
      FF_LAUNCH_NOT_BEFORE_TICKS="$launch_not_before_ticks" \
      powershell.exe -NoProfile -NonInteractive -Command '
        $candidate = [int]$env:FF_LISTENER_PID
        $root = [int]$env:FF_LAUNCH_PID
        $hasRootIdentity = $env:FF_LAUNCH_STARTED_TICKS -match "^[0-9]+$"
        $rootStartedTicks = if ($hasRootIdentity) {
          [long]$env:FF_LAUNCH_STARTED_TICKS
        } else {
          [long]0
        }
        $launchNotBeforeTicks = [long]$env:FF_LAUNCH_NOT_BEFORE_TICKS
        $descendantStartedTicks = [long]0
        for ($depth = 0; $depth -lt 128 -and $candidate -gt 0; $depth++) {
          if ($candidate -eq $root) {
            if ($descendantStartedTicks -eq 0) {
              if (-not $hasRootIdentity) {
                exit 1
              }
              try {
                $rootProcess = Get-CimInstance Win32_Process -Filter ("ProcessId = {0}" -f $root) -ErrorAction Stop
              } catch {
                exit 1
              }
              if ($null -eq $rootProcess) {
                exit 1
              }
              $currentRootTicks = $rootProcess.CreationDate.ToUniversalTime().Ticks
              if ($currentRootTicks -eq $rootStartedTicks) {
                exit 0
              }
              exit 1
            }
            if ($descendantStartedTicks -lt $launchNotBeforeTicks -or
                ($hasRootIdentity -and $descendantStartedTicks -lt $rootStartedTicks)) {
              exit 1
            }
            try {
              $rootProcess = Get-CimInstance Win32_Process -Filter ("ProcessId = {0}" -f $root) -ErrorAction Stop
            } catch {
              exit 1
            }
            if ($null -eq $rootProcess) {
              exit 0
            }
            $currentRootTicks = $rootProcess.CreationDate.ToUniversalTime().Ticks
            if (($hasRootIdentity -and $currentRootTicks -eq $rootStartedTicks) -or
                $currentRootTicks -gt $descendantStartedTicks) {
              # A newer process may reuse the exited wrapper PID. A listener
              # created before that reuse still descends from the saved root.
              exit 0
            }
            exit 1
          }
          try {
            $process = Get-CimInstance Win32_Process -Filter ("ProcessId = {0}" -f $candidate) -ErrorAction Stop
          } catch {
            exit 1
          }
          if ($null -eq $process) {
            exit 1
          }
          $currentStartedTicks = $process.CreationDate.ToUniversalTime().Ticks
          if ($descendantStartedTicks -ne 0 -and
              $currentStartedTicks -gt $descendantStartedTicks) {
            exit 1
          }
          $descendantStartedTicks = $currentStartedTicks
          $candidate = [int]$process.ParentProcessId
        }
        exit 1
      ' >/dev/null 2>&1
    return
  fi

  [ "$candidate" = "$root" ] && return 0
  command -v ps >/dev/null 2>&1 || return 1
  if [ "$OWNED_PROCESS_GROUP" = "1" ]; then
    listener_group="$(ps -o pgid= -p "$candidate" 2>/dev/null | tr -d '[:space:]')"
    [ "$listener_group" = "$root" ] && return 0
  fi
  while [ "$depth" -lt 128 ]; do
    parent="$(ps -o ppid= -p "$candidate" 2>/dev/null | tr -d '[:space:]')"
    [[ "$parent" =~ ^[0-9]+$ ]] || return 1
    [ "$parent" = "$root" ] && return 0
    [ "$parent" -le 1 ] && return 1
    candidate="$parent"
    depth=$((depth + 1))
  done
  return 1
}

cleanup() {
  local original_status=$?
  if [ -n "$OWNED_SERVER_PID" ] &&
    process_belongs_to_launch \
      "$OWNED_SERVER_PID" \
      "$OWNED_PID" \
      "$OWNED_PID_STARTED_TICKS" \
      "$OWNED_LAUNCH_NOT_BEFORE_TICKS"; then
    if command -v taskkill.exe >/dev/null 2>&1; then
      windows_process_matches_identity "$OWNED_SERVER_PID" "$OWNED_SERVER_STARTED_TICKS" &&
        windows_taskkill_if_same "$OWNED_SERVER_PID" "$OWNED_SERVER_STARTED_TICKS" ||
        true
    else
      kill "$OWNED_SERVER_PID" 2>/dev/null || true
    fi
  fi
  if [ -n "$OWNED_PID" ]; then
    if command -v taskkill.exe >/dev/null 2>&1; then
      if windows_taskkill_if_same "$OWNED_PID" "$OWNED_PID_STARTED_TICKS"; then
        wait "$OWNED_PID" 2>/dev/null || true
      fi
    else
      kill -- "-$OWNED_PID" 2>/dev/null || kill "$OWNED_PID" 2>/dev/null || true
      for _ in $(seq 1 20); do
        kill -0 "$OWNED_PID" 2>/dev/null || break
        sleep 0.1
      done
      kill -9 "$OWNED_PID" 2>/dev/null || true
      wait "$OWNED_PID" 2>/dev/null || true
    fi
  fi
  if [ -n "$OWNED_DATA" ] && [[ "$OWNED_DATA" == */cliphub-smoke.* ]] && [ -d "$OWNED_DATA" ]; then
    for _ in $(seq 1 20); do
      rm -r -- "$OWNED_DATA" 2>/dev/null && break
      sleep 0.1
    done
    if [ -d "$OWNED_DATA" ]; then
      echo "could not remove isolated smoke data: $OWNED_DATA" >&2
      original_status=1
    fi
  fi
  return "$original_status"
}
trap cleanup EXIT

if [ ! -f "$DEMO" ]; then
  echo "demo not found: $DEMO" >&2
  exit 1
fi

if [ -n "$BASE" ]; then
  IFS= read -r CAPABILITY
  if [[ ! "$CAPABILITY" =~ ^[0-9a-f]{64}$ ]]; then
    echo "pipe the external orchestrator session capability to stdin (64 lowercase hex characters)" >&2
    exit 1
  fi
else
  CAPABILITY="$(od -An -N32 -tx1 /dev/urandom | tr -d '[:space:]')"
  HTTP_ADDR="${ZV_SMOKE_HTTP_ADDR:-127.0.0.1:18080}"
  OWNED_PORT="${HTTP_ADDR##*:}"
  BASE="http://$HTTP_ADDR"
  ZV="./bin/zv"
  if [ ! -x "$ZV" ] && [ -x "$ZV.exe" ]; then
    ZV="$ZV.exe"
  fi
  if [ ! -x "$ZV" ]; then
    echo "zv binary not found; build first with ./scripts/build.ps1" >&2
    exit 1
  fi
  if ! EXISTING_PID="$(listener_pid "$OWNED_PORT")"; then
    echo "could not inspect smoke port $OWNED_PORT for an existing listener" >&2
    exit 1
  fi
  if [ -n "$EXISTING_PID" ]; then
    echo "smoke port $OWNED_PORT is already owned by process $EXISTING_PID" >&2
    exit 1
  fi
  OWNED_DATA="$(mktemp -d "${TMPDIR:-/tmp}/cliphub-smoke.XXXXXXXX")"
  # Canonical launch contract: ./bin/zv serve.
  SERVER_COMMAND=("$ZV" serve)
  if ! command -v taskkill.exe >/dev/null 2>&1 && command -v setsid >/dev/null 2>&1; then
    SERVER_COMMAND=(setsid "${SERVER_COMMAND[@]}")
    OWNED_PROCESS_GROUP="1"
  fi
  if command -v taskkill.exe >/dev/null 2>&1; then
    if ! OWNED_LAUNCH_NOT_BEFORE_TICKS="$(windows_utc_now_ticks)"; then
      echo "could not capture the isolated smoke launch timestamp" >&2
      exit 1
    fi
  fi
  ZV_DATABASE_URL=memory \
    ZV_DATA_DIR="$OWNED_DATA" \
    ZV_HTTP_ADDR="$HTTP_ADDR" \
    ZV_MUTATION_TOKEN="$CAPABILITY" \
    "${SERVER_COMMAND[@]}" >"$OWNED_DATA/orchestrator.log" 2>&1 &
  OWNED_PID=$!
  if command -v taskkill.exe >/dev/null 2>&1; then
    OWNED_PID_STARTED_TICKS="$(
      windows_process_started_ticks "$OWNED_PID"
    )" || OWNED_PID_STARTED_TICKS=""
  fi
  FOREIGN_LISTENER_PID=""
  LISTENER_LOOKUP_FAILED=""
  for _ in $(seq 1 40); do
    if ! CURRENT_LISTENER_PID="$(listener_pid "$OWNED_PORT")"; then
      LISTENER_LOOKUP_FAILED="1"
      break
    fi
    if [ -n "$CURRENT_LISTENER_PID" ]; then
      if [ -n "$OWNED_SERVER_PID" ]; then
        if [ "$CURRENT_LISTENER_PID" != "$OWNED_SERVER_PID" ]; then
          FOREIGN_LISTENER_PID="$CURRENT_LISTENER_PID"
          break
        fi
        if command -v taskkill.exe >/dev/null 2>&1 &&
          ! windows_process_matches_identity "$OWNED_SERVER_PID" "$OWNED_SERVER_STARTED_TICKS"; then
          LISTENER_LOOKUP_FAILED="1"
          break
        fi
      elif process_belongs_to_launch \
        "$CURRENT_LISTENER_PID" \
        "$OWNED_PID" \
        "$OWNED_PID_STARTED_TICKS" \
        "$OWNED_LAUNCH_NOT_BEFORE_TICKS"; then
        if command -v taskkill.exe >/dev/null 2>&1; then
          if ! CURRENT_LISTENER_STARTED_TICKS="$(
            windows_process_started_ticks "$CURRENT_LISTENER_PID"
          )"; then
            LISTENER_LOOKUP_FAILED="1"
            break
          fi
          if ! VERIFIED_LISTENER_PID="$(listener_pid "$OWNED_PORT")" ||
            [ "$VERIFIED_LISTENER_PID" != "$CURRENT_LISTENER_PID" ] ||
            ! windows_process_matches_identity \
              "$CURRENT_LISTENER_PID" \
              "$CURRENT_LISTENER_STARTED_TICKS" ||
            ! process_belongs_to_launch \
              "$CURRENT_LISTENER_PID" \
              "$OWNED_PID" \
              "$OWNED_PID_STARTED_TICKS" \
              "$OWNED_LAUNCH_NOT_BEFORE_TICKS"; then
            LISTENER_LOOKUP_FAILED="1"
            break
          fi
          OWNED_SERVER_STARTED_TICKS="$CURRENT_LISTENER_STARTED_TICKS"
        fi
        OWNED_SERVER_PID="$CURRENT_LISTENER_PID"
      else
        FOREIGN_LISTENER_PID="$CURRENT_LISTENER_PID"
        break
      fi
    fi
    if [ -n "$OWNED_SERVER_PID" ] && curl -fsS -o /dev/null "$BASE/healthz"; then
      break
    fi
    sleep 0.25
  done
  if [ -n "$LISTENER_LOOKUP_FAILED" ]; then
    echo "could not resolve the isolated smoke listener process at $BASE" >&2
    exit 1
  fi
  if [ -n "$FOREIGN_LISTENER_PID" ]; then
    echo "smoke port $OWNED_PORT became owned by unrelated process $FOREIGN_LISTENER_PID" >&2
    exit 1
  fi
  if [ -z "$OWNED_SERVER_PID" ]; then
    echo "could not resolve the isolated smoke listener process at $BASE" >&2
    exit 1
  fi
  if ! curl -fsS -o /dev/null "$BASE/healthz"; then
    echo "isolated smoke orchestrator did not become healthy at $BASE" >&2
    exit 1
  fi
  echo "→ started isolated smoke orchestrator at $BASE"
fi

HEADER_NAME="X-ClipHub-"Token
curl_auth() {
  printf 'header = "%s: %s"\n' "$HEADER_NAME" "$CAPABILITY" | curl --config - "$@"
}

echo "→ uploading $DEMO with target=$TARGET"
JOB=$(curl_auth -fsS -X POST "$BASE/api/jobs" \
  -F "demo=@$DEMO" \
  -F "config={\"target_steamid\":\"$TARGET\"}")
printf '%s\n' "$JOB" >&2
ID=$(echo "$JOB" | python3 -c "import sys,json;print(json.load(sys.stdin)['id'])")

echo "→ job id = $ID; polling status…"
for i in $(seq 1 60); do
  STATUS=$(curl_auth -fsS "$BASE/api/jobs/$ID" | python3 -c "import sys,json;print(json.load(sys.stdin)['status'])")
  echo "  [$i] status=$STATUS"
  case "$STATUS" in
    parsed) break ;;
    failed) echo "job failed" >&2; exit 2 ;;
  esac
  sleep 2
done

if [ "$STATUS" != "parsed" ]; then
  echo "timeout waiting for parse" >&2
  exit 3
fi

echo "→ fetching plan"
curl_auth -fsS "$BASE/api/jobs/$ID/plan" | python3 -m json.tool | head -40
echo "✔ smoke test passed"
