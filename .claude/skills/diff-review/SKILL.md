---
name: diff-review
description: Read-only checklist to review a ClipHub diff against the repo rules (MCP parity, 1000-line cap, dead code, store contracts, no new e2e or pull_request CI, secrets, rules updated with the code). Use when asked to review a branch, diff, commit or PR in this repo, or to self-check a change before opening a PR.
---

# Diff review

Read-only. Check the diff against each item and report what fails. Do not edit the branch from this skill.

## Parity and size

- A UI feature or a change to one updates the `cmd/zv-mcp` tool, `serverInstructions` in `cmd/zv-mcp/main.go`, and the tool table in `docs/studio-mcp.md` in the same diff. [MCP parity](../../rules/mcp-parity.md).
- No Go file crosses 1000 lines, and no frozen file in `oversizedGoFiles` (`cmd/zv/file_length_test.go`) grows. TS uses `max-lines` in the oxlint configs. A split is by responsibility, and a file that now fits has its frozen entry lowered or dropped. [Split a file](../split-file/SKILL.md).
- No function that only tests call is left in production code, and a helper only `_windows.go` code calls lives in a `_windows.go` file. `TestNoDeadCode` (`cmd/zv/deadcode_test.go`) is the check. `go test -short` skips it, so a `-short` log does not clear this item.
- A new or changed repository method exists on both the SQLite type and the memory type, with a contract test that runs both. [SQLite store](../../rules/store-sqlite.md).

## What must not appear

- No new e2e suite, verification script, evidence folder or report. The one exception is an extension of `internal/pipeline` and its `testdata/`, and a golden rewrite has a reviewed diff. [Render and the editor](../../rules/render-editor.md).
- No real user path, token, key, support code, or contents of `agent-session.json`. Paths use `<user>` and `%APPDATA%`.
- No `pull_request` trigger, branch `push` trigger or second workflow under `.github/workflows/`. [Release](../../rules/release.md).

## Rules move with the code

- A behavior change updates the `.claude/rules/` file that describes it, in the same PR. [Maintenance](../../../AGENTS.md#maintenance).
- A rule points at the test, the linter or the `docs/incidents.md` section. It does not paste that text.
- A capture, HLAE, FFmpeg or render change still matches the invariant in [HLAE pin and capture](../../rules/capture-hlae.md) or [Render and the editor](../../rules/render-editor.md), and the incident section those files name.

## Commands a reviewer can re-run

```bash
go vet ./...
go run ./cmd/zv check
go test ./cmd/zv -run 'TestGoFileLength|TestNoDeadCode' -count=1
```

`TestNoDeadCode` is slow and installs pinned tools. Skip it when the diff cannot add or remove a Go function. For a web or desktop diff, also re-run that package's `typecheck`, `lint` and `test:unit`.

The review is finished when every item above is reported as pass, fail (with file and line) or not applicable.
