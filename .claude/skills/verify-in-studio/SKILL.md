---
name: verify-in-studio
description: Build ClipHub from the branch, run Studio (desktop) in a throwaway Windows profile or ClipMind (web edition) on a throwaway data dir, and drive a user-facing flow from the UI with evidence. The feature map in features/ says which edition has each flow and how to drive it. Use when a change affects what a user sees or the import, capture or render flow, when asked to run Studio or ClipMind from source, to verify a change in the real app, or to take screenshots for a PR. Never uses the real Studio profile or the session's cliphub-studio MCP.
---

# Verify in Studio

The manual check of a user-visible change in the real app, in every edition that shows it. [The feature map](features/README.md) lists what a user can reach, which edition has it, and how to drive and prove it. A browser against `next dev` alone is only a quick look.

Nothing from this check is committed: no e2e suites, verification scripts, evidence folders, reports or screenshots. The commands below are run by hand, and the evidence stays in `C:\temp\cliphub-evidence\` until the PR carries what it needs.

Run it on Windows. Use demo files from outside the repo. Capture needs Steam and CS2.

## Build

In a linked worktree beside the main checkout, `build.ps1` stops with "carries no vcs.revision": Go reads VCS data only from a `.git` directory, and a linked worktree has a `.git` file. Build in a shared clone of your branch instead. The clone checks out the worktree's branch, and the diff carries uncommitted edits over (stage new files with `git add -N` first):

```powershell
$worktree = 'C:\path\to\your\worktree'
$repo = "C:\temp\cliphub-build\$(Get-Date -f yyyyMMdd-HHmmss)"
git clone --shared $worktree $repo
if ($LASTEXITCODE) { throw 'git clone failed' }
git -C $worktree diff HEAD --binary | git -C $repo apply --binary
if ($LASTEXITCODE) { throw 'git apply failed' }
```

Delete the clone when the check is done: its `node_modules` and build output take gigabytes. Build in `$repo` (the clone, or a checkout whose `.git` is a directory). The chain stops at the first failure:

```powershell
Set-Location -LiteralPath $repo -ErrorAction Stop
pnpm --dir web install --frozen-lockfile &&
  pnpm --dir desktop install --frozen-lockfile &&
  .\scripts\build.ps1 &&
  pnpm --dir desktop run build &&
  node desktop/scripts/seed-bundled-music.mjs &&
  pnpm --dir desktop run assemble
```

`assemble` stops on a missing `data\music\<track>.mp3` until `seed-bundled-music.mjs` fetches the local-only tracks from the pinned release. The build takes minutes, and the Next build runs twice because `assemble` builds `web` again.

## Launch Studio

```powershell
$repo = '<checkout with desktop\build-resources>'
if (-not (Test-Path "$repo\desktop\build-resources\bin\zv-orchestrator.exe")) { throw "no Studio build in '$repo'" }
$stamp = Get-Date -f yyyyMMdd-HHmmss
$p = "C:\temp\cliphub-try\$stamp"; $e = "C:\temp\cliphub-evidence\$stamp"
New-Item -ItemType Directory -Force $p, $e | Out-Null
'{"enabled":true}' | Set-Content -Encoding ascii "$p\agent-access.json"
$env:CLIPHUB_E2E_USER_DATA = $p
$studio = Start-Process "$repo\desktop\node_modules\electron\dist\electron.exe" "`"$repo\desktop\scripts\isolated-userdata.cjs`"" -WorkingDirectory "$repo\desktop" -RedirectStandardOutput "$e\electron.out.log" -RedirectStandardError "$e\electron.err.log" -PassThru
$studio.Id | Set-Content "$e\electron.pid"
```

- `agent-access.json` lets the branch's MCP in. `isolated-userdata.cjs` points Studio at `$p`, never at `%APPDATA%\cliphub-studio`.
- Studio is ready when `$p\agent-session.json` exists: Studio writes it after the orchestrator and the web server answer. A new profile took 15 to 35 s on 2026-11-19. The first boot installs HLAE, FFmpeg and yt-dlp into `$p\tools` and needs the network.
- `$p\ports.json` holds the `orchestrator` and `web` ports. The UI is at `http://127.0.0.1:<web>`.
- The Studio window opens on the desktop. Leave it alone: the check drives a browser, not the window.
- Shell state does not survive between tool calls. Start each later block by setting `$repo`, `$p` and `$e` again from `$stamp`.

## Doctor

Run it first, and again whenever something looks wrong. It only reads.

```powershell
$ports = Get-Content "$p\ports.json" | ConvertFrom-Json
$session = Get-Content "$p\agent-session.json" | ConvertFrom-Json
[pscustomobject]@{
  studio_alive = [bool](Get-Process -Id (Get-Content "$e\electron.pid") -ErrorAction SilentlyContinue)
  orchestrator_exe = (Get-Process -Id (Get-NetTCPConnection -LocalPort $ports.orchestrator -State Listen).OwningProcess).Path
  healthz = (Invoke-WebRequest "http://127.0.0.1:$($ports.orchestrator)/healthz" -UseBasicParsing).StatusCode
  web = (Invoke-WebRequest "http://127.0.0.1:$($ports.web)/partidas" -UseBasicParsing).StatusCode
  assistant_access = $session.enabled
  version = $session.studioVersion
}
```

The instance is worth driving when `studio_alive` is True, `orchestrator_exe` is under `$repo\desktop\build-resources\bin\`, both codes are 200, `assistant_access` is True, and `version` equals the one in `desktop/package.json`. Never print `agent-session.json` whole: it holds the token.

## Drive

Two channels. Never the Studio window, and never computer use.

State changes go through the branch's `bin\zv-mcp.exe` with `CLIPHUB_STUDIO_USER_DATA` set to `$p`. Never use the session's `cliphub-studio` MCP: it talks to the real Studio. Save this helper as `$p\mcp.ps1`, so it goes with the profile:

```powershell
function Invoke-StudioMcp {
    param([Parameter(Mandatory)][string]$Tool, [hashtable]$Arguments = @{})
    $runProfile = $env:CLIPHUB_STUDIO_USER_DATA
    # An empty value makes zv-mcp fall back to the real %APPDATA%\cliphub-studio.
    if ($runProfile -notmatch '^C:\\temp\\cliphub-try\\\d{8}-\d{6}$' -or -not (Test-Path "$runProfile\agent-session.json")) {
        throw "CLIPHUB_STUDIO_USER_DATA is not a running throwaway profile: '$runProfile'"
    }
    $psi = [Diagnostics.ProcessStartInfo]::new((Join-Path $env:CLIPHUB_REPO 'bin\zv-mcp.exe'))
    $psi.RedirectStandardInput = $true; $psi.RedirectStandardOutput = $true; $psi.UseShellExecute = $false
    $psi.Environment['CLIPHUB_STUDIO_USER_DATA'] = $runProfile
    foreach ($name in 'ORCHESTRATOR_URL', 'ZV_MUTATION_TOKEN') { $null = $psi.Environment.Remove($name) }
    $mcp = [Diagnostics.Process]::Start($psi)
    try {
        $send = { param($message) $mcp.StandardInput.WriteLine(($message | ConvertTo-Json -Depth 20 -Compress)) }
        $read = { param($id) while ($true) { $line = $mcp.StandardOutput.ReadLine(); if ($null -eq $line) { throw 'zv-mcp closed stdout' }; $msg = $line | ConvertFrom-Json; if ($msg.id -eq $id) { return $msg } } }
        & $send @{ jsonrpc = '2.0'; id = 1; method = 'initialize'; params = @{ protocolVersion = '2025-06-18'; capabilities = @{}; clientInfo = @{ name = 'verify-in-studio'; version = '1' } } }
        $null = & $read 1
        & $send @{ jsonrpc = '2.0'; method = 'notifications/initialized' }
        & $send @{ jsonrpc = '2.0'; id = 2; method = 'tools/call'; params = @{ name = $Tool; arguments = $Arguments } }
        $reply = & $read 2
        if ($reply.error) { throw "zv-mcp $Tool error: $($reply.error.message)" }
        if ($reply.result.isError) { throw "zv-mcp $Tool failed: $($reply.result.content[0].text)" }
        if ($reply.result.structuredContent) { return $reply.result.structuredContent }
        return $reply.result.content[0].text
    } finally {
        $mcp.StandardInput.Close()
        if (-not $mcp.WaitForExit(5000)) { $mcp.Kill() }
    }
}
```

Each call starts one `zv-mcp.exe` and stops it, so no MCP process outlives the block:

```powershell
$env:CLIPHUB_REPO = $repo; $env:CLIPHUB_STUDIO_USER_DATA = $p; . "$p\mcp.ps1"
Invoke-StudioMcp studio_status
```

- The helper refuses to start unless `CLIPHUB_STUDIO_USER_DATA` is a run profile with `agent-session.json` (Studio writes it after boot and removes it on a clean exit, not after a crash), and it drops `ORCHESTRATOR_URL` and `ZV_MUTATION_TOKEN` from the child. Either one would point `zv-mcp` at another Studio.
- One process per call cannot carry an approval. A tool that asks for one (`generate_short`, `render_full_demo`, the deletes) answers this client with an approval code that only the issuing process accepts, and a new process also forgets that a capture was just queued, so its first `wait_for_job` can read an old state. Run those tools from one MCP session that lives through the approval and the polling, such as an assistant connected to `$repo\bin\zv-mcp.exe` with `CLIPHUB_STUDIO_USER_DATA` set to `$p`. This skill has not proven that path yet.
- `studio_status` returns a support code. Never paste it into a PR.

The UI is your browser tool (the built-in browser pane, or the agent's own browser) at `http://127.0.0.1:<web>`. It reads and clicks. A POST from it fails without the window's capability cookie, so it cannot import or start a job: [Web inside Electron](../../rules/web-electron.md). Use the handles in the feature files (`href`, role and name, `aria-label`, `aria-current`) instead of coordinates.

For a change, exercise:

1. The flow you changed, from its row in [the feature map](features/README.md), in every edition that shows it.
2. One path off the happy path, such as a corrupt `.dem`, and the clear error in `$p\studio.log` instead of a crash or silence.

The reference journey, proven on 2026-11-19: import a demo through the MCP, then Partidas, the match page, its Replay tab and a round switch, plus the corrupt `.dem` ([Partidas](features/partidas.md), [Replay 2D](features/replay.md)).

## ClipMind (web edition)

Run it when the change shows in the web edition. Build it in the same checkout after Studio's `assemble`, because the web build replaces `web\.next`:

```powershell
$repo = '<checkout with the Studio build>'
Set-Location -LiteralPath $repo -ErrorAction Stop
go build -o "$repo\bin\clipmind-server.exe" ./cmd/clipmind-server
if ($LASTEXITCODE) { throw 'clipmind-server build failed' }
$env:NEXT_PUBLIC_CLIPHUB_EDITION = 'web'
pnpm --dir web run build
$webBuild = $LASTEXITCODE; Remove-Item Env:NEXT_PUBLIC_CLIPHUB_EDITION
if ($webBuild) { throw 'web edition build failed' }
```

`pnpm run build` also compiles the WebAssembly scanner that `/importar` needs. Launch on a throwaway data dir with a confirmed test account:

```powershell
$repo = '<checkout with bin\clipmind-server.exe and the web build>'
if (-not (Test-Path "$repo\bin\clipmind-server.exe")) { throw "no ClipMind build in '$repo'" }
$stamp = Get-Date -f yyyyMMdd-HHmmss
$c = "C:\temp\clipmind-try\$stamp"; $e = "C:\temp\cliphub-evidence\clipmind-$stamp"
New-Item -ItemType Directory -Force $c, $e | Out-Null
function Get-FreePort { $l = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0); $l.Start(); $port = $l.LocalEndpoint.Port; $l.Stop(); $port }
$ports = @{ api = Get-FreePort; web = Get-FreePort }; $ports | ConvertTo-Json | Set-Content "$c\ports.json"
$env:CLIPMIND_DATA_DIR = "$c\data"
$password = -join ((48..57) + (97..122) | Get-Random -Count 20 | ForEach-Object { [char]$_ }); $password | Set-Content "$c\password.txt"
$password | & "$repo\bin\clipmind-server.exe" user add -username verify -email verify@example.invalid -name Verify
$env:CLIPMIND_ADDR = "127.0.0.1:$($ports.api)"; $env:CLIPMIND_PUBLIC_URL = "http://127.0.0.1:$($ports.web)"; $env:CLIPMIND_COMMUNITY_URL = 'off'
$server = Start-Process "$repo\bin\clipmind-server.exe" -RedirectStandardOutput "$e\server.out.log" -RedirectStandardError "$e\server.err.log" -PassThru -WindowStyle Hidden
$env:PORT = "$($ports.web)"; $env:HOSTNAME = '127.0.0.1'; $env:CLIPMIND_URL = "http://127.0.0.1:$($ports.api)"
$web = Start-Process node "`"$repo\web\scripts\start-standalone.mjs`"" -WorkingDirectory "$repo\web" -RedirectStandardOutput "$e\web.out.log" -RedirectStandardError "$e\web.err.log" -PassThru -WindowStyle Hidden
@{ server = $server.Id; web = $web.Id } | ConvertTo-Json | Set-Content "$e\pids.json"
```

It is ready when `$e\web.out.log` has "Ready" and `$e\server.err.log` has "clipmind-server listening on". `CLIPMIND_COMMUNITY_URL=off` keeps the run offline, so "Públicas" and other community data show only their disabled state. To check a change there, leave the variable out: the server then reads the public community server. Doctor:

```powershell
$ports = Get-Content "$c\ports.json" | ConvertFrom-Json
foreach ($path in '/partidas', '/shorts') { $r = Invoke-WebRequest "http://127.0.0.1:$($ports.web)$path" -UseBasicParsing -MaximumRedirection 0 -SkipHttpErrorCheck -ErrorAction SilentlyContinue; "$path -> $($r.StatusCode) $($r.Headers.Location)" }
(Get-Process -Id (Get-NetTCPConnection -LocalPort $ports.api -State Listen).OwningProcess).Path
```

Expect `/partidas -> 307 /iniciar-sesion?next=%2Fpartidas`, `/shorts -> 307 /descargar`, and the API port owned by `$repo\bin\clipmind-server.exe`. Then open `http://127.0.0.1:<web>/iniciar-sesion?next=%2Fpartidas` and sign in as `verify` with the password in `$c\password.txt` (a throwaway account on a throwaway server) and drive [ClipMind](features/clipmind.md). Here the browser is the real client, so imports and edits run through the UI.

## Evidence

- Where: `$e`, under `C:\temp\cliphub-evidence\`, outside the profile and the repo. Cleanup never deletes it. Delete it once the PR carries what it needs. Browser tools save screenshots in their own folder: copy them into `$e`.
- What: a screenshot before and after the action, the MCP read-back as JSON (such as `wait_for_demo.json` and `get_match.json`), `studio.log` copied before cleanup, and the side effect on disk (such as `$p\data\library\<demo_id>\demo.dem`).
- Standards: drive the real user path, never internal setters or test-only routes. Capture the action and the state it produced, not only the last screen. Check side effects as well as the screen, and compare the MCP read-back with what the UI shows.
- In the PR: a few lines on what you did and saw. A PR that changes what a user sees embeds screenshots of the changed screen, attached with `gh pr create --attach` or `gh pr edit --attach`, never committed.
- Redact the PR text and screenshots: `<user>` and `%APPDATA%`, no tokens, keys, support codes or `agent-session.json` contents.

## Clean up

Stop only what you started. The user's installed Studio (`ClipHub Studio.exe`, `zv-orchestrator.exe`, `zv-mcp.exe` under `%LOCALAPPDATA%\Programs`) can run beside yours, so never stop a process by name. A saved PID is checked against the executable you started before anything is stopped, because Windows reuses the PID of a process that already exited.

Studio:

```powershell
if ($p -notmatch '^C:\\temp\\cliphub-try\\\d{8}-\d{6}$') { throw "not a run profile: '$p'" }
$ports = Get-Content "$p\ports.json" | ConvertFrom-Json
Copy-Item "$p\studio.log" $e
$owned = Get-CimInstance Win32_Process -Filter "ProcessId = $(Get-Content "$e\electron.pid")"
if ($owned.ExecutablePath -eq "$repo\desktop\node_modules\electron\dist\electron.exe") { taskkill /T /F /PID $owned.ProcessId } else { 'Studio already exited or the PID is not ours: nothing stopped' }
Get-NetTCPConnection -LocalPort $ports.orchestrator, $ports.web -State Listen -ErrorAction SilentlyContinue |
  ForEach-Object { Get-Process -Id $_.OwningProcess } | Where-Object Path -like "$repo\*" | Stop-Process -Force
Remove-Item -Recurse -Force -LiteralPath $p
```

ClipMind:

```powershell
if ($c -notmatch '^C:\\temp\\clipmind-try\\\d{8}-\d{6}$') { throw "not a run data dir: '$c'" }
$ports = Get-Content "$c\ports.json" | ConvertFrom-Json
$pids = Get-Content "$e\pids.json" | ConvertFrom-Json
foreach ($id in $pids.server, $pids.web) {
  $owned = Get-CimInstance Win32_Process -Filter "ProcessId = $id"
  $ours = $owned.ExecutablePath -eq "$repo\bin\clipmind-server.exe" -or "$($owned.CommandLine)".Contains("$repo\web\scripts\start-standalone.mjs")
  if ($ours) { taskkill /T /F /PID $id } else { "PID $id already exited or is not ours: nothing stopped" }
}
Get-NetTCPConnection -LocalPort $ports.api, $ports.web -State Listen -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force -LiteralPath $c
```

Then remove stale runs, skipping any whose ports still listen, and confirm the evidence survived:

```powershell
Get-ChildItem C:\temp\cliphub-try, C:\temp\clipmind-try -Directory -ErrorAction SilentlyContinue |
  Where-Object LastWriteTime -lt (Get-Date).AddDays(-2) |
  Where-Object {
    $old = Get-Content (Join-Path $_.FullName 'ports.json') -ErrorAction SilentlyContinue | ConvertFrom-Json
    -not ($old -and (Get-NetTCPConnection -LocalPort @($old.PSObject.Properties.Value) -State Listen -ErrorAction SilentlyContinue))
  } |
  Remove-Item -Recurse -Force
Test-Path $e
```

`taskkill /T` stops Electron together with the orchestrator and the web server it spawned. The ClipMind `Get-NetTCPConnection` line prints nothing when its ports are free. Each Studio profile keeps demos, captures and renders, and stale ones once filled 200 GB of `C:`.

The check is finished when the changed flow and one failure path behaved as expected in each edition that shows it, the PR says what you did and saw, and the profiles are deleted. It does not replace the automated checks in [Build and check](../../../AGENTS.md#build-and-check): before a PR they are green too, with the full Go suite (`-timeout 30m`, not `-short`).
