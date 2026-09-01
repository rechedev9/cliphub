---
name: verify-cliphub
description: "Prove ClipHub Studio and zv changes against the real user path before calling work done. Use when finishing a ClipHub change, verifying Studio routes (Inicio, Partidas, Shorts wait, Full Demo), or when compile/lint/CI is being treated as the feature."
---

# Verify ClipHub

Verification is infra. This skill is the control loop for agents that ship in this repo. Do not copy pstack. Do not stand up a Grok Bot court. Drive ClipHub with `zv verify` and the Studio feature map.

**Prove against the real artifact.** Loop until the path proves itself. Never treat compile, lint, unit tests, or hosted CI as the feature.

## Launch

ClipHub Studio is a Windows-local Electron shell over `zv-orchestrator` + the Next.js UI.

```powershell
.\scripts\local-studio.ps1
```

The orchestrator answers `http://127.0.0.1:8080/healthz` with `{"service":"cliphub","status":"ok"}`. Studio is same-origin through `web/app/api/*`. A 503 `{code:"service_unavailable"}` means the local service is down.

On this Cloud Linux VM there is no CS2, no HLAE, and no Windows Studio. Do not launch HLAE/CS2 from here. Do not fake a Studio walk.

For CLI-only cheap proof, no server is required: build or `go run` `zv` and inspect.

```text
./bin/zv verify doctor --format json
```

Ready means doctor JSON parsed, the skill and feature map are present, and every closed gap is named. It does not mean Full Demo or a live overlay passed.

Teardown: stop only the orchestrator or Studio instance you started. Never kill `cs2.exe` by image name. Keep doctor JSON and other proof artifacts.

## Doctor

Run this first whenever anything looks off, and before calling a change done:

```text
./bin/zv verify doctor --format json
./bin/zv capabilities --format json
```

Doctor inspects what **can** run without HLAE/CS2: the skill, the feature map, hosted gate catalog, and `/healthz` if an orchestrator is on loopback.

It must fail closed and names `hlae_cs2_windows_studio` when capture or Full Demo 16:9 recap cannot be recertified on this machine. Cloud Linux cannot launch CS2. Hosted CI green is not HLAE/CS2 proof. A `Pass` on compile is not a Pass on Full Demo or live overlay walks.

`host.capture_recertification` is `unavailable` or `tools_present`. `tools_present` still does not certify a Studio overlay walk.

## Drive

Read the feature map first: [references/features/INDEX.md](references/features/INDEX.md). A proof that hits one convenient entry point is incomplete when the map lists others.

Cheap (Linux OK):

```text
./bin/zv verify features --format json
./bin/zv verify http --format json
./bin/zv verify gates --run --dry-run --format json
./bin/zv verify prove --feature inicio --format json
```

`prove --feature` on `demo-completa`, `shorts-9x16-wait`, or `full-demo-16x9-wait` must fail closed here.

User-path (Windows Studio only):

1. Open the mapped route the way a user would (sidebar label, not an internal setter).
2. Exercise the action (upload, parse, wait, ready card).
3. Capture the action and the resulting state, not just a screenshot of the final frame.
4. For waits: live `%` and `current/total` come from the orchestrator progress object. `recordAdmitted` keeps a still-failed job in capture instead of latching FALLO. PR #120 is still draft — do not merge live-overlay work until a Windows Studio overlay walk.

Do not add Playwright or HLAE to hosted CI.

## Evidence

Proof lives with the run, not in a vanished temp dir:

- `zv verify doctor --format json` (schema_version, gaps, features)
- `zv verify features --format json`
- focused `go test` / `pnpm --dir web run test:unit` for the cheap contract
- for capture/recap: real ClipHub Studio on Windows + HLAE/CS2, or the named gap

Standards:

- Exercise the real user path, not test-only endpoints.
- Capture the action and the resulting state.
- Verify side effects (job status, artifacts, Library cards) alongside what is visible.
- When the safe path is `--dry-run`, observe that it skipped live capture/render (no CS2, no new MP4).

## Cleanup

Stop processes you started. Do not kill by process name. Cleanup never deletes proof artifacts.

## Helpers

```text
./bin/zv verify doctor --format json
./bin/zv verify features --feature shorts-9x16-wait --format json
./bin/zv verify prove --feature full-demo-16x9-wait --format json
./bin/zv verify prove --feature inicio --dry-run --format json
```

If `bin/zv` is missing: `go run ./cmd/zv verify doctor --format json` or `.\scripts\build.ps1`.

## Honest gap

HLAE/CS2 / Windows Studio is the named gap. This lever will not convert it into a Pass. Structural flows remain (1) Demo parser → 9:16 Shorts and (2) Full Demo → 16:9 recap. Unknown on a touched flow is a merge block. Say the gap out loud and do not call the work done.
