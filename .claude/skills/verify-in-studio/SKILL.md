---
name: verify-in-studio
description: Build Studio from the branch and check a change by hand in a throwaway Windows profile. Use when a change affects what a user sees or the capture, render or import flow. Never point this at the real Studio profile or the live cliphub-studio MCP.
---

# Verify in Studio

This is the manual check for the installed Windows app. A browser against `web/` is only a quick look at the UI. Do not add e2e suites, verification scripts, evidence folders or reports to the repo. Unit tests are welcome where they test real logic. The pipeline exception is [Update pipeline goldens](../update-golden/SKILL.md).

Run this on Windows, with Steam and CS2 installed when the change touches capture. Use demo files from outside the repo.

## Throwaway profile

1. `.\scripts\build.ps1` builds the Go binaries into `bin\`.
2. `pnpm --dir desktop run build`, then `pnpm --dir desktop run assemble` (stages `bin\`, the web standalone build and HLAE into `desktop\build-resources`).
3. Start Studio with a throwaway profile, never the real `%APPDATA%\cliphub-studio`. Write `{"enabled":true}` to `agent-access.json` in the profile first, so Studio lets the MCP in:

```powershell
$p = "C:\temp\cliphub-try\$(Get-Date -f yyyyMMdd-HHmmss)"; mkdir $p; '{"enabled":true}' | Set-Content -Encoding ascii "$p\agent-access.json"
$env:CLIPHUB_E2E_USER_DATA = $p
pnpm --dir desktop exec electron scripts\isolated-userdata.cjs
```

A new profile shows the first-run guide and fetches HLAE on first boot.

4. Import demos and drive other state changes through the branch's MCP, not the window. Run `bin\zv-mcp.exe` with `CLIPHUB_STUDIO_USER_DATA` set to the profile and call `import_demo` and `wait_for_demo`. A session's `cliphub-studio` MCP talks to the real Studio; never use it for a check. The browser pane cannot import: its POSTs lack the window's capability cookie. See [Web inside Electron](../../rules/web-electron.md).

## What to exercise

1. Build and start Studio from the branch as above, with a fresh throwaway profile.
2. Load a demo and follow the flow until the "Prepara tu Short" list shows up, the way a user would. Exercise the flow you changed along the way.
3. Try one path off the happy path, such as a corrupt `.dem`, and check `studio.log` in the throwaway profile for a clear error instead of a crash or silence.
4. In the PR, write a few lines on what you did and saw. A PR that changes what a user sees embeds screenshots of the changed screen from the browser pane, attached with `gh pr create` or `gh pr edit --attach`. Never commit them.

Drive and check the UI in the built-in browser pane, on the web server that Studio started (its `[web] - Local:` URL in the Electron output). Never use computer use for a check.

## Checks

The hand check above is the bar for a user-visible change. Before the PR, the automated checks for what you touched are green:

```bash
go vet ./...
go test ./... -count=1 -timeout 30m
go run ./cmd/zv check
```

`go test -short ./...` is the fast loop only. It skips the slow FFmpeg renders and `TestNoDeadCode`. When `web/` or `desktop/` changed:

```bash
pnpm --dir web run typecheck
pnpm --dir web run lint
pnpm --dir web run test:unit
pnpm --dir desktop run typecheck
pnpm --dir desktop run lint
pnpm --dir desktop run test:unit
```

`pnpm --dir web run build` when the web server output changed. One test: `pnpm --dir <pkg> exec node --test <path>.test.ts`, or `go test ./internal/<pkg> -run TestName -count=1`.

The change is finished when the throwaway-profile pass matches the notes in the PR and those commands are green. Redact the PR: `<user>` and `%APPDATA%`, no tokens, keys, support codes, or contents of `agent-session.json`.
