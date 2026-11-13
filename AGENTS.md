# ClipHub

ClipHub Studio is a Windows desktop app that turns CS2 demos and streams into edited videos and
analyzes them. The product we sell is that desktop app. `web/` is only its UI inside Electron: there
is no web product, and a browser is only a quick way to test the UI. Design, scope and verify every
change for the installed Windows app. The UI is in Spanish.

This file is the index. Rules live in `.claude/rules/` and load on their own when an agent touches the paths in their frontmatter. Skills, including the review and release checklists, live in `.claude/skills/`. `CLAUDE.md` imports this file.

## Architecture

- `desktop/src/main.ts` picks stable loopback ports (`desktop/src/stable-ports.ts`), spawns
  `zv-orchestrator.exe` and the web standalone server (`ORCHESTRATOR_URL`, `PORT`), then loads the UI.
- `web/app/api/*` route handlers proxy to the orchestrator. `web/proxy.ts` and
  `web/lib/api/local-request-guard.ts` reject cross-origin and DNS-rebound requests.
- `cmd/zv-orchestrator`: local HTTP API (`internal/httpapi`) and the job queue. Workers in
  `internal/workers` parse demos in process (`internal/parser`) and run `zv-recorder` (CS2 capture
  through HLAE) and `zv-editor` (FFmpeg render) as subprocesses. The installer ships these three
  binaries plus `zv-mcp.exe`, the MCP server for assistants (`cmd/zv-mcp`); `zv` and the other
  `cmd/` tools are for development.
- State lives in SQLite through `internal/store`; memory repos share contract tests with it.
  [Store](.claude/rules/store-sqlite.md).
- `cmd/zv` is the CLI over the same pipeline. `zv check` validates the workflow catalog, usage text
  and command build targets (`go run ./cmd/zv check`).

## Build and check

- Go: `go vet ./...`, `go test ./...`, and `go run ./cmd/zv check`. One test:
  `go test ./internal/<pkg> -run TestName -count=1`.
- Fast local Go loop: `go test -short ./...` skips the slow FFmpeg renders and `TestNoDeadCode`.
  Pre-PR runs use the full suite. `internal/editor` alone nears the 10 min default on Windows, so
  pass `-timeout 30m` to a full run.
- web, desktop: `pnpm --dir <pkg> run typecheck`, `lint`, `test:unit`; `run build` for web.
  One test: `pnpm --dir <pkg> exec node --test <path>.test.ts`.
- A few Full Demo editor tests depend on the FFmpeg version; Studio ships FFmpeg 8.1.2.
  [Render](.claude/rules/render-editor.md).

## Golden rules

- Source files stay at or under 1000 lines: `TestGoFileLength` (`cmd/zv/file_length_test.go`) for Go, `max-lines` in the web, desktop, landing and mcp-apps oxlint configs for TS. Files already past the cap are frozen at their size and may only shrink; lower or drop the frozen entry in the same change. Split by responsibility, each new file (or package, when there is a real API boundary) owning one concern, such as `handlers_library.go` and `handlers_streams.go`. A cut by line count (`handlers2.go`) does not count. [Split a file](.claude/skills/split-file/SKILL.md).
- `TestNoDeadCode` (`cmd/zv/deadcode_test.go`) runs pinned `deadcode` and `staticcheck -checks U1000` for both `GOOS=windows` and `GOOS=linux`. A function that only tests call is dead: delete it, or move a real test helper into a `_test.go` file. A helper only `_windows.go` code calls belongs in a `_windows.go` file.
- There is no CI on pushes or PRs. `.github/workflows/desktop-release.yml` runs the checks (Go with `-short`, web, desktop) before it publishes a release. Run the checks for what you touched, and the full Go suite before a release. [Release](.claude/rules/release.md).
- Start Studio from a throwaway profile, never the real `%APPDATA%\cliphub-studio`. [Verify in Studio](.claude/skills/verify-in-studio/SKILL.md).
- A session's `cliphub-studio` MCP talks to the real Studio; never use it for a check. Drive imports and other state changes through the branch's `bin\zv-mcp.exe`.
- The repository is public. Write `<user>` and `%APPDATA%` instead of a real profile path, and leave out tokens, keys, support codes and anything from `agent-session.json`.

## Index

### Rules

- [HLAE pin and capture](.claude/rules/capture-hlae.md): CS2 updates, the HLAE pin, capture pitfalls. History: `docs/incidents.md`.
- [Render and the editor](.claude/rules/render-editor.md): FFmpeg 8.1.2, the `internal/pipeline` golden test, render-attempt ownership, startup reconcile, the Full Demo artifact cache.
- [SQLite store](.claude/rules/store-sqlite.md): migrations and the paired SQLite and memory repositories.
- [MCP parity](.claude/rules/mcp-parity.md): a UI feature, its `cmd/zv-mcp` tool, `serverInstructions` and `docs/studio-mcp.md` land in the same change.
- [Web inside Electron](.claude/rules/web-electron.md): Chromium-only UI, the local request guard, the capability cookie, `landing/`, React attribute types.
- [Release](.claude/rules/release.md): `.github/workflows/desktop-release.yml` dry run, the full Go suite, the latest HLAE. Steps: `docs/incidents.md` (Releases).

### Skills

- [Feature with MCP](.claude/skills/feature-with-mcp/SKILL.md)
- [Store method](.claude/skills/store-method/SKILL.md)
- [Split a file](.claude/skills/split-file/SKILL.md)
- [Update pipeline goldens](.claude/skills/update-golden/SKILL.md)
- [Verify in Studio](.claude/skills/verify-in-studio/SKILL.md): the manual throwaway-profile flow.
- [Shorts thumbnail](.claude/skills/shorts-thumbnail/SKILL.md): pixel-art Shorts thumbnail.

### Review and release

- [Diff review](.claude/skills/diff-review/SKILL.md): checklist for a reviewer agent.
- [Release check](.claude/skills/release-check/SKILL.md): full checks and a dry-run release workflow. It does not publish.

## Issues

Outside contributors only file issues, through their own AI agent. `CONTRIBUTING.md` tells that
agent what evidence to collect and `.github/ISSUE_TEMPLATE/bug-report.md` holds the sections.
Keep both in step with the PR body sections and with what `studio_status` returns
(`cmd/zv-mcp/tools.go`, instructions in `cmd/zv-mcp/main.go`).

## Maintenance

- A rule changes in the same PR as the code it describes.
- A rule points at the test, the linter or the `docs/incidents.md` section that enforces it, and does not copy that text.
- [Diff review](.claude/skills/diff-review/SKILL.md) checks that a behavior change updated its rule in the same PR.
- Review the rules at each release. That step is in [Release check](.claude/skills/release-check/SKILL.md).
