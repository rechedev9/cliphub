# FragForge Product Guide

Deterministic CS2 demo/stream-to-video pipeline. Give its agent-first CLI a
`.dem` or a stream clip and produce upload-ready 1080x1920 vertical videos or
1920x1080 long-form videos at 60fps, with polished editing by default.

FragForge ships as a Windows desktop installer.
Download it from the [`landing/`](./landing) site - there is no hosted service to sign up for.

It parses demos into kill plans, records gameplay with HLAE/CS2, and
post-processes clips with FFmpeg, Lua effects, overlays, and publishing
metadata. Capture and rendering run locally on Windows, and no media or audio is
sent to a cloud service.

```text
.dem + prompt
  -> parse demo into a kill plan and scored moments
  -> record the right segments with HLAE/CS2
  -> render at 1080x1920 or 1920x1080 with the viral editing contract (60fps)
  -> publish pack: MP4, cover, caption, gallery, manifest
```

## The one command

```powershell
zv short match.dem --prompt "las mejores kills de martinez" --target-steamid 76561198148986856
```

`zv short` chains parse -> moments -> HLAE/CS2 recording -> render. The prompt
is interpreted deterministically (Spanish and English):

- A 17-digit number or `--target-steamid` selects the target player.
- `mejores` / `best` / `highlights` selects the top moments; otherwise all kills
  are compiled into one Short.
- `música` / `music` / `beat` adds beat analysis for the selected/default
  preset (requires `--music <audio>`).
- An explicit preset name in the prompt (or `--preset`) overrides the default.
- Anything else falls back to the default preset, `viral-60-clean`.

Useful flags:

| Flag | Purpose |
|------|---------|
| `--dry-run` | Print the resolved plan (player, selection, preset, output spec) without running anything. |
| `--format json` | With `--dry-run`, emit the resolved plan as one machine-readable JSON document. |
| `--from-recording <recording-result.json>` | Skip parse + record and render from an existing recording. |
| `--music <audio>` | Music track for beat-synced montages. |
| `--output-format short-9x16\|landscape-16x9` | Choose TikTok/Shorts or long-form YouTube geometry. |
| `--kill-effect clean\|punch-in\|velocity\|freeze-flash` | Choose the kill emphasis used by the edit contract. |
| `--transition cut\|flash\|whip\|dip` | Choose the transition language between selected plays. |
| `--intro`, `--outro` | Add explicit bookend text without editing a lower-level render plan. |
| `--hlae`, `--cs2` | Tool paths (or `ZV_HLAE_PATH` / `ZV_CS2_PATH`). |
| `--out <dir>` | Output directory. |

The command prints a plan summary before running and stage-by-stage progress
(`[1/4] parse`, ...). If a stage fails, rerun with `--from-recording` instead of
recording again.

## Web UI (Local Studio)

Prefer the browser? Local Studio runs the whole product from the web UI on your
own Windows + GPU PC, capture included:

```powershell
.\scripts\local-studio.ps1
```

It starts the orchestrator with a persistent local SQLite job database and an
in-process queue (HLAE/CS2 auto-detected), then starts the web UI and opens
`http://localhost:3000/upload`.
The flow is: upload a demo -> pick a player -> pick specific kills -> create the
reel, at which point HLAE + CS2 open to capture and the edit is applied.

### Codex app (CLI-first)

Open the repository root in Codex Desktop and select Codex. `AGENTS.md` loads
the project rules automatically, and Codex operates FragForge through the
unified Windows CLI; Studio does not need to be running. Build the local
binaries once after cloning or after CLI changes:

```powershell
.\scripts\build.ps1
```

The deterministic agent loop is:

```powershell
.\bin\zv.exe capabilities --format json
.\bin\zv.exe flows show demo --format json
.\bin\zv.exe flows show stream --format json
.\bin\zv.exe workflows list --format json
.\bin\zv.exe workflows show short --format json
.\bin\zv.exe workflows validate short --format json -- match.dem --prompt "all kills 76561198148986856" --dry-run --format json
.\bin\zv.exe workflows run short -- match.dem --prompt "all kills 76561198148986856" --dry-run --format json
```

FACEIT profile discovery is available through the same contract. The Data API
key stays in `FACEIT_API_KEY`; the command never prints or persists it. A
preflight is network-free, while the real call writes every indexed match,
match-level statistics, demo availability, and a manual room link:

```powershell
./bin/zv faceit index --profile https://www.faceit.com/en/players/m0NESY --from 2026-01-01 --to 2026-07-22 --out data/faceit/m0nesy-2026.json --dry-run --format json
./bin/zv workflows show faceit-index
./bin/zv workflows show faceit-index --format json
./bin/zv workflows validate faceit-index --format json -- --profile https://www.faceit.com/en/players/m0NESY --from 2026-01-01 --to 2026-07-22 --out data/faceit/m0nesy-2026.json --dry-run --format json
./bin/zv workflows run faceit-index -- --profile https://www.faceit.com/en/players/m0NESY --from 2026-01-01 --to 2026-07-22 --out data/faceit/m0nesy-2026.json --dry-run --format json
```

After preflight, remove `--dry-run` to create the index. Open each `room_url`
and use FACEIT's Watch/Demo action until Download API access is approved. The
manifest's `highlight_match_ids` is transparent match-level triage (multi-kill
round counts, kills, ADR, date); it does not infer clip ranges. Downloaded
`.dem` data remains authoritative for POV, weapons, kills, and capture ticks.

`capabilities` reports local record/compose/render/stream readiness without
starting workers or media processes. Workflow discovery exposes canonical commands,
positionals, flags, conditional requirements, allowed values, defaults, and
safety hints. Validation checks the exact argv without executing it; after it
succeeds, Codex can run the same argv. Remove both `--dry-run --format json`
only when an actual capture/render was requested; real execution streams
human-readable stage progress.

For a reviewable demo journey, use the granular commands instead of asking an
agent to guess across decision boundaries:

```powershell
zv demo players --demo match.dem --format json --out data\runs\match\players.json
zv demo parse --demo match.dem --steamid 76561198148986856 --out data\runs\match\plan.json
zv demo moments --killplan data\runs\match\plan.json --top 10 --format json --out data\runs\match\moments.json
zv demo select --killplan data\runs\match\plan.json --segments seg-003,seg-007 --out data\runs\match\selected-plan.json --dry-run --format json
zv record --killplan data\runs\match\selected-plan.json --demo match.dem --out data\runs\match\recording --dry-run --format json
zv shorts render --recording-result data\runs\match\recording\recording-result.json --out data\runs\match\shorts --publish-dir data\runs\match\shortslistosparasubir --output-format landscape-16x9 --kill-effect velocity --transition whip --dry-run
```

The first four stages are cheap and reviewable. Remove `--dry-run` from
`demo select` to persist the approved plan, then from capture and render only
after the player, segment order, output geometry, and edit choices are final.
The demo flow exposes two required human gates: `creative-brief` asks only the
still-unanswered choices for HUD/killfeed, effects, transitions, kill numbering,
intro/outro, music, and thumbnail direction; `thumbnail-selection` presents the
generated cover candidates before the pack is declared upload-ready. That
second gate applies only when covers are enabled; `--covers=false` has no
thumbnail to approve. A user can explicitly delegate either choice, but agents
must not silently skip an applicable gate.

`short --dry-run --format json` returns one JSON plan containing the resolved
player, selection, preset, output paths, and exact stage argv with
`executed: false`. For real `short` and `record` runs, missing HLAE/CS2 flags
are filled from environment or local autodetection; explicit flags still win.
HLAE autodetection compares version numbers and selects the highest installed
`C:\HLAE-*\HLAE.exe`. Keep that installation on the latest official AdvancedFX
release rather than pinning a release in commands or documentation.
The primary `short` workflow keeps render intermediates under `shorts/` and
writes the final upload-ready pack to `<run>/shortslistosparasubir/`.

Stream/VOD clips use the same CLI-first loop. Codex first discovers the layout
registry, probes the source into an editable plan, then validates and renders
that plan locally:

```powershell
./bin/zv stream variants --format json

# Preflight the plan without writing it.
./bin/zv stream plan --input stream.mp4 --out data/runs/stream/edit-plan.json --dry-run --format json

# After approval, persist every contract before the next stage consumes it.
./bin/zv stream plan --input stream.mp4 --out data/runs/stream/edit-plan.json --format json
./bin/zv stream render --input stream.mp4 --plan data/runs/stream/edit-plan.json --out data/runs/stream --dry-run --format json
./bin/zv stream render --input stream.mp4 --plan data/runs/stream/edit-plan.json --out data/runs/stream --format json

# The same persisted sequence through the workflow registry.
./bin/zv workflows validate stream-plan --format json -- --input stream.mp4 --out data/runs/stream/edit-plan.json
./bin/zv workflows run stream-plan -- --input stream.mp4 --out data/runs/stream/edit-plan.json
./bin/zv workflows validate stream-render --format json -- --input stream.mp4 --plan data/runs/stream/edit-plan.json --out data/runs/stream --dry-run
./bin/zv workflows run stream-render -- --input stream.mp4 --plan data/runs/stream/edit-plan.json --out data/runs/stream
```

Every `--dry-run` line above is a preflight only: it never creates its `--out`
artifact. Run the immediately following persistence command after approval
before continuing to the dependent stage. The final non-dry `stream render` is
the expensive local media operation.
Its upload-ready pack includes a cover JPG taken at 35% of the rendered clip duration, alongside the video, manifest, and gallery.

Use the default vertical variant for TikTok/Shorts or pass
`--variant streamer-landscape-16x9` to `stream plan` for a 1920x1080 YouTube
edit that preserves the complete source frame, HUD, killfeed, and facecam.

The edit plan is the reviewable contract for clip ranges, normalized facecam and gameplay crops, music, effects, and text.
Final MP4s, covers, manifest, and local gallery are written to `<run>/shortslistosparasubir/`.

Ask naturally from the app, for example: “usa la CLI de FragForge para revisar
las capacidades, valida un Short con todas las kills de este demo y ejecútalo”.
The repo-local production skills add the specialized parse, capture, render,
QA, and publishing steps while continuing to use `zv workflows run`.

### Manual YouTube publication assistant

Every finished reel in the Library has a **PREPARAR PUBLICACIÓN** action. It
builds a seven-day schedule in `Europe/Madrid`, three to five factual metadata
alternatives, keywords, and tags from the reel itself. The title and description
remain editable, and the dialog can copy each field, download the MP4, and open
the stable [YouTube Studio](https://studio.youtube.com/) home page in the system
browser.

FragForge does not choose the channel, audience, visibility, or publication
date. In YouTube Studio, follow the official **CREAR -> Subir vídeos** flow and
complete those decisions there; see [YouTube's official upload guide](https://support.google.com/youtube/answer/57407?hl=es).
No Google project or account connection is required by FragForge.

The assistant uses this deterministic local-time Shorts reference:

| Day | Recommended local hours, in order |
|-----|-----------------------------------|
| Monday | 20:00, 17:00, 18:00 |
| Tuesday | 20:00, 21:00, 19:00 |
| Wednesday | 19:00, 20:00, 21:00 |
| Thursday | 19:00, 20:00, 21:00 |
| Friday | 16:00, 18:00, 19:00 |
| Saturday | 19:00, 11:00, 18:00 |
| Sunday | 19:00, 20:00, 17:00 |

If `FIRECRAWL_API_KEY` is present server-side, Studio also performs a bounded
monthly search for recent public CS2 Shorts and shows the extracted terms as
trend hints. Firecrawl results never masquerade as YouTube views or ranking
metrics, and the key is never sent to the renderer. Without the key, the same
schedule and reel-derived recommendations remain available. Suggestions only
use terms that match the player, map, weapon, hook, or kill count from that
request.

Experimental local builds may have left generic Windows Credential Manager
entries named `FragForge/YouTube/OAuthClient`,
`FragForge/YouTube/Connection`, or `FragForge/YouTube/Upload/<id>`. Current
builds do not read or delete them. Remove them manually from **Credential
Manager -> Windows Credentials** if desired.

## Render presets

`internal/editor/preset.go` is the preset source of truth.
The loadout catalog (`internal/renderplan`), the HTTP API (`/api/presets`, `/api/loadouts`, render-variant validation), the workbench UI, and the render worker all derive from that registry.
All current presets default to 1080x1920 at 60fps; `--output-format landscape-16x9` uses the same editing contract at 1920x1080.
Unknown preset names are rejected with the valid list.

List them any time with `zv presets` (`--format json` for automation).

| Preset | What it does |
|--------|--------------|
| `viral-60-clean` (default) | HUD-less POV with in-game death notices and `viral-ultra-clean` effects. |
| `clean-pov-60` | Fully HUD-less POV with no in-game killfeed. |
| `full-hud-60` | Full gameplay HUD, including health, ammo, radar, and killfeed. |

The editing choices behind the viral presets: hook text in the first 1-2s,
punch-ins on kills, slow-mo only on the final kill, beat-synced drops,
loop-friendly endings, never cropping the killfeed.

## Quick start

Requires Go 1.26+, FFmpeg, and (for recording) CS2 plus HLAE.

```powershell
# Build all binaries into .\bin
.\scripts\build.ps1

# See what a run would do
.\bin\zv short testdata\match.dem --prompt "all kills" --target-steamid 76561198000000000 --dry-run

# Sanity-check the project contract (skills, workflows, docs)
.\bin\zv check
```

Unix-like shells can use `make build` / `make test` instead.

### Orchestrator (HTTP API + workers)

```bash
export ZV_DATABASE_URL=memory   # in-memory job repo + inline queue, no external services needed
export ZV_DATA_DIR="./data"
export ZV_MUTATION_TOKEN="$(openssl rand -hex 32)" # required per launch

./bin/zv serve
```

The server binds to `127.0.0.1:8080` by default and rejects non-loopback
addresses. `ZV_MUTATION_TOKEN` is a required 32-byte lowercase-hex session
capability for API reads and mutations. Other environment variables:

| Variable | Purpose |
|----------|---------|
| `ZV_HTTP_ADDR` | Listen address (default `127.0.0.1:8080`). |
| `ZV_HLAE_PATH`, `ZV_CS2_PATH` | Recording tool paths. |
| `ZV_RECORDER_PATH`, `ZV_COMPOSER_PATH`, `ZV_FFMPEG_PATH` | Stage binary overrides. |
| `ZV_WORKER_CONCURRENCY` | Asynq worker concurrency (default 2). |
| `ZV_MEDIA_WORK_DIR` | Keep media workdirs for debugging (deleted after each task when unset). |
| `FIRECRAWL_API_KEY` | Optional public CS2 Shorts trend hints for the publication assistant; never sent to the browser. |

### Smoke tests

```bash
# Parser-only (requires a .dem in testdata/)
./scripts/smoke.sh testdata/<your-demo>.dem <SteamID64>
```

```powershell
# Full real run against a running orchestrator with recorder/composer configured
.\scripts\smoke-real.ps1 -Demo testdata\lavked-vs-tnc-m2-nuke.dem -TargetSteamID 76561198148986856
```

The real smoke uploads the demo, waits for `parsed`, records, retries `record`
once to verify artifact skipping, composes, retries `compose`, then downloads
the final MP4 and validates H.264, 1920x1080, 60fps when `ffprobe` is
available.

## HTTP API

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/jobs` | Multipart upload: `demo` file + `config` JSON (`{"target_steamid":"..."}`). Returns `201 {id, status}`. |
| GET | `/api/jobs/{id}` | Job metadata and status; `?view=status` returns the lightweight polling representation. |
| GET | `/api/jobs/{id}/plan` | Kill plan JSON (200) or 409 if not ready. |
| POST | `/api/jobs/{id}/record` | Enqueue recording after parse approval. |
| POST | `/api/jobs/{id}/compose` | Enqueue final composition after recording. |
| GET | `/api/jobs/{id}/final` | Stream the composed MP4 when ready. |
| GET | `/api/presets` | Render preset registry as JSON (name, geometry, behavior flags, default). |
| GET | `/api/stream-variants` | Stream/VOD render variant registry, including the default. |
| GET | `/api/jobs/{id}/renders/{variant}/videos/{name}/publish-assistant?days=7` | Reel metadata, factual suggestions, Madrid schedule, optional trend hints, and the stable YouTube Studio URL. |

`POST /record` is accepted for `parsed` and `recorded` jobs; `POST /compose`
for `recorded` and `composed` jobs. Manual retries are idempotent: workers skip
external media commands when the durable artifacts already exist. Render
variant requests are validated against the preset registry; scored moments
default to `viral-60-clean`.

## CLI reference

`zv` is the unified entrypoint. Stage commands remain available for granular or
scripted use:

```bash
./bin/zv demo parse --demo match.dem --steamid 76561198000000000 --out plan.json
./bin/zv demo players --demo match.dem --format json --out players.json
./bin/zv demo moments --killplan testdata/agent-killplan.json --top 10 --format json --out data/runs/agent-doc/moments.json
./bin/zv demo select --killplan testdata/agent-killplan.json --segments seg-001 --out data/runs/agent-doc/selected-plan.json --dry-run --format json
./bin/zv record --killplan plan.json --demo match.dem --out run/recording --hlae <HLAE.exe> --cs2 <cs2.exe>
./bin/zv shorts render --recording-result run/recording/recording-result.json --out run/shorts --publish-dir run/shortslistosparasubir --preset viral-60-clean --output-format short-9x16
./bin/zv compose final --recording-result run/recording/recording-result.json --out run/final.mp4
./bin/zv music analyze --input track.mp3 --killplan plan.json --out run/rhythm.json
./bin/zv stream plan --input stream.mp4 --out data/runs/stream/edit-plan.json --dry-run --format json
./bin/zv stream render --input stream.mp4 --plan data/runs/stream/edit-plan.json --out data/runs/stream --dry-run --format json
./bin/zv analysis tactical --demo match.dem --out data/analysis/match-tactical.json --positions data/analysis/match-positions.zvpos --hz 8 --cell-size 64 --dry-run --format json
./bin/zv analysis rounds --tactical testdata/agent-tactical.json --side T --buy full --format json
./bin/zv analysis tendencies --tactical testdata/agent-tactical.json --team t-start --phase regulation --format json
./bin/zv flows list --format json
./bin/zv flows show demo --format json
./bin/zv flows show stream --format json
./bin/zv presets
./bin/zv capabilities --format json
./bin/zv check
./bin/zv serve
```

`zv analysis tactical` scans a demo once into a durable tactical document: the
round index with its economy and deterministic classification, the per-round
event list, the map geometry derived from observed play, and the descriptor of
an optional sidecar position blob.
`zv analysis rounds` and `zv analysis tendencies` then read that document
without touching the demo again.
Both accept the same round filters (`--side`, `--team`, `--buy`,
`--opponent-buy`, `--site`, `--outcome`, `--t-pattern`, `--ct-pattern`, `--tag`,
`--slot`, `--round-from`, `--round-to`, `--phase`), which map onto the one
filter vocabulary the local HTTP API also parses.
Every aggregated rate prints its denominator, and any rate computed from fewer
rounds than the reliable sample size is marked `low-sample`.

Other command groups: `zv utility audit` (lineup catalogs), `zv analysis view`
and `zv analysis tactical-data` (legacy replay viewers and exports),
`zv gallery open`, `zv skills` and `zv workflows`
(repo-local agent skills and the cataloged workflow contract; both support
`--format json`). Legacy binaries stay reachable through pass-throughs such as
`zv parser`, `zv editor`, `zv recorder`, `zv composer`, and `zv orchestrator`.

`zv shorts render` options worth knowing:

- `--segments seg-001,seg-004` / `--limit N` for fast partial iteration, plus
  `--skip-existing` and `--open-gallery`.
- `--render-jobs N` caps how many shorts render concurrently (default 0 =
  automatic CPU-based limit; pass 1 to force sequential rendering).
- `--output-format short-9x16|landscape-16x9`, `--kill-effect`, `--transition`,
  `--intro-text`, and `--outro-text` expose the high-level delivery and editing
  decisions without requiring an agent to rewrite manifests.
- `--dry-run` writes planned manifests, captions, FFmpeg commands, and cover
  prompts without rendering.
- `--music`, `--rhythm`, `--compile-segments` for music-scripted compilation
  edits (analyze the track first with `zv music analyze`).
- `--effects-preset viral-ultra-clean` or `--effects <script.lua>` for explicit
  custom Lua effects. The Lua DSL
  exposes `on_segment`, `on_kill`, `on_smoke`, `zoom`, `flash`, `text`, and
  `grade`; scripts run sandboxed (no filesystem/process access) with a capped
  evaluation budget. Standard kill/highlight renders use `viral-ultra-clean`.
- `--music`, `--rhythm`, and `--compile-segments` for music-synced
  compilations with the same `viral-60-clean` visual standard.

Every render writes its upload-ready pack under the run's
`shortslistosparasubir/` directory: clean MP4
filenames, caption files, cover JPGs, `pack-manifest.json`,
`publish-summary.md`, and an `index.html` review gallery with copy buttons.
Outputs are validated against the selected 1080x1920 or 1920x1080 H.264 60fps
profile. Vertical outputs are warned if they exceed the 180s Shorts limit.

## Media artifacts and cleanup

Durable local storage keeps, per job: `recording/recording-result.json`,
`recording/recording.js`, `recording/segments/*.mp4`, `shorts/*` (manifest,
result, prompts, publish pack), and `composition/final.mp4` with its result
JSON. Treat `data/` as output unless you are explicitly working on fixtures.

Local edit experiments can pile up `shorts*` directories. The cleanup script
previews by default and only deletes with `-Apply`:

```powershell
.\scripts\cleanup-artifacts.ps1            # preview targets and estimated space
.\scripts\cleanup-artifacts.ps1 -Apply     # delete regenerable variants, keep baselines
```

Pass `-RunDir` and comma-separated `-KeepShortsDir` values to clean a different
run. Never commit generated video/audio/image artifacts to git.

## Repository layout

- `cmd/` — thin CLI entrypoints (`zv`, `zv-parser`, `zv-recorder`,
  `zv-composer`, `zv-editor`, `zv-orchestrator`, ...).
- `internal/parser` — `.dem` parsing and segment extraction.
- `internal/killplan` — shared kill/segment plan types.
- `internal/moments` — scored, reviewable clip candidates from kill plans.
- `internal/recording` — HLAE/CS2 recording scripts and validation.
- `internal/editor` — Shorts rendering, the preset registry, Lua effects,
  validation, publish packs.
- `internal/renderplan` — render variants, loadouts, edit documents, QA.
- `internal/composition` — concat/composition planning and FFmpeg boundaries.
- `internal/httpapi` — orchestrator HTTP routes, handlers, and the embedded
  workbench UI.
- `internal/workers` — Asynq parser and media workers.
- `internal/youtubeinsights`, `internal/youtubetrends` — explainable Madrid
  scheduling, factual metadata recommendations, and optional bounded Firecrawl
  discovery for the manual publication assistant.
- `internal/storage`, `internal/job`, `internal/tasks` — persistence and job
  state.
- `effects/` — editable Lua effect scripts.

## Tests

```bash
make test            # also runs `zv check` to keep the CLI contract aligned
scripts/go-gate.sh   # main verification gate (fmt, vet, build, tests)
```

Worker integration tests that need a real demo skip automatically when
`TEST_DEMO_PATH` is unavailable. Tests never launch HLAE/CS2 or long FFmpeg
renders.
