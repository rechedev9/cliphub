# ClipHub

ClipHub Studio is a Windows desktop app that turns CS2 demos and streams into edited videos and
analyzes them. The product we sell is that desktop app. `web/` is only its UI inside Electron: there
is no web product, and a browser is only a quick way to test the UI. Design, scope and verify every
change for the installed Windows app. The UI is in Spanish.

This file is the index. Rules live in `.claude/rules/` and load on their own when a session touches
the paths in their frontmatter. Skills (recipes and checklists) live in `.claude/skills/`.
`CLAUDE.md` imports this file.

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
- `cmd/zv` is the CLI over the same pipeline. `zv check` validates the workflow catalog, usage text
  and command build targets.

## Build and check

- Go: `go vet ./...`, `go test ./...`, and `go run ./cmd/zv check`. One test:
  `go test ./internal/<pkg> -run TestName -count=1`.
- Fast local Go loop: `go test -short ./...` skips the slow FFmpeg renders and `TestNoDeadCode`.
  Pre-PR runs use the full suite. `internal/editor` alone nears the 10 min default on Windows, so
  pass `-timeout 30m` to a full run.
- web, desktop: `pnpm --dir <pkg> run typecheck`, `lint`, `test:unit`; `run build` for web.
  One test: `pnpm --dir <pkg> exec node --test <path>.test.ts`.
- A few Full Demo editor tests depend on the FFmpeg version; Studio ships FFmpeg 8.1.2.
  [Render and the editor](.claude/rules/render-editor.md).

## Golden rules

Each rule has a check or a longer page behind it. Follow the link before you work around one.

- Source files stay at or under 1000 lines. Files already past it are frozen: they shrink, never grow, and no max is raised (`TestGoFileLength` in `cmd/zv`, oxlint `max-lines`). [Split a file](.claude/skills/split-file/SKILL.md).
- No production function that only tests call: delete it or move it to a `_test.go` file. A helper that only `_windows.go` code calls lives in a `_windows.go` file. `TestNoDeadCode` in `cmd/zv` runs pinned `deadcode` and `staticcheck -checks U1000` for `GOOS=windows` and `GOOS=linux`; `-short` skips it.
- A UI feature lands with its `cmd/zv-mcp` tool, `serverInstructions` and `docs/studio-mcp.md` row. [MCP parity](.claude/rules/mcp-parity.md).
- A store method lands in SQLite and memory with a shared contract test. [SQLite store](.claude/rules/store-sqlite.md).
- No e2e suites, verification scripts, evidence folders or reports; the `internal/pipeline` golden test is the one exception. Unit tests are welcome where they test real logic. [Render and the editor](.claude/rules/render-editor.md).
- No CI on pushes or pull requests. Run the checks for what you touched; the full Go suite before a PR and before a release. [Release](.claude/rules/release.md).
- Check a user-visible change by hand in a throwaway Studio profile, never the real `%APPDATA%\cliphub-studio` and never through the session's `cliphub-studio` MCP. [Verify in Studio](.claude/skills/verify-in-studio/SKILL.md).
- The repository is public: write `<user>` and `%APPDATA%`, never tokens, keys, support codes or `agent-session.json` contents.

## Index

### Rules

- [HLAE pin and capture](.claude/rules/capture-hlae.md): CS2 updates, the HLAE pin, capture pitfalls.
- [Render and the editor](.claude/rules/render-editor.md): FFmpeg 8.1.2, the `internal/pipeline` golden test, render-attempt ownership, startup reconcile, the Full Demo artifact cache.
- [SQLite store](.claude/rules/store-sqlite.md): migrations and the paired SQLite and memory repositories.
- [MCP parity](.claude/rules/mcp-parity.md): what a UI change updates under `cmd/zv-mcp` and in `docs/studio-mcp.md`.
- [Web inside Electron](.claude/rules/web-electron.md): Chromium-only UI, the local request guard, the capability cookie, `landing/`, React attribute types.
- [Release](.claude/rules/release.md): the bar before a tag, and the one hosted workflow.
- `docs/incidents.md`: domain rules and incident history (HLAE pin, Full Demo audio and overlays, telemetry, performance, release steps). The rules name its sections.

### Skills

- [Feature with MCP](.claude/skills/feature-with-mcp/SKILL.md): a Studio screen, flow or control is added or changed.
- [Store method](.claude/skills/store-method/SKILL.md): `internal/store` gains or changes a method or a migration.
- [Split a file](.claude/skills/split-file/SKILL.md): a file nears 1000 lines, or `TestGoFileLength` or `max-lines` fails.
- [Update pipeline goldens](.claude/skills/update-golden/SKILL.md): `go test ./internal/pipeline` fails on a golden diff.
- [Verify in Studio](.claude/skills/verify-in-studio/SKILL.md): run Studio from source and check a change by hand.
- [Diff review](.claude/skills/diff-review/SKILL.md): review a branch, diff or PR. Read-only.
- [Release check](.claude/skills/release-check/SKILL.md): full checks and a dry-run release workflow before a tag. It does not publish.
- [Shorts thumbnail](.claude/skills/shorts-thumbnail/SKILL.md): pixel-art Shorts thumbnail.

## Issues

Outside contributors only file issues, through their own AI agent. `CONTRIBUTING.md` tells that
agent what evidence to collect and `.github/ISSUE_TEMPLATE/bug-report.md` holds the sections.
Keep both in step with the PR body sections and with what `studio_status` returns
(`cmd/zv-mcp/tools.go`, instructions in `cmd/zv-mcp/main.go`).

## Maintenance

- When agents repeat a mistake, add a test or lint first. Write a rule only for what a check cannot catch.
- A rule states the invariant and names the test, linter or `docs/incidents.md` section behind it. It does not copy counts, lists or messages from the code.
- A rule changes in the same PR as the code it describes. [Diff review](.claude/skills/diff-review/SKILL.md) checks it, and [Release check](.claude/skills/release-check/SKILL.md) rereads the rules before each release.
- Each fact lives in one file; the others link to it.
- Keep this file an index. When a section here grows past a few lines, it moves to a rule or a skill.
