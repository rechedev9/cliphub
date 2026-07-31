#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"

source scripts/go-env.sh
ensure_go_toolchain

if ! command -v codex >/dev/null 2>&1; then
  echo "FAIL: codex CLI not found" >&2
  echo "Install with: npm install -g @openai/codex" >&2
  exit 1
fi

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

echo "== Codex sees AGENTS.md =="
tmp="$(mktemp)"
err="$(mktemp)"
trap 'rm -f "$tmp" "$err"' EXIT
if ! codex --cd "$root" debug prompt-input "harness smoke test" > "$tmp" 2> "$err"; then
  echo "FAIL: codex CLI could not read project prompt context" >&2
  if grep -q "@openai/codex-linux-x64" "$err"; then
    echo "Missing optional dependency @openai/codex-linux-x64. Reinstall with: npm install -g @openai/codex@latest" >&2
  else
    cat "$err" >&2
  fi
  exit 1
fi

grep -q "TickCut is a Windows-local, deterministic CS2 demo/stream-to-video pipeline" "$tmp"
grep -q "AGENTS.md" "$tmp"

echo "== TickCut workflow contract =="
go run ./cmd/zv check

echo "OK: Codex harness is wired"
