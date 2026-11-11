# ClipHub

ClipHub Studio is a Windows desktop app that turns CS2 demos and streams into edited videos and
analyzes them. The product we sell is that desktop app. `web/` is only its UI inside Electron: there
is no web product, and a browser is only a quick way to test the UI. Design, scope and verify every
change for the installed Windows app. The UI is in Spanish.

## Architecture
- `desktop/src/main.ts` picks stable loopback ports (`stable-ports.ts`), spawns
  `zv-orchestrator.exe` and the web standalone server (`ORCHESTRATOR_URL`, `PORT`), then loads the UI.
- `web/app/api/*` route handlers proxy to the orchestrator. `web/proxy.ts` and
  `lib/api/local-request-guard.ts` reject cross-origin and DNS-rebound requests.
- `cmd/zv-orchestrator`: local HTTP API (`internal/httpapi`) and the job queue. Workers in
  `internal/workers` parse demos in process (`internal/parser`) and run `zv-recorder` (CS2 capture
  through HLAE) and `zv-editor` (FFmpeg render) as subprocesses. The installer ships these three
  binaries plus `zv-mcp.exe`, the MCP server for assistants (`cmd/zv-mcp`); `zv` and the other
  `cmd/` tools are for development.
- State lives in SQLite through `internal/store`; memory repos share contract tests with it.
- `cmd/zv` is the CLI over the same pipeline. `zv check` validates the workflow catalog, usage text
  and command build targets.

## MCP parity
- Every user-facing feature in the Studio UI has a matching tool in `cmd/zv-mcp`: UI users and
  assistant users are both first class.
- A change that adds or changes a UI feature adds or updates its MCP tool and the server
  instructions (`serverInstructions` in `cmd/zv-mcp/main.go`) in the same change, and the tool
  table in `docs/studio-mcp.md`.

## Browser support
- `web/` runs only in the Chromium that Electron bundles (`electron` in `desktop/package.json`). Use any
  web platform feature that Chromium ships, with no fallback and no polyfill.
- `landing/` is a public site. Baseline Widely available features need no fallback; a newer feature
  needs feature detection and a fallback of at most 20 lines with no new dependency.
- Type a newly shipped HTML attribute by augmenting React's attribute types in a `.d.ts`. This
  replaces the `as any` spread that the `modern-web-guidance` skill suggests.

## Build and check
- Go: `go vet ./...`, `go test ./...`, and `go run ./cmd/zv check`. One test:
  `go test ./internal/<pkg> -run TestName -count=1`. A few Full Demo editor tests depend on the
  FFmpeg version; Studio ships FFmpeg 8.1.2.
- `go test ./...` includes `TestNoDeadCode` (`cmd/zv`), which runs pinned `deadcode` and `staticcheck -checks U1000`
  for both `GOOS=windows` and `GOOS=linux`. A function that only tests call is dead: delete it, or move a
  real test helper into a `_test.go` file. A helper only `_windows.go` code calls belongs in a `_windows.go` file.
- Source files stay at or under 1000 lines: `TestGoFileLength` (`cmd/zv`) for Go, `max-lines` in the
  web, desktop and landing oxlint configs for TS. Files already past it are frozen at their size; split, never raise.
  Split by responsibility: each new file (or package, when there is a real API boundary) owns one concern,
  such as `handlers_library.go` and `handlers_streams.go`. A cut by line count (`handlers2.go`) does not count.
  Lower or drop the frozen entry in the same change.
- Fast local Go loop: `go test -short ./...` skips the slow FFmpeg renders; pre-PR runs use the full suite.
  `internal/editor` alone nears the 10 min default on Windows, so pass `-timeout 30m` to a full run.
- web, desktop: `pnpm --dir <pkg> run typecheck`, `lint`, `test:unit`; `run build` for web.
  One test: `pnpm --dir <pkg> exec node --test <path>.test.ts`.
- There is no CI on pushes or PRs. `desktop-release.yml` runs the checks (Go with `-short`, web, desktop) before it publishes a release. Run the checks for what you touched, and the full Go suite before a release.

## Run Studio from source (Windows)
1. `.\scripts\build.ps1` builds the Go binaries into `bin\`.
2. `pnpm --dir desktop run build`, then `pnpm --dir desktop run assemble` (stages `bin\`, the web
   standalone build and HLAE into `desktop\build-resources`).
3. Start it with a throwaway profile, never the real `%APPDATA%\cliphub-studio`. Write
   `{"enabled":true}` to `agent-access.json` in the profile first, so Studio lets the MCP in:
   `$p = "C:\temp\cliphub-try\$(Get-Date -f yyyyMMdd-HHmmss)"; mkdir $p; '{"enabled":true}' | Set-Content -Encoding ascii "$p\agent-access.json"`
   `$env:CLIPHUB_E2E_USER_DATA = $p`
   `pnpm --dir desktop exec electron scripts\isolated-userdata.cjs`
   A new profile shows the first-run guide and fetches HLAE on first boot.
4. Import demos and drive other state changes through the branch's MCP, not the window: run
   `bin\zv-mcp.exe` with `CLIPHUB_STUDIO_USER_DATA` set to the profile and call `import_demo` and
   `wait_for_demo`. A session's `cliphub-studio` MCP talks to the real Studio; never use it for a
   check. The browser pane cannot import: its POSTs lack the window's capability cookie.
5. Capture needs Steam and CS2 on the machine. Use demo files from outside the repo.

## Verify a change
Verify by hand in the real app:
1. Build and start Studio from your branch as above, with a fresh throwaway profile.
2. Load a demo and follow the flow until the "Prepara tu Short" list shows up, the way a user would.
   Exercise the flow you changed along the way.
3. Try one path off the happy path, such as a corrupt `.dem`, and check `studio.log` in the
   throwaway profile for a clear error instead of a crash or silence.
4. In the PR, write a few lines on what you did and saw. A PR that changes what a user sees embeds
   screenshots of the changed screen from the browser pane, attached with `gh pr create|edit --attach`.
   Never commit them.

Drive and check the UI in the built-in browser pane, on the web server that Studio started (its
`[web] - Local:` URL in the Electron output), and import demos through `bin\zv-mcp.exe` as in
"Run Studio from source". Never use computer use for a check.
Do not add e2e suites, verification scripts, evidence folders or reports to the repo.
The one exception is the deterministic Short pipeline test in `internal/pipeline`: plain `go test`,
golden files in its `testdata/`, no CS2, network, real clock or video encode, and no scripts beside it.
Extend that test instead of adding another. When a change moves its output, run
`go test ./internal/pipeline -update` and review the golden diff like code; never accept it blindly.
It does not replace the manual check above. Unit tests are welcome where they test real logic.

## Issues
Outside contributors only file issues, through their own AI agent. `CONTRIBUTING.md` tells that
agent what evidence to collect and `.github/ISSUE_TEMPLATE/bug-report.md` holds the sections.
Keep both in step with the PR body sections and with what `studio_status` returns.

## Releases and domain rules
Release steps are in `docs/incidents.md` (Releases). Before tagging, prove the release with
`gh workflow run desktop-release.yml` from `main`: `dry_run` defaults to true, so it runs the checks
and builds the installer without publishing.

Domain rules and incident history (HLAE pin, Full Demo audio and overlays, telemetry): `docs/incidents.md`.
