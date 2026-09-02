---
name: verify-cliphub
description: "Prove ClipHub Studio and zv changes against the real user path before calling work done. Use when finishing a ClipHub change, verifying Studio routes (Inicio, Partidas, Shorts wait, Full Demo), or when compile/lint/CI is being treated as the feature."
---

# Verify ClipHub

Verification is infra. This skill is the Windows-first control loop for agents that ship in this repo. Do not copy pstack. Do not stand up a Grok Bot court. Drive ClipHub with `zv verify` and the Studio feature map.

**Prove against the real artifact.** Loop until the path proves itself. Never treat compile, lint, unit tests, or hosted CI as the feature. Doctor and prove fail closed when the live Studio surface is missing.

## Pick a flow

`zv verify doctor --format json` decides what this host can prove.

- **Cheap contract (any host).** Feature map, skill phrases, hosted gates. `prove` for non-HLAE features is map-complete. When Studio is up it also GETs the catalog `probe_path` (read-only). That inspect is not a UI walk and not a capture Pass.
- **Studio UI walk (granted Windows Studio only).** Doctor `studio.web_url` + `prove` `drive.open_url`. Click rail labels. Optional chrome-devtools snapshot/click by accessible name against that loopback URL. Chrome is not the Electron shell.
- **Capture recertification (granted Windows Studio + HLAE + running cs2.exe only).** `prove --feature` on `demo-completa`, `shorts-9x16-wait`, or `full-demo-16x9-wait` with `--job-id`. Live overlay percent is `progress.percent`. This CLI does not screenshot Studio and does not add Playwright to CI.

On Cloud Linux there is no CS2, no HLAE, and no Windows Studio. Do not fake a Studio walk. Do not drive King's Windows PC, HLAE, or CS2 unless that run was explicitly granted.

## Launch

```powershell
.\scripts\local-studio.ps1
```

Packaged Studio writes `%APPDATA%\cliphub-studio\ports.json` (`web`, the single Go UI/API port) and `%APPDATA%\cliphub-studio\data\jobs.db`. Doctor also accepts the former two-port document during upgrades. The CLI default `http://127.0.0.1:8080` is not packaged Studio.

If ports.json is already live, drive that instance. Do not launch a second Studio against the same userData.

For CLI-only cheap proof, no server is required:

```text
./bin/zv verify doctor --format json
```

Ready means doctor JSON parsed, the skill and feature map are present, and every closed gap is named. Linux doctor fail-closes (`hlae_cs2_windows_studio`). It does not mean Full Demo or a live overlay passed.

If `bin/zv` is missing: `go run ./cmd/zv verify doctor --format json` or `.\scripts\build.ps1`.

Teardown: stop only the orchestrator or Studio instance you started. Never kill `cs2.exe` by image name.

## Doctor

Run this first whenever anything looks off, and before calling a change done:

```text
./bin/zv verify doctor --format json
./bin/zv capabilities --format json
```

Doctor inspects:

- `ports.json` under `%APPDATA%\cliphub-studio`
- `data\jobs.db`
- GET `/healthz` on the orchestrator port (`{"service":"cliphub","status":"ok"}`)
- HLAE at `C:\HLAE-*\HLAE.exe` or packaged `tools\hlae\*\HLAE.exe` — never `C:\HLAE\HLAE.exe`
- a running `cs2.exe` process (never faked from a file path)

Windows doctor Passes only when Studio + HLAE + CS2 are actually up. A Windows host with Studio down still fail-closes (`studio_orchestrator_down` or `studio_ports_missing`). `--dry-run` does not GET `/healthz`, does not write `jobs.db`, and does not enqueue capture.

`host.capture_recertification` is `unavailable` or `studio_live`. `studio_live` still does not mean Full Demo Pass.

## Command surface

`--help` is canonical. JSON is the agent contract (`--format json` in both text and json).

- **Health:** `doctor`, `http`, `gates --run --dry-run`
- **Map:** `features`, `features --feature <id>`
- **Prove:** `prove --feature <id>`, `prove --feature <id> --job-id <uuid>`
- **Safety:** every mutating path has `--dry-run` (no HTTP, no jobs.db writes, no capture enqueue)

```text
./bin/zv verify features --format json
./bin/zv verify http --format json
./bin/zv verify gates --run --dry-run --format json
./bin/zv verify prove --feature inicio --format json
./bin/zv verify prove --feature shorts-9x16-wait --job-id <uuid> --format json
```

`prove` JSON includes `drive` (`route`, `nav_label`, `open_url`, `probe_url`) and, when Studio is up, `live` (read-only GET of `probe_path` on Studio `web_url`, the same-origin proxy the UI uses). Direct orchestrator `/api/*` GETs 401 without the session token. `user_path` becomes `inspected` on a successful live GET, never `pass`. `--dry-run` prints the planned GET and does not HTTP.

## Proof bar

A proof that hits one convenient entry point is incomplete when the map lists others.

- Exercise the real user path (sidebar label, not an internal setter).
- Capture the action and the resulting state, not just a screenshot of the final frame.
- Verify side effects (job status, artifacts, Library cards) alongside what is visible.
- When the safe path is `--dry-run`, observe that it skipped live capture/render (no CS2, no new MP4, no jobs.db writes).
- Mocks count only behind a production boundary. They do not count when they skip the renderer, orchestrator, persistence, or the path under test.
- An unreachable path is `verified-unreachable` only with the concrete prerequisite and the route attempted.

For waits: live `%` and `current/total` come from the orchestrator progress object. `recordAdmitted` keeps a still-failed job in capture instead of latching FALLO. PR #120 is still draft — do not merge live-overlay work until a Windows Studio overlay walk.

## Driving conventions

- Read [references/features/INDEX.md](references/features/INDEX.md) first, then the matching feature file.
- Prefer rail labels from `web/lib/nav.ts`, visible headings, and ARIA names over CSS selectors. Rail accessible names are uppercase (`INICIO`, `EDITOR`); the `00`–`11` prefixes are source order, not the clickable name.
- `drive.open_url` is Studio's loopback Go-served SPA origin plus the feature route. A Chrome tab against that URL is the same UI, not Electron chrome (Ajustes versión/telemetry need the desktop bridge).
- Wait on observable copy (`EMPIEZA AQUÍ`, `BIBLIOTECA`, `ANALIZAR DEMO`), not fixed sleeps.
- Same-origin only: UI talks through `/api/*` proxies. Orchestrator down is `503 {code:"service_unavailable"}`.
- On Cloud Linux, cover Windows detection paths with table-driven fixtures (missing `ports.json` / missing HLAE / CS2 not running / all present). Never fake a live `cs2.exe`.

## Feature map

Behavior inventory: [references/features/INDEX.md](references/features/INDEX.md). Each file uses the same five H2s: `Sub-features`, `How to get to it (user POV)`, `What done looks like`, `Driving it with zv verify`, `Gotchas`.

For a broad regression, walk INDEX top to bottom, then [references/features/journeys.md](references/features/journeys.md).

## Evidence

Proof lives with the run, not in a vanished temp dir:

- `zv verify doctor --format json` (schema_version, studio, hlae, cs2, gaps)
- `zv verify prove --feature <id> --format json` (`drive.open_url`, `live` GET when Studio is up, and `--job-id` for waits)
- focused `go test` / `pnpm --dir web run test:unit` for the cheap contract
- for capture/recap: real ClipHub Studio on King's Windows host + HLAE/CS2, or the named gap

## Cleanup

Stop processes you started. Do not kill by process name. Cleanup never deletes proof artifacts.

## Helpers

```text
./bin/zv verify doctor --format json
./bin/zv verify features --feature shorts-9x16-wait --format json
./bin/zv verify prove --feature full-demo-16x9-wait --format json
./bin/zv verify prove --feature inicio --dry-run --format json
./bin/zv verify prove --feature shorts-9x16-wait --job-id <uuid> --dry-run --format json
```

## Honest gap

HLAE/CS2 / Windows Studio is the named gap on Cloud Linux (`hlae_cs2_windows_studio`). This lever will not convert it into a Pass. Structural flows remain (1) Demo parser → 9:16 Shorts and (2) Full Demo → 16:9 recap. Unknown on a touched flow is a merge block. Say the gap out loud and do not call the work done.
