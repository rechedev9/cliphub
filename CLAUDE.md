# ClipHub Agent Instructions

This file is the agent contract for any AI coding harness working this repo (Claude Code, omp, or otherwise). Skills live in `.claude/skills/`; the harness guide and `zv` command catalog live in `.claude/GUIDE.md`.

`AGENTS.md` is a tracked symlink to this file: edit `CLAUDE.md` only, never replace the symlink with a regular file. On Windows a clone with `core.symlinks=false` materializes it as a 9-byte file containing `CLAUDE.md` while `git status` stays clean, which silently fails `zv check`. Repair: `git config core.symlinks true`, delete the file, `git checkout -- AGENTS.md` (needs Developer Mode or an elevated shell).

`README.md` is the public product entrypoint. Operational detail goes in purpose-specific files (`GUIDE.md`, `RUNBOOK.md`), never in nested `README.md` files.

## What ClipHub Is

Windows-local, deterministic CS2 demo/stream-to-video pipeline, mostly Go. The `.dem` is the source of truth for player, camera, tick ranges, kills, and utility; never infer a recording decision from rendered video.

```text
.dem  -> parse/score -> kill plan -> HLAE/CS2 capture -> FFmpeg/Lua render -> publish pack   (9:16 Shorts)
.dem  -> full round plan -> HLAE/CS2 capture -> overlays + concat -> recap                    (16:9 Full Demo)
VOD   -> persisted edit plan -> render -> publish pack                                        (stream clips)
```

Studio ships no assistant surface: it is a GUI over the same pipeline, and no publish text is model-generated. Do not resurrect the retired external MCP server; drive the product through Studio or the `zv` CLI. Current Studio (as of 2026-09-03): 2.4.51.

## Repo Map

| Path | What | Owner doc |
|---|---|---|
| `cmd/` | 12 `package main` binaries → `bin/zv*`. Thin flags + `os.Exit`. Known leaks: recorder launch, orchestrator SQLite/queue, demo-players parse, analysis-viewer. Do not add more. | `cmd/AGENTS.md` |
| `cmd/zv/` | Unified CLI dispatcher and catalog. `zv check` enforces docs/skills/workflows against the command contract. | `cmd/zv/AGENTS.md` |
| `internal/` | 51 flat Go packages, one directory = one package. Durable plans (`killplan`, `moments`, `streamclips.EditPlan`, `tacticalplan`, `timelineplan`) are the contracts later stages honor. | `internal/AGENTS.md` |
| `effects/` | Sandboxed `gopher-lua` effect scripts; no filesystem or process access. | - |
| `web/` | Next.js 16 / React 19 local Studio UI. | `web/CLAUDE.md`, `~/.grok/design.md`, `frontend-design` skill |
| `desktop/` | Electron 43 wrapper packaging `web/` + Go binaries. No React in `desktop/src`. | `desktop/GUIDE.md` |
| `landing/` | Only hosted app (Next 15). Build only; no lint/test scripts. | - |
| `docs/` | `AI_AGENT_ARCHITECTURE.md` (agent contracts), `CAPTURE_LAB.md` (L1-L5 verification), `RENDER_QUALITY.md`, `TACTICAL_ANALYSIS.md`, `TELEMETRY_RUNBOOK.md`, `cli-operator-workflow.md`. | - |
| `scripts/` | `build.ps1`, `local-studio.ps1`, `go-gate.sh`, `capture-lab.ps1`, `capturelab/` (HLAE-free simulator), `hlae/` (experiments, not product capture). | - |
| `testdata/` | Fixtures and goldens. Real `.dem` and `*.expected.json` stay local; `*.rules.json` may be committed. | `testdata/GUIDE.md` |
| `overlays/hyperframes/` | Probe only; not the FFmpeg/Lua pipeline. | - |
| `data/`, `bin/`, run output | Artifacts, not source. | - |

Package one-liners and anti-patterns are in `internal/AGENTS.md`; read it before touching a package you do not already know.

## Daily Loop

Bare `bash` on this machine is a broken WSL shim; run shell scripts through Git Bash.

```powershell
.\scripts\build.ps1                       # rebuild bin\zv*.exe (required when zv is missing or stale)
.\scripts\local-studio.ps1                # orchestrator + Next dev Studio
go test ./internal/parser -run TestFoo -count=1
go test ./... -count=1 -timeout 3m
& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --no-format   # test + vet + zv check (dirty worktree safe)
& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --race        # add for shared-state / concurrency changes
& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --security    # add for fs, subprocess, auth, dependency changes
```

`go-gate.sh` formats changed Go files unless `--no-format`; `--build` adds `go build ./cmd/...`; `--no-staticcheck` skips the optional linter. Toolchain sources of truth: `go.mod` (Go 1.26.6), `packageManager` (pnpm 11.22.0), Node 24. Do not add dependencies or run `go mod tidy` without explicit approval.

JavaScript packages have independent lockfiles: always `pnpm --dir web|desktop|landing`. Lint is oxlint, tests are `node:test` on colocated `*.test.ts` / `*.test.mjs`, `web/proxy.ts` is the Next 16 request guard, Playwright E2E lives in `web/e2e/` and runs only on explicit `pnpm --dir web run test:e2e`. Run these when the package is affected, in this order:

```powershell
pnpm --dir web run lint
pnpm --dir web run typecheck
pnpm --dir web run test:unit
pnpm --dir web run build
pnpm --dir desktop run lint
pnpm --dir desktop run typecheck
pnpm --dir desktop run test:unit
pnpm --dir desktop run build
pnpm --dir landing run build
```

Electron UI E2E (`pnpm --dir desktop run assemble` then `test:e2e:ui`) is manual and only for flows that need it.

## CLI-first

The `zv` CLI is the primary interface for parsing, capture, render, QA, and publishing; Studio is not a prerequisite. `flows show` and `workflows show` are the executable command contract; never guess flags from prose. Validate the exact argv, keep `--dry-run --format json` until real media work is approved, then preserve the approved argv when executing.

```powershell
.\bin\zv.exe capabilities --format json
.\bin\zv.exe verify features --format json
.\bin\zv.exe verify http --format json
.\bin\zv.exe verify gates --run --dry-run --format json
.\bin\zv.exe flows show demo --format json
.\bin\zv.exe flows show stream --format json
.\bin\zv.exe workflows list --format json
.\bin\zv.exe workflows show short --format json
.\bin\zv.exe workflows validate short --format json -- match.dem --prompt "all kills 76561198000000000" --dry-run --format json
.\bin\zv.exe workflows run short -- match.dem --prompt "all kills 76561198000000000" --dry-run --format json
.\bin\zv.exe presets --format json
.\bin\zv.exe skills list --format json
```

- Demo, when player/plays/order still need review: `demo players -> demo parse -> demo moments -> demo select -> record -> shorts render`. Use `short` only when target and selection policy are already complete.
- Stream: `stream fetch -> stream variants -> stream plan -> human review -> stream render`; skip fetch for a local MP4. A stream dry-run does not create its `--out`; persist each approved plan before the dependent stage. The persisted edit plan is canonical for ranges, order, crop, audio, fades, text, and `music.volume`; do not bolt ad hoc FFmpeg flags around it.
- Public presets are `viral-60-clean` (death notices + `viral-ultra-clean` effects) and `viral-aggressive-60` (same HUD, `viral-aggressive` grade, headshot chroma pulses). `clean-pov-60` / `full-hud-60` stay internal. HUD mode is a capture-time choice: changing it means recapture, not rerender.
- Default kill deliverable is one compiled vertical video per player/game, not one file per kill. Everything final (MP4, cover, caption, manifest, gallery) goes under the run's `shortslistosparasubir/`; point the user there.
- FACEIT Data API is approved for indexing players/matches/stats; the Download API is not. Demos come through FACEIT's authenticated room download or another user-authorized source. Credentials only in env/server-side secrets; never in indexes or logs. For "current"/"best performance" requests persist cutoff, sample size, match IDs, filters, and ranking formula; normalize rate stats per round; use stats to shortlist demos and parsed demo evidence to select moments.

## Standing Law

- **Prove it works.** Verify the real path, not a proxy. CI, mocks, lint, "it compiles" are supplemental. For parser/capture/render that means real ClipHub Studio on Windows + HLAE/CS2, or name the gap and do not call the work done.
- **Verification lever.** `zv verify` is the Windows-first control CLI; close the loop with `zv verify doctor` and `zv verify prove` before calling a change done. Host of record is King's Windows Studio (live orchestrator from `%APPDATA%\cliphub-studio\ports.json`, detected `C:\HLAE-*\HLAE.exe`, running `cs2.exe`). Doctor passes only when all three are up; Linux and a Windows host with Studio down fail-close as `hlae_cs2_windows_studio`. Do not drive King's PC, HLAE, or CS2 without an explicit grant on that run. No bot court, no Playwright-in-CI, no second local bot farm.
- **Structural flows:** (1) Demo parser → 9:16 Shorts, (2) Full Demo → 16:9 recap. A PR touching a flow must prove that flow; unknown on a touched flow blocks merge.
- **Hosted CI** (PRs and `main`): `CI frontend` (web + desktop typecheck/lint/unit), `CI backend` (`go vet`, `go test ./...`, `zv check`; triggers on `CLAUDE.md`, `AGENTS.md`, `.claude/**` too), `CI infra` (actionlint + release contract + the `Go 1.26.6` pin in this file). Not HLAE/CS2 E2E, not Playwright. `main` is unprotected with no required checks.
- **PR body** (`.github/pull_request_template.md`): exactly four H2s - What Problem This Solves, Why This Change Was Made, User Impact, Evidence. Empty Evidence is a blocker.
- **Release:** unsigned Windows NSIS installer only, via `.github/workflows/desktop-release.yml` (`workflow_dispatch` or a `v*.*.*` tag matching `desktop/package.json`) to GitHub Releases in `rechedev9/cliphub`. `dist` must rebuild every Go executable in the same invocation before `assemble` stages `bin/`. Never code-sign (no Authenticode, no `signtool`, no cert purchase); integrity is the release asset plus `SHA256SUMS.txt`. Actualizar reads `releases/latest`; landing is not the updater.
- **Capture Lab.** For demo/capture/render changes read `docs/CAPTURE_LAB.md`, run the cheapest relevant level, escalate through `scripts/capture-lab.ps1 -Mode Full` when end-to-end is in scope. Report the highest completed level; L1-L4 simulation is never HLAE/CS2 certification, and `capture_mode: "fake"` evidence never crosses the production-real gate. The real Windows canary is a separate explicit approval.

## Approval And Media

- Before any non-dry-run capture or render, stop at the creative brief gate and ask only the unanswered choices: format, HUD/killfeed, kill effect, transition, counter, intro/outro, music, cover strategy. For streams also clip bounds/title, crop/framing, source-audio treatment.
- Approval must answer a shown brief; `go`, `hazlo`, `dale`, `ok` alone are not approval.
- Translate every approved choice into an explicit final command value, including negatives (`--kill-counter=false`, `--hook=false`, `--covers=false`); never rely on a preset default to keep an `off`. After rendering, inspect the effective config and generated effects/metadata; reject output that re-enables a disabled element or contradicts the selected kills, weapons, rounds, or narrative.
- A render with unresolved QA warnings is not final. Inspect each warning at its interval; remove frozen, post-death, or dead-air footage or document why it is intentional; rerun QA.
- Any trim, reorder, or duration change invalidates rhythm timing: regenerate the canonical rhythm plan and re-verify every kill against its beat/onset before rerendering.
- Thumbnail approval is a second gate for CLI/agent pack delivery once candidates exist: require a selected candidate or explicit delegation before calling the pack upload-ready; `--covers=false` removes the gate. Studio Library does not use it (MP4 download and PREPARAR PUBLICACIÓN enable as soon as the file exists). Before marking a CLI pack upload-ready, verify MP4, title, caption, hashtags, cover, cover timestamp, gallery, manifest paths, and artifact metadata all describe the same facts; after thumbnail selection replace the canonical cover and re-verify the gallery.
- Changing a stream plan invalidates its brief; settle it again before the next real render.
- Third-party music: persist source URL, creator, license, file SHA-256, and rhythm evidence under the run. Never claim CC0 without an authoritative source.

Capture hardware rules:

- Do not launch HLAE, CS2, a long FFmpeg render, or paid/cloud media work without an explicit request; prefer the CLI preflight.
- Host capture auto-detects the highest installed HLAE version under `C:\HLAE-*\HLAE.exe`; before a real run compare it with the latest official HLAE release. Never use `C:\HLAE\HLAE.exe` for ClipHub capture. Packaged Studio uses the SHA-256-pinned archive in `desktop/src/hlae-tool.json`; do not copy a version number into prose or silently swap the asset.
- CS2 launches through HLAE with `-windowed`; fullscreen and borderless are unsupported.
- Recording is enqueued with `MaxRetry(0)`; a `demo_incompatible:` failure is deterministic and must not be retried on the same CS2 build. A failed recording `--out` is never reused; fresh namespace.
- After final media is validated with no recapture/reparse pending, send used extracted `.dem` files to the Recycle Bin; keep the original archive unless asked.

## Domain Invariants

**Jobs.** `internal/httpapi` + `internal/workers` are the local API and inline queue; one dedicated capture lane because every capture contends for one `cs2.exe`. Workers skip completed durable artifacts on retry. Series jobs share a client-minted `series_id`; roster choice aggregates across maps, HLTV `-pN.dem` parts are one logical map. Pipeline failures record once through `internal/obs` with stable `stage`/`class` labels; the journal is authoritative.

**CheaterDetect** (`internal/anticheat`). One deterministic parser pass, no CS2/HLAE, no network. CLI `demo anticheat [--dossier]` and `demo anticheat calibrate` (read flags from `--help`); API `POST|GET /api/jobs/{id}/anticheat`, task `analyze:anticheat`, artifact `jobs/<id>/anticheat.json`. Side lane: never changes job status. Weights/bands in `score.go`; baseline is data in `baseline_default.json` measured over 15 pro maps and carries per-metric sample counts. Never edit sample counts by hand or zero them to reconcile text; recalibrate. Composite blends the strongest of the information/aim clusters with the mean so a single-kind cheat still flags. Output is an anomaly report, never a verdict of guilt; keep `limitations`, `insufficient_data`, and confidence gates. ClipHub prepares a dossier and links official channels; it never submits reports, automates submission, or helps mass-report one account.

**Share codes and Steam.** `CSGO-xxxxx-...` is bijective base-57; `internal/sharecode` decodes offline; `web/lib/sharecode.ts` checks shape only - never port the decode to TypeScript. Listing recent matches uses `ICSGOPlayers_730/GetNextMatchSharingCode` with SteamID64 + revocable auth code (`help.steampowered.com` `appid=730&issueid=128`) + that player's Web API key, stored in `<dataDir>/steam/account.json`. Downloading a `.dem` needs a logged-in CS2 Game Coordinator session: `internal/steamgc` owns the wire format (9147 out, 9139 back; demo URL is the `map` field of the last `roundstatsall`), `internal/steamresolve` owns decode/history/fetch, `internal/steamclient` is the go-steam session and must not be imported by `httpapi`.

- Never persist `ZV_STEAM_USERNAME` / `ZV_STEAM_PASSWORD` / `ZV_STEAM_GUARD`; the password is prompted, held in memory, or read from env, and never logged or echoed into an error.
- The account is the one the user plays on. Steam allows one CS2 session per account, so opening the GC kicks a live match: touch it only on explicit user action, keep it short, disconnect, never poll, never open at startup, and say so in the UI.
- Decoding works without Steam configured and returns `status: "decoded"`, not an error. Download is `POST /api/steam/import` and ends in the same `CreateJob` path as `/upload`.
- Both ids exceed 2^53 and cross HTTP as strings.
- `go-steam` and `demoinfocs-golang` both register protobuf extension 50000; `internal/allowproto` must stay the first internal import of the orchestrator.
- `internal/steamclient` has no test coverage (needs real credentials); treat it as unverified.

## Code Contracts

- Write boring, idiomatic Go.
- Do not introduce `util`, `common`, `helper`, `manager`, or vague service layers.
- Add useful context when returning errors.
- Every goroutine must have a clear owner and stop condition.
- Add or update behavior-level tests for fixes and behavior changes. Table-driven tests wherever the pattern fits; near-duplicate clones are a review finding.
- Parser-only and pure unit tests never launch HLAE/CS2 or long renders. Real-demo worker tests skip without `TEST_DEMO_PATH`.
- Do not add generated video/audio/image artifacts to git.
- Browser API access stays same-origin through server proxy routes; orchestrator URLs/tokens stay server-side; validate IDs before building upstream URLs; preserve `503 {code: "service_unavailable"}`. Electron renderer access stays behind preload/IPC.

## Working Style

- Before frontend work read `web/CLAUDE.md`; before visual work read `~/.grok/design.md`, load `frontend-design`, restyle onto `web/app/globals.css`. Before Electron/packaging/release work read `desktop/GUIDE.md`.
- Committing or pushing requires an explicit user request. There is no commit-time gate and a push to `main` lands immediately, so run focused tests plus affected package checks first, stage only in-scope paths, and never disturb unrelated work. Temporary worktrees are fine; never remove or repurpose one holding uncommitted work.
- Review findings use `BLOCKER`, `WARNING`, or `NIT` with file/path, problem, why it matters, and a practical fix; if clean, say `No blocking issues found.`
