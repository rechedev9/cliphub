#!/usr/bin/env bash
# Hosted CI infra lane: actionlint + unsigned-release contract + semver pin.
# Linux/amd64 only (ubuntu-latest and this cloud VM). Not HLAE/CS2 E2E.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

ACTIONLINT_VERSION="${ACTIONLINT_VERSION:-1.7.12}"
ACTIONLINT_SHA256="${ACTIONLINT_SHA256:-8aca8db96f1b94770f1b0d72b6dddcb1ebb8123cb3712530b08cc387b349a3d8}"

if [ "$(uname -s)" != "Linux" ] || [ "$(uname -m)" != "x86_64" ]; then
  echo "scripts/ci-infra.sh pins actionlint for linux/amd64 only" >&2
  exit 1
fi

workdir="$(mktemp -d)"
cleanup() {
  rm -rf "$workdir"
}
trap cleanup EXIT

archive="actionlint_${ACTIONLINT_VERSION}_linux_amd64.tar.gz"
curl -fsSL -o "$workdir/$archive" \
  "https://github.com/rhysd/actionlint/releases/download/v${ACTIONLINT_VERSION}/${archive}"
printf '%s  %s\n' "$ACTIONLINT_SHA256" "$workdir/$archive" | sha256sum -c -
tar -xzf "$workdir/$archive" -C "$workdir" actionlint

echo "== actionlint ${ACTIONLINT_VERSION} =="
"$workdir/actionlint" -version
"$workdir/actionlint" -color

echo "== hosted pipeline contract =="
node --test desktop/scripts/release-workflow.test.mjs desktop/scripts/ci-lanes.test.mjs
