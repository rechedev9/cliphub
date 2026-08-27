# ClipHub Studio Desktop Guide

A Windows desktop wrapper around Local Studio: one app that boots the Go
orchestrator and the Next.js web UI (in local mode) and shows the flow in a
native window, so an end user never touches Node, a terminal, or a browser.

It bundles the same pieces `scripts/local-studio.ps1` runs:

- `zv-orchestrator.exe` - spawned directly (not via `zv serve`), so quitting
  the app kills the real server instead of leaving an orphaned grandchild
  holding the port and the SQLite job db. Runs with `ZV_DATABASE_URL=sqlite`
  (job state persists in `<userData>/data/jobs.db` across restarts) and
  `ZV_DATA_DIR=<userData>/data`; HLAE/CS2/FFmpeg are auto-detected, or use the
  tools provisioned on first boot below.
- `zv-recorder.exe` and `zv-editor.exe` - the required capture and render
  workers. Studio explicitly pins the bundled recorder so a stale inherited
  `ZV_RECORDER_PATH` cannot select an incompatible developer binary; the editor
  is auto-detected beside the orchestrator.
- The Next.js standalone server - started with Electron's own Node (no separate
  Node runtime shipped), in local mode so the UI proxies the whole pipeline to
  the orchestrator.

Both processes bind loopback (`ZV_HTTP_ADDR=127.0.0.1:<port>`) on ports chosen
once per install and persisted in `<userData>/ports.json`; the web port in
particular must stay stable across launches because the reel library lives in
the browser's `localStorage`, which is keyed by origin (`host:port`).
That file holds ports only; no secret is ever written to disk.

The installer bundles the official HLAE archive pinned by `src/hlae-tool.json`.
On first boot the app installs it alongside FFmpeg and yt-dlp into `<userData>/tools`, verifies every pinned SHA-256 digest, and provisions the music catalog.
Each tool also has a trusted extracted-tree SHA-256 embedded in the signed app.
Studio rehashes every cached file before reuse and compares the canonical tree
against that code-pinned value; the writable cache manifest is metadata, not a
root of trust. Markerless, forged, and archive-only legacy caches are migrated
only by reinstalling from the pinned, verified source. The FFmpeg source is an
immutable ClipHub release asset copied from a checksum-verified BtbN build,
not a rotating `latest` autobuild URL.
HLAE is available offline; the remaining downloads are best-effort and retry on the next launch.
After the current HLAE package is verified, Studio removes older versioned HLAE caches.
The packaged HLAE version is intentionally fixed by the manifest so every desktop build is reproducible.
The window lands on `/onboarding` (Inicio), the first-run door into demo, stream, and player entry.

Capture still needs CS2 installed on the machine (Windows + GPU); Studio installs
HLAE automatically. Job data (demos, artifacts) is written under the per-user
app data dir, not Program Files.

Finished Library reels include a manual publication assistant. Studio generates
Madrid-time guidance and factual metadata alternatives, lets the user copy the
title, description, and tags, downloads the MP4, and opens
`https://studio.youtube.com/` in the system browser. The user completes YouTube's
official **CREAR -> Subir vídeos** flow there, including channel, audience,
visibility, and scheduling choices. No Google credentials are required by the
installer. Optional public trend hints remain available when
`FIRECRAWL_API_KEY` is inherited by the desktop process.

Clipboard reads and Chromium web permissions remain denied. Copy buttons use a
text-only preload IPC capped at 512 KiB; preload requires transient user
activation, and the main process authenticates the active Studio top frame and
exact loopback origin before writing.

## No embedded assistant

Studio ships no assistant surface.
There is no agent rail, no chat, no embedded `codex app-server` connection, and no typed operation gateway; the preload bridge exposes `cliphubSettings.getAppInfo`, the clipboard write, and `cliphubUpdate` for GitHub Release checks.
The pipeline is driven through the interface itself and, for scripted work, the `zv` CLI in the repository build.
No publish text is model-generated: the render writes each pack's title, caption, and hashtags deterministically from demo facts, and the publication assistant above offers factual, reel-derived metadata alternatives.

## Credentials

Studio ships and runs without a model-provider credential.
Stream clips have no burned-in subtitle or killfeed pipeline, so no speech-to-text or vision key is ever read or stored.
`/settings` only reports the installed app, Electron, and Chromium versions through the narrow preload bridge; it stores nothing.

An operator's own `XAI_API_KEY` can still reach the Electron process by ordinary environment inheritance, and Studio refuses to pass it on.
The main process deletes the name from `process.env` at startup, before it spawns anything, so no child inherits it: not the bundled Next.js server, not `zv-orchestrator.exe`, and not the PowerShell that expands a runtime-tool archive.
The Next.js server is additionally launched with that name explicitly removed from its environment, and `zv-orchestrator.exe` additionally unsets it for itself and for every media subprocess it spawns.
Studio never reads the value it removes.

Packaging still strips `XAI_API_KEY` from the build, web, and electron-builder environments, and the installer manifest contains no credential resource.
That scrub is defence in depth against an operator's unrelated key leaking into a build, not a feature: nothing in ClipHub reads the value.

The optional Steam login (`ZV_STEAM_USERNAME`, `ZV_STEAM_PASSWORD`, `ZV_STEAM_GUARD`) is the only credential Studio forwards from the host environment, and only because nothing else can supply a Game Coordinator session.
`createOrchestratorEnvironment` deliberately builds a closed environment instead of spreading `process.env`, so those three are copied across explicitly by `steamEnvironment()` (`desktop/src/steam-environment.ts`).
Without that hop a packaged Studio would tell the user to set three variables and then ignore all three, because the orchestrator would never see them.
Only non-empty values are forwarded, so an unset variable stays absent rather than arriving as an empty string that reads as configured.
The authentication code, SteamID and Web API key are not env vars: Ajustes writes them to `<userData>/data/steam/account.json` through the orchestrator. A password typed at first download stays in orchestrator memory and is never written to disk.

## Build the installer (on Windows)

Prerequisites: **Go 1.26.6**, Node.js + pnpm, and the web deps installed.
`scripts/build.ps1` (and therefore `pnpm --dir desktop run dist`) bootstraps the
official Windows Go 1.26.6 zip into `%LOCALAPPDATA%\ClipHub\toolchain` when PATH
still has an older patch, verifies the SHA-256, and sets `GOTOOLCHAIN=local`.
Pin and checksums live in `scripts/go-windows.json`:

```text
https://go.dev/dl/go1.26.6.windows-amd64.zip
SHA-256: 5b6c5b556525810463b5c897b50dc7a82d6a3dc0bfaf55d990a7e9f31d6b2318
https://go.dev/dl/go1.26.6.windows-amd64.msi
SHA-256: 7c1390d3ab814753c3176bc0e0648ff70d3c2b4c3b22cced9c347f40dc920168
```

```powershell
# From the repo root:
pnpm --dir web install
pnpm --dir desktop install
pnpm --dir desktop run dist
```

`pnpm run dist` first runs the root `scripts/build.ps1 -RequireFaceitEmbed` so
every Go runtime is rebuilt from the current source and `zv-orchestrator.exe`
embeds `FACEIT_API_KEY` (User or Process env). Dist fails if that key is
missing or if the compiled orchestrator does not contain it. It then
runs `scripts/assemble.mjs` (builds the web in local mode and
stages `zv-orchestrator.exe`, `zv-editor.exe`, `zv-recorder.exe`, and the
standalone server into `build-resources/`), then `electron-builder` produces the
installer under `dist-installer/` (`ClipHub Studio Setup <version>.exe`,
where `<version>` is the `version` field in `desktop/package.json`). The
distribution command verifies the packaged HLAE archive, installer, blockmap,
and checksums before returning success. The FACEIT key is inside the
orchestrator binary, not a file in the installer.
The app icon lives at `build/icon.ico`, which electron-builder picks up
automatically;
`assemble.mjs` fails fast if it's missing. `zv-orchestrator.exe`,
`zv-editor.exe`, and `zv-recorder.exe` are required at assemble time so the
packaged app can parse, capture, and render reels. The developer `zv.exe` CLI
stays available in the repository build but is not shipped in the desktop
installer.

The build has one distribution target, `pnpm run dist`. It rejects unsupported
arguments, rebuilds the Go runtime binaries before staging, removes
`XAI_API_KEY` from every child build environment, forwards only `FACEIT_API_KEY`
into the Go rebuild so Jugadores works on a machine that never set the env var,
and cannot stage or declare a credential resource file. Never publish from an existing `bin/` or from a manually
assembled resource tree. The installed app needs no credential of its own.

The distribution command also creates `dist-installer/SHA256SUMS.txt` for the
installer and its blockmap, then verifies both before returning success.
`pnpm run verify:dist-integrity` repeats checksum verification in a fresh Node
process. Publish all three files together in GitHub Releases.

The installer is **intentionally unsigned** (project policy: never Authenticode /
`signtool` / cert-based signing of the app or NSIS package). Integrity is the
GitHub Release asset plus `SHA256SUMS.txt`. Windows SmartScreen may show an
"unknown publisher" prompt on first run — choose "More info" then "Run anyway".
Do not add code signing as a release step or treat SmartScreen as a blocker.

## Cut a release without a Windows PC

`.github/workflows/desktop-release.yml` is the only hosted release pipeline. It
runs the same `pnpm --dir desktop run dist` on `windows-latest` and publishes
`ClipHub.Studio.Setup.<version>.exe`, the blockmap, and `SHA256SUMS.txt` as a
stable GitHub Release (`releases/latest`). Actualizar reads that endpoint; a
landing/Vercel deploy is not required.

1. Set `desktop/package.json` `version` to the number the updater should see.
2. Land that commit on `main`.
3. Run **Desktop release** (`workflow_dispatch`) or push tag `v<version>`.
   The job seeds gitignored Suno mp3s from the last published installer, then
   runs `pnpm --dir desktop run dist`. Repo secret `FACEIT_API_KEY` is required
   so `-RequireFaceitEmbed` can stamp the orchestrator.
4. Confirm `https://github.com/rechedev9/cliphub/releases/latest` is that tag
   and lists the three assets.

`GITHUB_TOKEN` needs `contents: write` to create the release and upload assets.
A 403 on upload means that permission is missing under repo Settings → Actions
→ General → Workflow permissions. Do not add a second workflow.

## Run without packaging (dev)

```powershell
# From the repo root, once: build the Go binaries and the standalone bundle.
.\scripts\build.ps1
cd desktop; pnpm install
pnpm run assemble        # builds the web + stages build-resources/

# In dev, src/main.ts resolves every bundled resource (zv-orchestrator.exe,
# zv-editor.exe, zv-recorder.exe, the web server) from .\build-resources, the
# same layout `pnpm run assemble` stages for packaging. Launch the Electron shell:
pnpm start
```

The Electron suites use unique disposable `userData` profiles and one shared
420-second cold-boot budget. They may copy only an explicitly verified runtime
tool fixture into a profile, so concurrent suites never share SQLite, cookies,
ports, window state, or a writable tool cache.

```powershell
pnpm run build
pnpm run assemble
pnpm run test:e2e:ui
```

## Measure desktop efficiency

After Studio is open, capture a 1 Hz process-tree sample of the Electron tree
and its owned children. Do not launch HLAE, CS2, capture, or a long
FFmpeg/render job for this measurement. The sampler does not persist command
lines or environment data.

Named scenarios: `foreground-idle`, `background-idle`, `stream-static`,
`stream-playback`.

```powershell
.\scripts\measure-desktop-efficiency.ps1 -RootPid <electron-main-pid> -Scenario foreground-idle
.\scripts\measure-desktop-efficiency.ps1 -RootPid <electron-main-pid> -Scenario background-idle
.\scripts\measure-desktop-efficiency.ps1 -RootPid <electron-main-pid> -Scenario stream-static
.\scripts\measure-desktop-efficiency.ps1 -RootPid <electron-main-pid> -Scenario stream-playback
```

Each run writes schema-versioned JSON under `desktop/e2e/artifacts/` with
`schema_version`, the requested `duration_seconds`, summary percentiles/peaks
(`cpu_p95_percent`, `working_set_peak_bytes`, `private_bytes_peak`, plus
informational GPU fields), and a top-level `roles` breakdown.

Iterate only on raw-process wins, same scenario and duration, same machine
assumptions:

1. Measure and persist a baseline (`-OutputPath desktop\e2e\artifacts\efficiency-<scenario>-baseline.json`).
2. Change code or config. Do not recapture or rerender.
3. Remeasure the same scenario and `-Seconds` onto a candidate path.
4. Compare with the shipped rule:

```powershell
node desktop\scripts\compare-efficiency.mjs --baseline desktop\e2e\artifacts\efficiency-foreground-idle-baseline.json --candidate desktop\e2e\artifacts\efficiency-foreground-idle-candidate.json
```

Accept only when `accept` is true (`verdict=improve`): every comparable
summary (`cpu_p95_percent`, `working_set_peak_bytes`, `private_bytes_peak`)
is no worse than the baseline and at least one is strictly better. A mixed
result is a regress. Different scenario, duration, or schema is
incomparable. `gpu_p95_percent` may be 0 and never decides accept/reject.

The Grok workflow `.grok/workflows/desktop-efficiency.rhai` orchestrates the
same loop (`measure` → persist baseline → change → remasure → compare).

## How it works

`src/main.ts` (Electron main process, compiled to `dist/main.js`):

1. Reads or picks two per-install-stable loopback ports (`orchestrator`,
   `web`) and creates two distinct 32-byte per-boot secrets.
   Neither is persisted: `<userData>/ports.json` holds ports only.
   The mutation token is shared directly between the orchestrator, Next server,
   and trusted main process, while the proxy capability reaches only Next and an
   HttpOnly, SameSite=Strict loopback cookie seeded before the first app
   navigation.
2. Kicks off music catalog provisioning in the background, and awaits
   provisioning of bundled HLAE plus FFmpeg/yt-dlp into `<userData>/tools`
   (first boot only; later boots return the cached installs instantly).
3. Spawns `zv-orchestrator.exe` directly - without a `zv.exe serve`
   intermediary - so quitting the app reliably kills the real server (`ZV_DATABASE_URL=sqlite`,
   `ZV_DATA_DIR=<userData>/data`, `ZV_HTTP_ADDR=127.0.0.1:<orchPort>`, the
   ephemeral `ZV_MUTATION_TOKEN`, the bundled `ZV_RECORDER_PATH`, plus any
   provisioned tool paths).
4. Spawns the Next standalone `server.js` via `ELECTRON_RUN_AS_NODE`
   (`ORCHESTRATOR_URL` pointing at the orchestrator, `PORT=<webPort>`).
5. Waits for `/healthz` and the web root.
6. Loads `/onboarding` in the window.
7. Kills the orchestrator and web children on quit. Packaged builds acquire
   Electron's OS-backed single-instance lock under canonical `appData` before
   restoring any explicit profile, so changing `--user-data-dir` cannot bypass
   it; dev/E2E keeps profile-scoped isolation.

## App updates

Packaged Windows builds check [GitHub Releases](https://github.com/rechedev9/cliphub/releases)
for a newer `ClipHub.Studio.Setup.<version>.exe` after the window loads, and
again every six hours. The command strip shows an update control only when a
newer stable tag exists, a download is in progress, or a verified installer is
ready. Browser and unpackaged `pnpm start` sessions never show it.

The download is the same NSIS artifact the release publishes. Studio fetches
`SHA256SUMS.txt` from that tag, writes the installer under `<userData>/updates`,
and refuses to run it unless the SHA-256 matches. Applying the update spawns
the installer with `/S --updated --force-run` and quits so NSIS can replace the
running files and relaunch Studio. Assisted silent installs skip the finish-page
Run checkbox; `--force-run` is the electron-builder switch that starts the app
after the replace. `build/installer.nsh` also relaunches on silent `--updated`
when the running app is too old to pass `--force-run`. Integrity remains the
GitHub Release plus `SHA256SUMS.txt`; the updater does not code-sign.
