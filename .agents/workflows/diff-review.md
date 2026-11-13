# Diff review

Read-only checklist for a reviewer agent. Run it against the diff. Do not edit the branch from this workflow.

## Parity and size

- A UI feature or a change to one updates the `cmd/zv-mcp` tool, `serverInstructions` in `cmd/zv-mcp/main.go`, and the tool table in `docs/studio-mcp.md` in the same diff. [MCP parity](../rules/mcp-parity.md).
- No Go file crosses 1000 lines, and no frozen file in `oversizedGoFiles` (`cmd/zv/file_length_test.go`) grows. A split is by responsibility. A file that now fits has its frozen entry lowered or dropped. TS uses `max-lines` in the web, desktop, landing and mcp-apps oxlint configs. [Split a file](../skills/split-file/SKILL.md).
- A function only tests call is not left in production code. A helper only `_windows.go` calls lives in a `_windows.go` file. `TestNoDeadCode` (`cmd/zv/deadcode_test.go`) is the check, and `go test -short` skips it, so a `-short` log does not clear this item.

## What must not appear

- No new e2e suite, verification script, evidence folder or report. The one exception is an extension of `internal/pipeline` and its `testdata/`. A golden rewrite has a reviewed diff. [Render and the editor](../rules/render-editor.md).
- No real user path, token, key, support code, or contents of `agent-session.json`. Paths use `<user>` and `%APPDATA%`.
- No second GitHub workflow. `.github/workflows/desktop-release.yml` is the only hosted workflow.

## Rules move with the code

- A behavior change updates the `.agents/rules/` file that describes it, in the same PR. [Maintenance](../../AGENTS.md#maintenance).
- The rule points at the test, the linter or the `docs/incidents.md` section. It does not paste that text.
- A new repository method exists on both the SQLite type and the memory type, with a contract that runs both. [SQLite store](../rules/store-sqlite.md).
- A capture, HLAE, FFmpeg or render change still matches the invariant in [HLAE pin and capture](../rules/capture-hlae.md) or [Render and the editor](../rules/render-editor.md), and the incident section those files name.

## Commands a reviewer can re-run

```bash
go vet ./...
go run ./cmd/zv check
go test ./cmd/zv -run 'TestGoFileLength|TestNoDeadCode' -count=1
```

`TestNoDeadCode` is slow and installs pinned tools. Skip it when the diff cannot add or remove a Go function. Web or desktop diffs also re-run that package's `typecheck`, `lint` and `test:unit`.
