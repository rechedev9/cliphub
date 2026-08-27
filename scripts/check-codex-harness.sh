#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"

source scripts/go-env.sh
ensure_go_toolchain

echo "== shell syntax =="
# Bash 3.2 (macOS /bin/bash) has no mapfile; keep a portable file list.
shell_scripts=()
while IFS= read -r script; do
  shell_scripts+=("$script")
done < <(find scripts -maxdepth 1 -type f -name '*.sh' | sort)
if [ "${#shell_scripts[@]}" -eq 0 ]; then
  echo "FAIL: no scripts/*.sh files found" >&2
  exit 1
fi
bash -n "${shell_scripts[@]}"

echo "== harness structure =="
test -L AGENTS.md
test "$(readlink AGENTS.md)" = "CLAUDE.md"
test -f scripts/codex-harness.ps1
scripts/codex-run.sh .codex/prompts/go-plan.md "harness smoke test" >/dev/null

echo "== ClipHub workflow contract =="
go run ./cmd/zv check

echo "OK: Codex harness is wired"
