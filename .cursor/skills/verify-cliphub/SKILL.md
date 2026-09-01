---
name: verify-cliphub
description: "Prove ClipHub Studio and zv changes against the real user path before calling work done. Use when finishing a ClipHub change, verifying Studio routes (Inicio, Partidas, Shorts wait, Full Demo), or when compile/lint/CI is being treated as the feature."
---

# Verify ClipHub

Verification is infra. This skill is the Windows-first control loop for agents that ship in this repo. Do not copy pstack. Do not stand up a Grok Bot court. Drive ClipHub with `zv verify` and the Studio feature map.

**Prove against the real artifact.** Loop until the path proves itself. Never treat compile, lint, unit tests, or hosted CI as the feature. Doctor and prove fail closed when the live Studio surface is missing.

## Launch

ClipHub Studio is a Windows-local Electron shell over `zv-orchestrator` + the Next.js UI.

```powershell
.\scripts\local-studio.ps1
```

Packaged Studio writes `%APPDATA%\cliphub-studio\ports.json` (`orchestrator`, `web`) and `%APPDATA%\cliphub-studio\data\jobs.db`. Doctor reads those. The CLI default `http://127.0.0.1:8080` is not packaged Studio.

On this Cloud Linux VM there is no CS2, no HLAE, and no Windows Studio. Do not launch HLAE/CS2 from here. Do not fake a Studio walk. Do not drive King's Windows PC, HLAE, or CS2 unless that run was explicitly granted. The verification host of record for capture and overlays remains King's Windows Studio; a later granted Windows loop runs this CLI. This run proves Linux fail-close and fixture tests only.

For CLI-only cheap proof, no server is required: build or `go run` `zv` and inspect.

```text
./bin/zv verify doctor --format json
```

Ready means doctor JSON parsed, the skill and feature map are present, and every closed gap is named. Linux doctor fail-closes (`hlae_cs2_windows_studio`). It does not mean Full Demo or a live overlay passed.

Teardown: stop only the orchestrator or Studio instance you started. Never kill `cs2.exe` by image name. Keep doctor JSON and other proof artifacts.

## Doctor

Run this first whenever anything looks off, and before calling a change done:

```text
./bin/zv verify doctor --format json
./bin/zv capabilities --format json
```

Doctor is a Windows-first control CLI shipped so a later granted Windows loop can inspect:

- `ports.json` under `%APPDATA%\cliphub-studio`
- `data\jobs.db`
- GET `/healthz` on the orchestrator port (`{"service":"cliphub","status":"ok"}`)
- HLAE at `C:\HLAE-*\HLAE.exe` or packaged `tools\hlae\*\HLAE.exe` — never `C:\HLAE\HLAE.exe`
- a running `cs2.exe` process (never faked from a file path)

Windows doctor Passes only when Studio + HLAE + CS2 are actually up. A Windows host with Studio down still fail-closes (`studio_orchestrator_down` or `studio_ports_missing`).

Linux doctor fail-closes and names `hlae_cs2_windows_studio`. Never Pass capture recertification on Cloud Linux. Hosted CI green is not HLAE/CS2 proof. A `Pass` on compile is not a Pass on Full Demo.

`host.capture_recertification` is `unavailable` or `studio_live`. `studio_live` still does not mean Full Demo Pass.

`--dry-run` does not GET `/healthz`, does not write `jobs.db`, and does not enqueue capture.

## Drive

Read the feature map first: [references/features/INDEX.md](references/features/INDEX.md). A proof that hits one convenient entry point is incomplete when the map lists others.

Cheap (Linux OK):

```text
./bin/zv verify features --format json
./bin/zv verify http --format json
./bin/zv verify gates --run --dry-run --format json
./bin/zv verify prove --feature inicio --format json
```

User-path (later granted Windows Studio only — do not drive King's PC from Cloud Linux):

```text
./bin/zv verify prove --feature shorts-9x16-wait --job-id <uuid> --format json
```

That GET `/api/jobs/{id}?view=status` returns `{status, failure_reason, progress?: {done, total, percent}}`. Overlay percent is `progress.percent`. `--dry-run` prints the planned GET and does not HTTP. On Cloud Linux, cover the Windows detection paths with table-driven fixtures (missing `ports.json` / missing HLAE / CS2 not running / all present). Never fake a live `cs2.exe`.

`prove --feature` on `demo-completa`, `shorts-9x16-wait`, or `full-demo-16x9-wait` must fail closed here on Cloud Linux.

User-path walk still matters:

1. Open the mapped route the way a user would (sidebar label, not an internal setter).
2. Exercise the action (upload, parse, wait, ready card).
3. Capture the action and the resulting state, not just a screenshot of the final frame.
4. For waits: live `%` and `current/total` come from the orchestrator progress object. `recordAdmitted` keeps a still-failed job in capture instead of latching FALLO. PR #120 is still draft — do not merge live-overlay work until a Windows Studio overlay walk.

This CLI does not screenshot Studio and does not add Playwright to hosted CI.

## Evidence

Proof lives with the run, not in a vanished temp dir:

- `zv verify doctor --format json` (schema_version, studio, hlae, cs2, gaps)
- `zv verify prove --feature <id> --job-id <uuid> --format json`
- focused `go test` / `pnpm --dir web run test:unit` for the cheap contract
- for capture/recap: real ClipHub Studio on King's Windows host + HLAE/CS2, or the named gap

Standards:

- Exercise the real user path, not test-only endpoints.
- Capture the action and the resulting state.
- Verify side effects (job status, artifacts, Library cards) alongside what is visible.
- When the safe path is `--dry-run`, observe that it skipped live capture/render (no CS2, no new MP4, no jobs.db writes).

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

If `bin/zv` is missing: `go run ./cmd/zv verify doctor --format json` or `.\scripts\build.ps1`.

## Honest gap

HLAE/CS2 / Windows Studio is the named gap on this Cloud Linux VM (`hlae_cs2_windows_studio`). This lever will not convert it into a Pass. Structural flows remain (1) Demo parser → 9:16 Shorts and (2) Full Demo → 16:9 recap. Unknown on a touched flow is a merge block. Say the gap out loud and do not call the work done.
