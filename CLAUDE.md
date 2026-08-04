# TickCut Agent Instructions

`AGENTS.md` is a tracked symbolic link to this file.
Edit `CLAUDE.md` only, and never replace the `AGENTS.md` symlink with a regular file.
The pre-commit hook rejects a broken link and commits made outside `main`.
The root `README.md` is the public product entrypoint (GitHub + onboarding). Prefer purpose-specific names such as `PRODUCT.md`, `GUIDE.md`, `RUNBOOK.md`, or `PROVENANCE.md` for operational detail.

## Product

TickCut is a Windows-local, deterministic CS2 demo/stream-to-video pipeline written primarily in Go.
The demo is the source of truth for player, camera, tick ranges, kills, and utility; never infer recording decisions from rendered video.

```text
.dem -> parse/score -> selected kill plan -> HLAE/CS2 capture -> FFmpeg/Lua render -> publish pack
stream video -> persisted edit plan -> render -> publish pack
```

- `cmd/` contains thin entrypoints; business logic belongs under `internal/`.
- `internal/parser`, `internal/killplan`, and `internal/moments` own the durable plan passed to every later demo stage.
- `internal/recording` owns HLAE/CS2 scripts and capture validation; `internal/editor`, `internal/renderplan`, and `internal/composition` own post-capture effects, variants, QA, and FFmpeg composition.
- `effects/` contains sandboxed `gopher-lua` scripts with no filesystem or process access.
- `internal/httpapi` and `internal/workers` implement the local API and jobs; the inline queue has a dedicated one-worker capture lane because all captures contend for one `cs2.exe`.
- Workers skip completed durable artifacts on retry, but recording is enqueued with `MaxRetry(0)`; a `demo_incompatible:` failure is deterministic and must not be retried against the same CS2 build.
- Series jobs share a client-minted `series_id`; roster choice is aggregated across maps, and HLTV `-pN.dem` parts are one logical map.
- `web/` is the Next.js 15/React 19 local UI, `desktop/` packages it with the Go services in Electron, and `landing/` is the only hosted application.
- `data/`, `bin/`, capture output, and generated media are artifacts, not source, unless a task explicitly targets fixtures or cleanup.

## Codex Desktop: CLI-first

Use the unified `zv` CLI for normal parsing, capture, render, QA, and publishing; Studio is not a prerequisite.
If `bin\zv.exe` is missing or stale, run `.\scripts\build.ps1` first.

```powershell
.\bin\zv.exe capabilities --format json
.\bin\zv.exe flows show demo --format json
.\bin\zv.exe flows show stream --format json
.\bin\zv.exe workflows list --format json
.\bin\zv.exe workflows show short --format json
.\bin\zv.exe workflows validate short --format json -- match.dem --prompt "all kills 76561198000000000" --dry-run --format json
.\bin\zv.exe workflows run short -- match.dem --prompt "all kills 76561198000000000" --dry-run --format json
```

- Treat `flows show` and `workflows show` as the executable command contract; do not guess flags from prose.
- Validate the exact argv first, retain `--dry-run --format json` until real media work is approved, and preserve the approved argv when executing.
- Use `demo players -> demo parse -> demo moments -> demo select -> record -> shorts render` when the player, plays, or order still need review; use `short` only when target and selection policy are complete.
- Use `stream variants -> stream plan -> human review -> stream render` for VODs.
- A stream dry-run does not create its `--out` artifact; persist each approved plan before invoking a dependent stage.
- The persisted stream edit plan is canonical for ranges, order, crop, audio, fades, text, and `music.volume`; do not bolt ad hoc FFmpeg flags around it.
- Discover task-specific guidance with `.\bin\zv.exe skills list --format json` rather than duplicating skill tutorials here.
- Do not resurrect the retired external MCP server; drive the product through the Studio interface or the `zv` CLI.
- Studio ships no assistant surface: it is a GUI over the same pipeline, and no publish text is model-generated — the render writes each pack's title, caption, and hashtags deterministically from demo facts, and the Library's publication assistant offers factual, reel-derived metadata alternatives.
- This project has approved FACEIT Data API access for player, match, and statistics indexing. The FACEIT Download API is not approved; obtain demo files through FACEIT's authenticated room/Watch download flow or another user-authorized manual source. Keep every FACEIT credential in environment or server-side secret storage, and never commit, print, or persist the key in indexes or logs.
- For "current" or "best performance" requests, persist the query cutoff, sample size, match IDs, filters, and ranking formula. Normalize rate statistics per round when match lengths differ. Use external statistics to shortlist demos, but use parsed demo evidence to select moments.

## CheaterDetect

`internal/anticheat` screens a demo for cheat-suspicion signals in one deterministic parser pass; it never launches CS2 or HLAE and never calls a network service.
The CLI entry points are `demo anticheat` (screen one demo, optionally rendering one player's dossier with `--dossier`) and `demo anticheat calibrate` (measure a new reference distribution); read their exact flags from `demo anticheat --help`, not from prose.
It is exposed over `POST|GET /api/jobs/{id}/anticheat`, backed by the `analyze:anticheat` task and the `jobs/<id>/anticheat.json` artifact.
The screening is a side lane: it never changes a demo job's status, so a demo can be screened and clipped independently and a failed screening never makes a healthy job look broken.

Metric definitions, weights, and verdict bands live in `internal/anticheat/score.go`; the shipped reference distribution is data in `internal/anticheat/baseline_default.json`.
The shipped baseline was measured over 15 top-level professional maps; the demos are local files that cannot be versioned, so the per-metric sample count is the evidence that travels with the numbers.
Do not reconcile a mismatch between the baseline and some other text by zeroing those counts: that turns a measurement into a claim it never made. Fix the other text, or recalibrate.
Never raise a metric's `samples` without a calibration run that actually produced it, and recalibrate with `demo anticheat calibrate` rather than editing numbers by hand; provenance and per-metric sample counts travel inside every report.
Calibration uses a median and a MAD-derived spread and counts each demo once by SHA-256, so neither an atypical match nor a re-uploaded one can bend the reference.
The composite blends the strongest of the information and aim clusters with the overall mean, because a plain mean across every metric cannot flag a single-kind cheat: a wall-only user maxes the information metrics and sits at the median on aim.

The output is an anomaly report, never a verdict of guilt, and every surface must keep saying so: the report carries its own `limitations`, and the score is a prompt to review the listed ticks by hand.
TickCut prepares a report dossier and links the official channels; it must never submit a report, automate a submission, or help produce several reports against one account.
Valve decides cheating bans from its own detection, not from report volume, and coordinated mass reporting is both ineffective and against the Steam Subscriber Agreement.
Do not add a feature that files reports on the user's behalf, and do not weaken the `insufficient_data` and confidence gates that stop a thin sample from producing a verdict.

## Approval And Media

- Before any non-dry-run capture or render, stop at the creative brief gate and ask only unanswered choices: format, HUD/killfeed, kill effect, transition, counter, intro/outro, music, and cover strategy.
- Approval must answer a shown brief; ambiguous words such as `go`, `hazlo`, `dale`, or `ok` are not approval by themselves.
- Translate every approved brief choice into an explicit final command value, including negative booleans such as `--kill-counter=false`, `--hook=false`, and `--covers=false`; never rely on a preset or flag default to preserve an approved `off` choice. After rendering, inspect the effective result configuration and generated effects/metadata, and reject any output that re-enables a disabled element or contradicts the selected kills, weapons, rounds, or narrative.
- A successful render is not final while QA has unresolved warnings. Inspect every warning at its exact interval; remove unintended frozen, post-death, or dead-air footage, or document why it is intentional, then rerun QA.
- Any trim, reorder, or duration change invalidates existing rhythm timing. Regenerate or update the canonical rhythm plan and verify every selected kill against its assigned beat or onset before rerendering.
- For streams, also settle clip bounds/title, crop/framing, and source-audio treatment.
- Thumbnail approval is a second gate after candidates exist; require a selected candidate or explicit delegation before calling the pack upload-ready.
- Before marking a pack upload-ready, verify that the canonical MP4, title, caption, hashtags, cover, cover timestamp, gallery, manifest paths, and artifact metadata describe the same facts and files. After thumbnail selection, replace the canonical cover and visually verify the gallery again.
- `--covers=false` removes the thumbnail gate.
- Changing a stream plan invalidates its creative brief; settle the brief again before the next non-dry-run render.

The editor registry in `internal/editor/preset.go` retains the capture-mode profiles, but the public unified `zv` preset catalog intentionally exposes only `viral-60-clean`; discover the supported CLI values with `.\bin\zv.exe presets --format json`.
`viral-60-clean` records death notices and uses `viral-ultra-clean` effects. `clean-pov-60` and `full-hud-60` remain internal capture profiles and are not selectable through the unified CLI without a future product decision.
HUD mode is a recording-stage choice, so changing it after capture requires recapture rather than a render-only change.
The default kill/highlight deliverable is one compiled vertical video per player/game containing all selected kills, not one upload-ready file per kill.
Put every final MP4, cover, caption, manifest, and review gallery under the run's `shortslistosparasubir/` directory, and point the user there when delivering media.
For third-party music, persist the source URL, creator, license, downloaded-file SHA-256, and rhythm-analysis evidence under the run. Never claim a track is CC0 or otherwise reusable without an authoritative source.

Do not launch HLAE, CS2, a long FFmpeg render, or paid/cloud media work without an explicit request; prefer the CLI preflight.
Host capture auto-detects the highest installed HLAE version under `C:\HLAE-*\HLAE.exe`; before a real run, compare it with the latest official HLAE release.
Never use `C:\HLAE\HLAE.exe` for TickCut capture.
Packaged Studio instead uses the SHA-256-pinned archive in `desktop/src/hlae-tool.json`; do not copy a version number into instructions or silently replace the manifest asset.
CS2 must launch through HLAE with `-windowed`; fullscreen and borderless capture are unsupported.
After final media is validated and no recapture/reparse is needed, send used extracted `.dem` files to the Windows Recycle Bin, but keep the original archive unless asked to remove it.

## Development

Toolchain sources of truth are `go.mod` (Go 1.26.5), each package's `packageManager` field (pnpm 11.9.0), and Node 24.
There is no hosted CI: `.githooks/pre-commit` is the only gate, and it runs before the commit exists rather than after the push.
Nothing re-checks the work on GitHub, so a gate skipped locally is a gate that never runs.
The three JavaScript packages have independent lockfiles; run commands with `pnpm --dir web|desktop|landing`, not from an assumed root workspace.

```powershell
.\scripts\build.ps1
.\scripts\local-studio.ps1
go test ./internal/parser -run TestFoo -count=1
go test ./... -count=1 -timeout 3m
& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --no-format
& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --race
& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --security
```

- Bare `bash` is a broken WSL shim on this machine; invoke shell gates through `C:\Program Files\Git\bin\bash.exe`.
- `scripts/go-gate.sh` formats changed Go files by default, then runs tests, vet, `zv check`, and optional `staticcheck`; use `--no-format` in a dirty worktree and add `--build` for the full Go gate.
- Add `--race` for shared-state/concurrency changes and `--security` for filesystem, subprocess, auth, or dependency-sensitive changes.
- Real-demo worker tests skip without `TEST_DEMO_PATH`; parser-only and pure unit tests must not launch HLAE/CS2 or long renders.
- Real `.dem` and generated `*.expected.json` golden files stay local; `testdata/*.rules.json` may be committed when they are reference inputs.

Run package gates in the pre-commit order:

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

The Electron UI E2E is manual and expensive: build, run `pnpm --dir desktop run assemble`, then `pnpm --dir desktop run test:e2e:ui` only when that product flow needs end-to-end verification.
Before frontend work, read `web/CLAUDE.md`; before visual work, also read `web/design.md`.
All browser API access must remain same-origin through server proxy routes, keep orchestrator URLs/tokens server-side, validate IDs before upstream URL construction, and preserve `503 {code: "service_unavailable"}`.
Before Electron lifecycle, packaging, or release work, read `desktop/GUIDE.md` and keep renderer access behind preload/IPC.

## Code Contracts

- Write boring, idiomatic Go.
- Do not introduce `util`, `common`, `helper`, `manager`, or vague service layers.
- Add useful context when returning errors.
- Every goroutine must have a clear owner and stop condition.
- Add or update behavior-level tests for fixes and behavior changes.
- Do not add dependencies or run `go mod tidy` without explicit approval.
- New pipeline failure paths must record once through `internal/obs` with stable `stage` and `class` labels.
- Do not add generated video/audio/image artifacts to git.

## Git And Release

Work directly on `main`; committing or pushing still requires an explicit user request.
`main` is unprotected and there are no required status checks, so a push lands immediately: never open a pull request for work that belongs on `main`.
The change-aware `.githooks/pre-commit` gate runs project checks and package-specific lint/typecheck/test/build commands from staged paths, and it is now the only automated gate the repository has.
Use the authorized global `committer` with explicit, quoted file lists; when a repository-owned `.githooks` directory exists, it activates that directory for the commit without writing persistent Git configuration.
Never bypass the gate with `--no-verify` or by clearing or redirecting `core.hooksPath` away from `.githooks`: with no CI behind it, a skipped hook means the change was never checked at all.

TickCut has no hosted backend; the desktop release command is `pnpm --dir desktop run dist`, which verifies the bundled HLAE and emits installer checksums.
Every desktop distribution must rebuild all Go runtime executables in the same `dist` invocation before `assemble` stages `bin/`; an existing executable is not proof that it matches the current source. Keep the guarded `scripts/build.ps1` step in `desktop/scripts/dist.mjs`, and never publish an installer produced from a manually staged or pre-existing `bin/`.
**Never code-sign the desktop app or installer** (no Authenticode, no `signtool`, no cert/PIN signing, no EV/OV cert purchase or CI signing setup). Shipping stays unsigned on purpose: integrity is the GitHub Release asset plus `SHA256SUMS.txt`, not a publisher signature. Do not treat SmartScreen "unknown publisher" as a release blocker, and do not add signing steps to release docs or automation.
Publish versioned installer assets and `SHA256SUMS.txt` to GitHub Releases in `rechedev9/tickcut`, update the landing download URL, then deploy Vercel project `tickcut-landing` with root `landing/` to `https://tickcut.gravityroom.app/`.
Do not use the retired VPS landing path.

## Codex Harness

```bash
CODEX_DRY_RUN=1 scripts/codex-run.sh .codex/prompts/go-tdd.md "preview"
scripts/codex-go-tdd.sh "behavior change"
scripts/codex-go-bugfix.sh "bug fix"
scripts/codex-go-pr-ready.sh
```

Review findings use `BLOCKER`, `WARNING`, or `NIT` and include file/path, problem, why it matters, and a practical fix; if clean, say `No blocking issues found.`
When using `codex --yolo` with GPT-5.6 Sol Ultra, cap the entire delegation tree at 15 sub-agents.
