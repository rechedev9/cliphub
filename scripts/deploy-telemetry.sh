#!/usr/bin/env bash
# Build and deploy the telemetry collector and alerter to the VPS.
#
# Host details live only in a git-ignored local file (default
# .telemetry-deploy.local at the repo root; override with CLIPHUB_DEPLOY_ENV):
#   DEPLOY_SSH_TARGET=<ssh alias of the VPS>
# The ssh user must be root or have passwordless sudo.
#
# Steps: versioned linux/amd64 build, upload with checksum, pre-deploy copy of
# the collector databases, atomic install keeping .previous, restart, admin
# health check for the new version with db_ok, automatic rollback on failure,
# then install and (once configured) enable the alerter timer.
#
# Usage: bash scripts/deploy-telemetry.sh [--force-rollback]
#   --force-rollback  deploy, then fail the health check on purpose to prove
#                     the rollback path.
# Nothing here prints host details.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

mode="deploy"
case "${1:-}" in
  "") ;;
  --force-rollback) mode="force-rollback" ;;
  *)
    printf 'usage: %s [--force-rollback]\n' "$0" >&2
    exit 2
    ;;
esac

local_env="${CLIPHUB_DEPLOY_ENV:-$root/.telemetry-deploy.local}"
if [[ ! -f "$local_env" ]]; then
  printf 'missing %s with DEPLOY_SSH_TARGET=<ssh alias>\n' "$(basename "$local_env")" >&2
  exit 2
fi
# shellcheck disable=SC1090
source "$local_env"
: "${DEPLOY_SSH_TARGET:?set DEPLOY_SSH_TARGET in the local deploy env}"

version="$(git describe --always --dirty)"
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

echo "== build $version (linux/amd64) =="
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-X main.version=$version" \
  -o "$workdir/cliphub-telemetry" ./services/telemetry
(cd "$workdir" && sha256sum cliphub-telemetry > cliphub-telemetry.sha256)
cp deploy/telemetry/cliphub-telemetry.service \
  deploy/telemetry/cliphub-telemetry-alert.service \
  deploy/telemetry/cliphub-telemetry-alert.timer "$workdir/"

cat > "$workdir/remote.sh" <<'REMOTE'
#!/usr/bin/env bash
# Runs on the VPS as root. Arguments: <version> <deploy|force-rollback>.
set -euo pipefail
if [ "$(id -u)" -ne 0 ]; then
  exec sudo -n bash "$0" "$@"
fi
version="$1"
mode="$2"
here="$(cd "$(dirname "$0")" && pwd)"
trap 'rm -rf "$here"' EXIT

install_dir=/opt/cliphub-telemetry
bin="$install_dir/cliphub-telemetry"
unit_dir=/etc/systemd/system
collector=cliphub-telemetry.service
alert=cliphub-telemetry-alert
backup_root=/var/backups/cliphub-telemetry

(cd "$here" && sha256sum -c --quiet cliphub-telemetry.sha256)

# Read the admin address, token and database from the env file the installed
# collector unit already uses, so the repository never names that file.
env_file="$(systemctl show -P EnvironmentFiles "$collector" | awk '{print $1}')"
if [ -z "$env_file" ] || [ ! -r "$env_file" ]; then
  echo "the collector unit has no readable EnvironmentFile" >&2
  exit 1
fi
env_value() {
  sed -n "s/^$1=//p" "$env_file" | tail -n 1 | sed -e 's/^"\(.*\)"$/\1/' -e "s/^'\(.*\)'$/\1/"
}
admin_addr="$(env_value CLIPHUB_TELEMETRY_ADMIN_ADDR)"
admin_addr="${admin_addr:-127.0.0.1:8121}"
admin_token="$(env_value CLIPHUB_TELEMETRY_ADMIN_TOKEN)"
database="$(env_value CLIPHUB_TELEMETRY_DATABASE)"

# The token goes to curl on stdin, never in argv.
admin_health() {
  printf 'silent\nfail\nmax-time = 5\nheader = "Authorization: Bearer %s"\nurl = "http://%s/healthz"\n' \
    "$admin_token" "$admin_addr" | curl --config - 2>/dev/null
}

# wait_for <pattern>...: poll healthz for up to 30 s until every pattern matches.
wait_for() {
  local body pattern ok
  for _ in $(seq 1 30); do
    if body="$(admin_health)"; then
      ok=1
      for pattern in "$@"; do
        case "$body" in *"$pattern"*) ;; *) ok=0 ;; esac
      done
      [ "$ok" -eq 1 ] && return 0
    fi
    sleep 1
  done
  return 1
}

echo "== backup =="
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
install -d -m 0700 "$backup_root/predeploy-$stamp"
install -m 0755 -o root -g root "$here/cliphub-telemetry" "$bin.new"
systemctl stop "$collector"
for file in "$database" "$database-wal" "$database-shm" "$database.logs" "$database.logs-wal" "$database.logs-shm"; do
  if [ -f "$file" ]; then
    cp -p "$file" "$backup_root/predeploy-$stamp/"
  fi
done
# Keep the three newest pre-deploy copies; the collector keeps its own dailies.
find "$backup_root" -mindepth 1 -maxdepth 1 -type d -name 'predeploy-*' | sort | head -n -3 | xargs -r rm -rf

echo "== install $version =="
if [ -f "$bin" ]; then
  cp -p "$bin" "$bin.previous"
fi
if [ -f "$unit_dir/$collector" ]; then
  cp -p "$unit_dir/$collector" "$install_dir/$collector.previous"
fi
mv -f "$bin.new" "$bin"
install -m 0644 "$here/$collector" "$unit_dir/$collector"
systemctl daemon-reload
systemctl start "$collector"

expected_version="\"version\":\"$version\""
if [ "$mode" = "force-rollback" ]; then
  expected_version='"version":"forced-rollback-check"'
fi
if ! wait_for "$expected_version" '"db_ok":true'; then
  echo "health check failed for $version; rolling back" >&2
  systemctl stop "$collector" || true
  if [ -f "$bin.previous" ]; then
    cp -p "$bin.previous" "$bin"
  fi
  if [ -f "$install_dir/$collector.previous" ]; then
    install -m 0644 "$install_dir/$collector.previous" "$unit_dir/$collector"
  fi
  systemctl daemon-reload
  systemctl start "$collector"
  if wait_for '"status":"ok"'; then
    echo "rolled back to the previous collector" >&2
  else
    echo "the previous collector is not healthy either; check journalctl -u $collector" >&2
  fi
  exit 1
fi
sha256sum "$bin" > "$bin.sha256"
echo "collector $version healthy"

echo "== alerter =="
if ! id "$alert" >/dev/null 2>&1; then
  useradd --system --no-create-home --shell /usr/sbin/nologin "$alert"
fi
install -m 0644 "$here/$alert.service" "$here/$alert.timer" "$unit_dir/"
systemctl daemon-reload
if [ -n "$(systemctl show -P EnvironmentFiles "$alert.service")" ]; then
  systemctl enable --now "$alert.timer"
  echo "alert timer enabled"
else
  echo "alert timer installed but not enabled: attach <alert env file> first (docs/telemetry-operations.md)"
fi
REMOTE

echo "== upload =="
ssh_opts=(-o BatchMode=yes -o LogLevel=ERROR)
remote_dir="$(ssh "${ssh_opts[@]}" "$DEPLOY_SSH_TARGET" 'mktemp -d')"
scp -q "${ssh_opts[@]}" "$workdir"/* "$DEPLOY_SSH_TARGET:$remote_dir/"
ssh "${ssh_opts[@]}" "$DEPLOY_SSH_TARGET" \
  "bash $(printf '%q' "$remote_dir/remote.sh") $(printf '%q' "$version") $mode"
echo "== deployed $version =="
