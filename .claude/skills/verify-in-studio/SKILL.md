---
name: verify-in-studio
description: Build ClipHub Studio from the branch, run it from source in a throwaway Windows profile and check a change by hand. Use when a change affects what a user sees or the import, capture or render flow, when asked to run or start Studio from source, to verify a change in the real app, or to take screenshots for a PR. Never uses the real Studio profile or the session's cliphub-studio MCP.
---

# Verify in Studio

This is the manual check for the installed Windows app. A browser against `web/` alone is only a quick look at the UI. Nothing from this check is committed: no e2e suites, verification scripts, evidence folders, reports or screenshots.

Run it on Windows. Capture needs Steam and CS2 on the machine. Use demo files from outside the repo.

## Run Studio from source

1. `.\scripts\build.ps1` builds the Go binaries into `bin\`.
2. `pnpm --dir desktop run build`, then `pnpm --dir desktop run assemble` (stages `bin\`, the web standalone build and HLAE into `desktop\build-resources`).
3. Start Studio with a fresh throwaway profile, never the real `%APPDATA%\cliphub-studio`. Write `{"enabled":true}` to `agent-access.json` in the profile first, so Studio lets the MCP in:

```powershell
$p = "C:\temp\cliphub-try\$(Get-Date -f yyyyMMdd-HHmmss)"; mkdir $p; '{"enabled":true}' | Set-Content -Encoding ascii "$p\agent-access.json"
$env:CLIPHUB_E2E_USER_DATA = $p
pnpm --dir desktop exec electron scripts\isolated-userdata.cjs
```

A new profile shows the first-run guide and fetches HLAE on first boot.

## Drive it

- Import demos and make other state changes through the branch's MCP, not the window: run `bin\zv-mcp.exe` with `CLIPHUB_STUDIO_USER_DATA` set to the profile and call `import_demo` and `wait_for_demo`.
- Never use the session's `cliphub-studio` MCP for a check: it talks to the real Studio.
- Look at and click the UI in the built-in browser pane, on the web server that Studio started (its `[web] - Local:` URL in the Electron output). The browser pane cannot import: its POSTs lack the window's capability cookie. [Web inside Electron](../../rules/web-electron.md).
- Never use computer use for a check.

## What to exercise

1. Load a demo and follow the flow until the "Prepara tu Short" list shows up, the way a user would. Exercise the flow you changed along the way.
2. Try one path off the happy path, such as a corrupt `.dem`, and check `studio.log` in the throwaway profile for a clear error instead of a crash or silence.
3. In the PR, write a few lines on what you did and saw. A PR that changes what a user sees embeds screenshots of the changed screen from the browser pane, attached with `gh pr create --attach` or `gh pr edit --attach`. Never commit them.
4. Redact the PR text and screenshots: `<user>` and `%APPDATA%`, no tokens, keys, support codes or contents of `agent-session.json`.

The check is finished when the changed flow and one failure path behaved as expected in the throwaway profile and the PR says what you did and saw. It does not replace the automated checks in [Build and check](../../../AGENTS.md#build-and-check): before a PR they are green too, with the full Go suite (`-timeout 30m`, not `-short`).
