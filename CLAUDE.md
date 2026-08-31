# ClipHub Agent Instructions

`AGENTS.md` is a tracked symbolic link to this file.
Edit `CLAUDE.md` only, and never replace the `AGENTS.md` symlink with a regular file.
`scripts/codex-harness.ps1 -Action Doctor` rejects a broken `AGENTS.md` symlink.
On Windows a clone with `core.symlinks=false` materializes `AGENTS.md` as a regular 9-byte file whose contents are the text `CLAUDE.md`, and `git status` stays clean because a symlink's blob is its target. That silently fails the hook and every `zv check` rule that reads `AGENTS.md`. Repair it with `git config core.symlinks true` followed by deleting the file and `git checkout -- AGENTS.md` (needs Developer Mode or an elevated shell); never "fix" it by pasting the guide's text into the file.
The root `README.md` is the public product entrypoint (GitHub + onboarding). Prefer purpose-specific names such as `GUIDE.md`, `RUNBOOK.md`, or `PROVENANCE.md` for operational detail.

## Product

ClipHub is a Windows-local, deterministic CS2 demo/stream-to-video pipeline written primarily in Go.
The demo is the source of truth for player, camera, tick ranges, kills, and utility; never infer recording decisions from rendered video.

```text
.dem -> parse/score -> selected kill plan -> HLAE/CS2 capture -> FFmpeg/Lua render -> publish pack
stream video -> persisted edit plan -> render -> publish pack
```

- `cmd/` is thin flags + `os.Exit`; domain stays in `internal/`. Recorder launch, orchestrator SQLite/queue, analysis-viewer, and demo-players roster parse already leak — see `cmd/AGENTS.md`. Do not copy that pattern.
- `internal/parser`, `internal/killplan`, and `internal/moments` own the durable plan passed to every later demo stage.
- `internal/recording` owns HLAE/CS2 scripts and capture validation; `internal/editor`, `internal/renderplan`, and `internal/composition` own post-capture effects, variants, QA, and FFmpeg composition.
- `effects/` contains sandboxed `gopher-lua` scripts with no filesystem or process access.
- `internal/httpapi` and `internal/workers` implement the local API and jobs; the inline queue has a dedicated one-worker capture lane because all captures contend for one `cs2.exe`.
- Workers skip completed durable artifacts on retry, but recording is enqueued with `MaxRetry(0)`; a `demo_incompatible:` failure is deterministic and must not be retried against the same CS2 build.
- Series jobs share a client-minted `series_id`; roster choice is aggregated across maps, and HLTV `-pN.dem` parts are one logical map.
- `web/` is the Next.js 16 / React 19 local Studio UI. `desktop/` packages it with the Go services in Electron (no React in `desktop/src`). `landing/` is the only hosted app (Next.js 15; it has a build command but no lint/test scripts).
- `data/`, `bin/`, capture output, and generated media are artifacts, not source, unless a task explicitly targets fixtures or cleanup.
- Agent-oriented architecture guidance lives in `docs/AI_AGENT_ARCHITECTURE.md`: agents discover contracts, inspect durable artifacts, ask for unanswered gates, and execute only approved explicit commands. Do not convert that guidance into model-driven clip decisions.

## Punk Records

Shared standing law for every agent that touches this repo. If it is not here, it did not happen. Update this block in the same PR when a law changes.

- **Prove It Works.** Verify the real path, not a proxy. CI, mocks, lint, and "it compiles" are supplemental. For parser/capture/render: real ClipHub Studio on Windows + HLAE/CS2, or name the gap and do not call the work done.
- **PR body** (`.github/pull_request_template.md`): only four H2s — What Problem This Solves, Why This Change Was Made, User Impact, Evidence. Empty Evidence is a blocker.
- **Hosted quality CI** (PRs and `main`): `CI frontend` (web + desktop typecheck/lint/unit), `CI backend` (`go vet`, `go test ./...`, `zv check`), `CI infra` (actionlint + unsigned-release contract). Not HLAE/CS2 E2E. Not Playwright.
- **Release:** unsigned Windows installer only, via `.github/workflows/desktop-release.yml` (`workflow_dispatch` or `v*.*.*` matching `desktop/package.json`). Actualizar reads `releases/latest`. No Authenticode. Vercel/landing is not the updater.
- **Structural flows:** (1) Demo parser → 9:16 Shorts (2) Full Demo → 16:9 recap. A PR that touches a flow must prove that flow. Unknown on a touched flow is a merge block.
- **Current Studio** (as of 2026-08-31): 2.4.37.

## Where To Look

| Task | Location |
|------|----------|
| Go package map | `internal/AGENTS.md` |
| 12 binaries / cmd leaks | `cmd/AGENTS.md` |
| `zv` CLI surface | `cmd/zv/AGENTS.md`; contract is `zv flows show` / `workflows show` |
| AI-agent system design | `docs/AI_AGENT_ARCHITECTURE.md` |
| Studio UI / TypeScript | `web/CLAUDE.md`; visuals: `~/.grok/design.md` then `web/app/globals.css` + `frontend-design` skill |
| Electron / installer | `desktop/GUIDE.md` |
| Testdata / goldens | `testdata/GUIDE.md` |
| HLAE experiments | `scripts/hlae/` — not product capture |
| HyperFrames probe | `overlays/hyperframes/` — not the FFmpeg/Lua pipeline |

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
- Use `stream fetch -> stream variants -> stream plan -> human review -> stream render` for VODs; skip fetch when the source is already a local MP4.
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
ClipHub prepares a report dossier and links the official channels; it must never submit a report, automate a submission, or help produce several reports against one account.
Valve decides cheating bans from its own detection, not from report volume, and coordinated mass reporting is both ineffective and against the Steam Subscriber Agreement.
Do not add a feature that files reports on the user's behalf, and do not weaken the `insufficient_data` and confidence gates that stop a thin sample from producing a verdict.

## Share Codes And Steam

A CS2 match sharing code (`CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx`) is bijective base-57 over a 57-character dictionary; `internal/sharecode` decodes it to `matchid`, `outcomeid` and `tokenid` with pure arithmetic, no network and no credentials. `web/lib/sharecode.ts` validates shape only — never port the base-57 decode to TypeScript, because two implementations of one bijection is how they start disagreeing.

Listing recent matches is a different, cheaper capability: Valve's `ICSGOPlayers_730/GetNextMatchSharingCode` walks share codes from a known starting code. It needs the player's SteamID64, the revocable authentication code from `help.steampowered.com` (`appid=730&issueid=128`), and that player's Steam Web API key. Those three live in `<dataDir>/steam/account.json`. They are not a password.

Turning a share code into a downloadable `.dem` still needs a logged-in CS2 Game Coordinator session. `internal/steamgc` owns the wire format (`k_EMsgGCCStrike15_v2_MatchListRequestFullGameInfo` 9147 out, `k_EMsgGCCStrike15_v2_MatchList` 9139 back; the demo URL arrives as the `map` field of the last `roundstatsall` entry). `internal/steamresolve` owns decode, history, and fetch. `internal/steamclient` is the go-steam session; httpapi must not import it.

- Ajustes collects SteamID + authentication code + Web API key. Never persist `ZV_STEAM_USERNAME` / `ZV_STEAM_PASSWORD` / `ZV_STEAM_GUARD`. Password is prompted on the first download, held in process memory, or read from those env vars. Never log it, never echo it into an error.
- The account to connect is **the one the user plays on**. The match history lives in that account, so a secondary bot account enumerates nothing.
- Steam allows one CS2 session per account, so opening the GC disconnects a live match. Touch the GC **only on an explicit user action**, keep the session short, and disconnect when done. Never poll it, never open it at startup, and keep saying so in the UI.
- Decoding must keep working when Steam is not configured: "we know which match this is, we just cannot fetch it" is a real answer and returns `status: "decoded"`, not an error. Download is `POST /api/steam/import` and ends in the same `CreateJob` path as `/upload`.
- The two ids exceed 2^53, so they cross the HTTP boundary as strings. Parsing them as JavaScript numbers silently corrupts them.
- `go-steam` and `demoinfocs-golang` both register protobuf extension 50000. `internal/allowproto` must stay imported by the orchestrator so the second registration is ignored. `internal/steamclient` cannot be imported from `httpapi`.
- `internal/steamclient` cannot be exercised by the test suite: it needs real credentials. Treat it as unverified until someone runs it against a live account.

## Approval And Media

- Before any non-dry-run capture or render, stop at the creative brief gate and ask only unanswered choices: format, HUD/killfeed, kill effect, transition, counter, intro/outro, music, and cover strategy.
- Approval must answer a shown brief; ambiguous words such as `go`, `hazlo`, `dale`, or `ok` are not approval by themselves.
- Translate every approved brief choice into an explicit final command value, including negative booleans such as `--kill-counter=false`, `--hook=false`, and `--covers=false`; never rely on a preset or flag default to preserve an approved `off` choice. After rendering, inspect the effective result configuration and generated effects/metadata, and reject any output that re-enables a disabled element or contradicts the selected kills, weapons, rounds, or narrative.
- A successful render is not final while QA has unresolved warnings. Inspect every warning at its exact interval; remove unintended frozen, post-death, or dead-air footage, or document why it is intentional, then rerun QA.
- Any trim, reorder, or duration change invalidates existing rhythm timing. Regenerate or update the canonical rhythm plan and verify every selected kill against its assigned beat or onset before rerendering.
- For streams, also settle clip bounds/title, crop/framing, and source-audio treatment.
- Thumbnail approval is a second gate for CLI/agent pack delivery after candidates exist; require a selected candidate or explicit delegation before calling that pack upload-ready. Studio Library does not use this gate: a ready reel's MP4 download and PREPARAR PUBLICACIÓN are enabled as soon as the video file exists. Cover JPGs may still be generated in the render pipeline; picking one is not required in the Library ready card.
- Before marking a CLI pack upload-ready, verify that the canonical MP4, title, caption, hashtags, cover, cover timestamp, gallery, manifest paths, and artifact metadata describe the same facts and files. After thumbnail selection, replace the canonical cover and visually verify the gallery again.
- `--covers=false` removes the CLI thumbnail gate.
- Changing a stream plan invalidates its creative brief; settle the brief again before the next non-dry-run render.

The editor registry in `internal/editor/preset.go` retains the capture-mode profiles, but the public unified `zv` preset catalog exposes `viral-60-clean` and `viral-aggressive-60`; discover the supported CLI values with `.\bin\zv.exe presets --format json`.
`viral-60-clean` records death notices and uses `viral-ultra-clean` effects. `viral-aggressive-60` records the same death notices HUD with the `viral-aggressive` grade and headshot chroma pulses. `clean-pov-60` and `full-hud-60` remain internal capture profiles and are not selectable through the unified CLI without a future product decision.
HUD mode is a recording-stage choice, so changing it after capture requires recapture rather than a render-only change.
The default kill/highlight deliverable is one compiled vertical video per player/game containing all selected kills, not one upload-ready file per kill.
Put every final MP4, cover, caption, manifest, and review gallery under the run's `shortslistosparasubir/` directory, and point the user there when delivering media.
For third-party music, persist the source URL, creator, license, downloaded-file SHA-256, and rhythm-analysis evidence under the run. Never claim a track is CC0 or otherwise reusable without an authoritative source.

Do not launch HLAE, CS2, a long FFmpeg render, or paid/cloud media work without an explicit request; prefer the CLI preflight.
Host capture auto-detects the highest installed HLAE version under `C:\HLAE-*\HLAE.exe`; before a real run, compare it with the latest official HLAE release.
Never use `C:\HLAE\HLAE.exe` for ClipHub capture.
Packaged Studio instead uses the SHA-256-pinned archive in `desktop/src/hlae-tool.json`; do not copy a version number into instructions or silently replace the manifest asset.
CS2 must launch through HLAE with `-windowed`; fullscreen and borderless capture are unsupported.
After final media is validated and no recapture/reparse is needed, send used extracted `.dem` files to the Windows Recycle Bin, but keep the original archive unless asked to remove it.

## Development

Toolchain sources of truth are `go.mod` (Go 1.26.6), each package's `packageManager` field (pnpm 11.22.0), and Node 24.
Pull requests and pushes to `main` run three cheap hosted checks: `CI frontend` (web + desktop typecheck/lint/unit), `CI backend` (`go vet`, `go test ./...`, `zv check`), and `CI infra` (actionlint + the unsigned-release contract). These are not HLAE/CS2 E2E and do not replace local verification.
The only hosted release pipeline is `.github/workflows/desktop-release.yml`: a `windows-latest` job that runs `pnpm --dir desktop run dist` and publishes the unsigned NSIS installer (`ClipHub.Studio.Setup.<ver>.exe`, `.exe.blockmap`, `SHA256SUMS.txt`) to GitHub Releases. Trigger it with `workflow_dispatch` or by pushing a `v*.*.*` tag that matches `desktop/package.json`. It must stay the only release job.
The three JavaScript packages have independent lockfiles; run commands with `pnpm --dir web|desktop|landing`, not from an assumed root workspace.
Lint is oxlint, not ESLint. Unit tests are `node:test` on colocated `*.test.ts` / `*.test.mjs`; no Jest/Vitest. `web/proxy.ts` is the Next 16 request guard (not `middleware.ts`). Browser E2E is Playwright in `web/e2e/`, run explicitly with `pnpm --dir web run test:e2e`; landing has no lint/test scripts.

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
- Table-driven tests are required wherever the pattern fits; near-duplicate test clones are a review finding.
- Real-demo worker tests skip without `TEST_DEMO_PATH`; parser-only and pure unit tests must not launch HLAE/CS2 or long renders.
- For applicable demo/capture/render changes, read `docs/CAPTURE_LAB.md` and run the cheapest relevant Capture Lab level, escalating through `scripts/capture-lab.ps1 -Mode Full` when end-to-end behavior is in scope. Report the highest completed level and never describe L1-L4 simulation as current HLAE/CS2 certification.
- The default Capture Lab never launches HLAE/CS2. Its Windows real canary remains an explicit, separately approved action and simulated `capture_mode: "fake"` evidence must never cross the production-real validation gate.
- Real `.dem` and generated `*.expected.json` golden files stay local; `testdata/*.rules.json` may be committed when they are reference inputs. Fixture convention: `testdata/GUIDE.md`.

Run package checks in this order when their package is affected:

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
Before frontend work, read `web/CLAUDE.md`; before visual work, read `~/.grok/design.md`, then load the `frontend-design` skill and restyle onto `web/app/globals.css`.
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

Committing or pushing still requires an explicit user request.
`main` is unprotected and has no required status checks, so a push lands immediately.
There is no automatic commit-time quality gate. The agent must run focused tests plus the affected package checks before a requested commit. Unsigned Windows installers are cut by `.github/workflows/desktop-release.yml`.
Use ordinary non-interactive Git commands for requested commits and stage only the explicit in-scope paths. Never disturb unrelated staged or unstaged work.

### Parallel Workspaces

- Keep five long-lived, isolated workspaces for parallel implementation: `/home/reche/projects/fragforge` is the canonical integration checkout, and `/home/reche/projects/fragforge-slot-1` through `/home/reche/projects/fragforge-slot-4` are execution slots. Use independent full clones for these five workspaces; each must keep its own `.git` directory and must not use shared object alternates.
- This Windows checkout (`tickcut`) is outside that Linux pool. Do not treat it as a slot or apply slot push/reuse rules here.
- Keep every long-lived checkout on `main` and assign at most one writer or task to each slot. Before reusing a clean slot, synchronize it with `origin/main` using fast-forward-only operations; never discard uncommitted work to make a slot reusable.
- Git worktrees are allowed for temporary review, inspection, or verification tasks and do not count toward the five long-lived workspaces. Agents may create and remove clean temporary review worktrees as needed; never remove or repurpose one that contains uncommitted work.
- Do not push from execution slots. Integrate and push only from the canonical checkout, and continue to require explicit user approval before every commit or push.
- `/home/reche/projects/fragforge-tactical` is a legacy worktree outside the five-workspace pool. Do not remove or modify it unless the user explicitly assigns work there or requests migration or cleanup.

ClipHub has no hosted backend; the desktop release command is `pnpm --dir desktop run dist` on Windows, or the `Desktop release` GitHub Action on `windows-latest`.
Every desktop distribution must rebuild all Go runtime executables in the same `dist` invocation before `assemble` stages `bin/`; an existing executable is not proof that it matches the current source. Keep the guarded `scripts/build.ps1` step in `desktop/scripts/dist.mjs`, and never publish an installer produced from a manually staged or pre-existing `bin/`.
**Never code-sign the desktop app or installer** (no Authenticode, no `signtool`, no cert/PIN signing, no EV/OV cert purchase or CI signing setup). Shipping stays unsigned on purpose: integrity is the GitHub Release asset plus `SHA256SUMS.txt`, not a publisher signature. Do not treat SmartScreen "unknown publisher" as a release blocker, and do not add signing steps to release docs or automation.
Publish versioned installer assets and `SHA256SUMS.txt` to GitHub Releases in `rechedev9/cliphub`. Actualizar reads `releases/latest`; a landing/Vercel deploy is not required for the in-app updater.
Do not use the retired VPS landing path.

## Codex Harness

```bash
powershell -ExecutionPolicy Bypass -File scripts/codex-harness.ps1 -Action Doctor
powershell -ExecutionPolicy Bypass -File scripts/codex-harness.ps1 -Action Preview -Playbook tdd "behavior change"
powershell -ExecutionPolicy Bypass -File scripts/codex-harness.ps1 -Action Run -Playbook bugfix "bug fix"
powershell -ExecutionPolicy Bypass -File scripts/codex-harness.ps1 -Action Check
```

Review findings use `BLOCKER`, `WARNING`, or `NIT` and include file/path, problem, why it matters, and a practical fix; if clean, say `No blocking issues found.`
When using `codex --yolo` with GPT-5.6 Sol Ultra, cap the entire delegation tree at 15 sub-agents.
