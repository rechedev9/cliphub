## Inventory

**TS clients:** `web/lib/api/index.ts` binds `api` to `RealApiClient` only. Parallel clients: `editorApi`, `streamsApi`, plus `tactical.ts` / `anticheat.ts` / `faceit.ts` / `steam-*.ts` that `fetch('/api/...')` directly.

**Go job JSON (`internal/job/job.go:98-118`):** `id` UUID string, `status`, `failure_reason`, `failure_code`, `series_id`, `demo_file_name`, `demo_path`, `demo_sha256`, `target_steamid`, `rules`, `kill_plan`, `created_at`, `updated_at`.

**Go job statuses (`job.go:27-58`):** `queued`, `parsing`, `parsed`, `recording`, `recorded`, `composing`, `composed`, `done`, `failed`, `scanning`, `scanned`, `review_required`.

**Go render statuses (`internal/renderplan/render_variant.go:13-18`):** `queued`, `rendering`, `ready`, `review_required`, `failed`. GET body is `RenderVariantState` + `videos`/`covers`/`segment_ids`/`edit`/`music` (`handlers.go:1855-1860`). Failure text is `error`, not `failure_reason` (`render_variant.go:38`).

**Go stream statuses (`internal/streamclips/types.go:23-29`):** `acquiring`, `uploaded`, `ready`, `rendering`, `rendered`, `failed`. Job JSON includes `source_path`, `source_sha256` (`types.go:54-74`).

**Go editor statuses (`internal/timelineplan/types.go:27-30`):** `draft`, `rendering`, `rendered`, `failed`. GET project remaps plan to a Document object (`editor_handlers.go:574-586`).

**Proxy core:** `web/app/api/demos/_lib.ts` → `ORCHESTRATOR_URL` default `http://127.0.0.1:8080`, `503 {error, code: service_unavailable}` (`74-79`). UUID guard on job/series/stream/editor ids.

---

## Findings

1. **BLOCKER** — `web/lib/api/real.ts:982-1032` vs `internal/renderplan/render_variant.go:23-40`
   - **Problem:** `fetchRenderStatus` reads `failure_reason`. Go emits `error`.
   - **Why:** Render failures show generic copy; `requiresRecapture` (`failure-reason.ts:16-26`) never sees `recording_not_reusable:` from a render GET. `real-mismatch-redrive.test.ts:234` even fakes `failure_reason`, so tests cannot catch production.
   - **Fix:** Map `data.error` (and keep `failure_reason` if ever added). Align the test fixture with Go.

2. **BLOCKER** — `web/lib/api/jobs-index.ts:21-30`, `types.ts:264-272`, `jobs-index.test.ts:50-56` vs `job.go:41-58` and `internal/workers/media_worker.go:983-985`
   - **Problem:** `ROSTER_READY` / `PLAN_READY_STATUSES` omit `review_required`. Composition with QA warnings sets job status to `review_required`.
   - **Why:** `listableJobs` hides the partida; `getMatch` returns null (`real.ts:303-305`); `findClips` returns `[]` (`344-349`). Hub treats unknown as parsing (`hub.ts:49-53`) if the row ever appears.
   - **Fix:** Add `review_required` to both sets (and series labels). Treat it as plan-ready + roster-ready.

3. **BLOCKER** — `reel-reconcile.ts:31-33` + `real.ts:714-718` + `applyView` `830-848`
   - **Problem:** Reconcile skips `status === 'ready'`. `applyView` sets ready/review and only then attaches `downloadUrl` if `artifactNames` exist; comment admits names may arrive next tick.
   - **Why:** Ready + empty `videos` (or 404 treated as `none` then later ready without names) freezes Library/hub with no MP4. `hub.ts:100-113` maps `review_required` to output `ready`; `clips/page.tsx` then idles the poll (`anyoneWorking`).
   - **Fix:** Keep reconciling until `downloadUrl` exists (or unrecoverable). Do not mark UI-ready without `videos[0]`.

4. **WARNING** — `handlers.go:1269-1363` vs `edit-request.ts:70-72` / `renderplan/edit_request.go:65-66`
   - **Problem:** POST `/renders/{variant}` decodes `renderEditRequest` which has no `overlay_theme`; `merge` never copies it. Generate uses full `renderplan.EditRequest` (`handlers.go:1048-1051`).
   - **Why:** Full Demo theme survives first generate; music/review rerender POSTs drop `overlay_theme`.
   - **Fix:** Add `OverlayTheme *string \`json:"overlay_theme"\`` and merge it.

5. **WARNING** — `render-hydration.ts:47-93` vs `edit-request.ts:60-65`
   - **Problem:** Hydration parses family/style/code/position_y, not `keydrop_start_seconds` / `keydrop_end_seconds`.
   - **Why:** Adopted server edit loses banner timing → mismatch redrive or silent wrong intent.
   - **Fix:** Parse the two timing fields like `buildEditRequest`.

6. **WARNING** — `real.ts:264-265, 300-301, 345-346, 380, 461-462` + `mock.ts:321-338, 219-237`
   - **Problem:** Non-UUID ids (`m-upload-N`, fixture `m-*`) hit `MockApiClient`. Mock `createVideo` becomes `ready` after 10s with `SAMPLE_REEL_URL`. Mock `scanDemo` returns immediately `scanned` with synthetic roster. `listSeriesSummaries` is always `[]` (`mock.ts:303-305`). `listMatches` returns fixtures, not uploads (`289-293`).
   - **Why:** Components work on mock/design-sync and break on real UUIDs/async scan/no fixture matches.
   - **Fix:** Remove mock fallback from RealApiClient; gate fixtures behind an explicit demo mode.

7. **WARNING** — `reel-store.ts:39-43, 86-89` + `real.ts:172-197`
   - **Problem:** Library truth is `cliphub.reels.v1` (cap 50, newest win) plus in-memory `reels`/`artifactNames`/`driveLatch`. Orchestrator jobs/renders are separate.
   - **Why:** Per-user SQLite will fight this: intents vanish, covers (`selectVideoCover` is local-only `real.ts:630-651`), and drive latches will not follow the user.
   - **Fix:** Persist intents/covers server-side; treat localStorage as cache only.

8. **WARNING** — `demos/_lib.ts:104-110` vs `handlers.go:476-490`
   - **Problem:** Partidas index requests `limit=100` (Go default 50, max 100).
   - **Why:** Older uploads/series disappear from `listMatches` / `listSeriesSummaries` while still on disk.
   - **Fix:** Paginate or raise with a cursor; do not silently cap the UI index.

9. **WARNING** — `jobs-index.ts:44-49` + test `50-56`
   - **Problem:** `failed` jobs never list.
   - **Why:** User cannot delete/retry a failed scan from Partidas; series page can still show them via `getSeries`.
   - **Fix:** List failed with `failureReason` / `failure_code`.

10. **WARNING** — `map.ts:95-129` + `jobs-index.ts:110-126`
    - **Problem:** `planToMatch` sets `playedAt: new Date().toISOString()`, `score: ''`, `decentPlays: plays.length`. `jobToMatch` also forces `score: ''`, `decentPlays: 0`. Killplan drops `tick_*`, per-kill, utility, demo path/sha (`map.ts:8-12`). Roster `clan_name_*` (`parser/roster.go:45-51`) dropped by `RosterMatch` (`types.ts:325-330`). Status proxy drops `failure_code` (`_local.ts:74-88`).
    - **Why:** Wrong timestamps after reload; empty scores; lost failure taxonomy for SQLite.
    - **Fix:** Use `created_at`; map score from roster; forward `failure_code`.

11. **WARNING** — `clips/hub.ts:80-93` + `clips/page.tsx:37-41`
    - **Problem:** `settleHubSnapshot` keeps previous matches if jobs GET fails and previous videos if listVideos fails.
    - **Why:** Stale Partidas paired with fresh reels (or vice versa) after a blip — wrong rows/orphans.
    - **Fix:** Do not mix generations; retry the failed source or freeze the whole snapshot.

12. **WARNING** — `streams/route.ts:87-91`, `streams/[jobId]/route.ts:16` vs `streamclips/types.go:61-62`; editor assets GET forwards full `mediaassets.Asset` including `media_key` (`mediaassets/types.go:41-51`)
    - **Problem:** Proxies forward raw Go jobs/assets.
    - **Why:** `source_path` / `media_key` leak into the browser. TS types omit them so the leak is invisible.
    - **Fix:** Whitelist like `localJobs` / capabilities.

13. **WARNING** — `editor/projects/route.ts:13-18`, `steam/account/route.ts:16-21`
    - **Problem:** Unbounded `request.text()` / `request.json()`; other control routes use `readBoundedText`.
    - **Why:** Inconsistent 413/body-guard contract.
    - **Fix:** Use `readBoundedText` + `parseControlJSONObject`.

14. **WARNING** — `series-status.ts:25-54, 103-111`
    - **Problem:** `review_required` has no label → "analizando"; not in `SERIES_PENDING_STATUSES`; `seriesStatusIsForgeable` uses `PLAN_READY_STATUSES`.
    - **Why:** Series header buckets it pending but polling stops; map is not forgeable.
    - **Fix:** Same status-set fix as finding 2.

15. **WARNING** — `real.ts:436-440`
    - **Problem:** `getVideo` returns memory without `reconcile()`.
    - **Why:** Publish page (`clips/[id]/publicar`) can show queued/no URL until something else calls `listVideos`.
    - **Fix:** Reconcile that intent in `getVideo`.

16. **NIT** — `demos/[jobId]/parse/route.ts:21-24` vs dossier `STEAM_ID64_RE` in `_local.ts:272-296`
    - **Problem:** Parse accepts any non-empty `steamId` in JSON (not URL). SteamIDs are strings end-to-end (good vs int64).
    - **Fix:** Reuse the SteamID64 regex for consistency.

17. **NIT** — `demos/[jobId]/record/route.ts` exists; `RealApiClient.drive` POSTs `/generate` only (`real.ts:881-894`). Harmless orphan proxy.

18. **WARNING** — Go routes with no Studio proxy (UI does not call them today): `/api/loadouts`, `/api/stream-variants`, `/api/voice-profiles/*`, `/api/maps/{map}/radar`, `/api/jobs/{id}/moments`, `/compose`, `/final`, `/quality`, `/pack`, `/edit-document`, `/gallery`, `/captions`, `/revisions/*`. `/api/session/bootstrap` is Next-only.

---

## Persistence map / Contract map

### TS type ↔ Go struct (field diffs)

| TS | Go | Diff |
|---|---|---|
| `Video.status` queued\|recording\|composing\|ready\|review_required\|failed | job.Status 12 strings; render 5 strings | Client collapses recorded/composed/done/scanning/parsed into queued/recording/composing. |
| `Video.failureReason` | job `failure_reason` / render `error` | Render field name mismatch (finding 1). `failure_code` never reaches TS. |
| `Video.downloadUrl` | `videos[]` names via proxy | Client synthesizes `/api/demos/.../videos/{name}`. Ready without names possible. |
| `EditConfig` | `renderplan.EditRequest` | Go-only: `cover_first_frame`. Mixed camel/snake (`killEffect` vs `hook_text`). TS-only optional affiliate/overlay. POST render drops `overlay_theme` (finding 4). |
| `DemoPlayer.steamId` | `PlayerStat.steamid64` | Renamed in `toDemoPlayer` (`real.ts:1153-1174`). String IDs. |
| `RosterMatch` | `MatchInfo` | Drops `clan_name_ct/t`. score_ct/t → scoreCt/T. |
| `Match.playedAt/score/decentPlays` | job `created_at`; roster scores; killplan segments | Fabricated/zeroed in map/jobs-index. |
| `IndexedJob` | `job.Job` list | Proxy camelCases; drops demo_path/sha/kill_plan/rules/failure_code. |
| `CaptureProgress` | `captureProgressView` `{done,total,percent}` | Match. Percent optional in TS. |
| `Preset` | `presetSummary` | TS drops fps/effects/hq/audio/quality/covers/smoothing. |
| `Song` | `httpapi.song` | `audioUrl` → `previewUrl`. |
| `StreamJob` | `streamclips.Job` | TS omits source_path/sha256/failure_code/probe extras; proxy still forwards them. Status sets match. |
| `EditorProject` | `editorProjectJSON` | Aligned after remap. List omits `plan`. `EditorAsset` omits origin_job_id/media_key (leaked). |
| `KillPlan` (map.ts) | `killplan.Plan` | Client keeps demo.map, target, stats.total_kills_target, segment id/round/ticks/kills.weapon. Drops path/sha/tickrate/utility/positions. |

**SteamID64 / matchId:** Go share-code emits `matchId`/`outcomeId` as decimal strings (`steam.go:113-119`). TS steam helpers consume strings. Do not parse as number.

**Normalization silent defaults:** `map.ts` score `''`, playedAt now, kd=kills if deaths=0, weapon majority vote. `coerceEditConfig` / `DEFAULT_EDIT_CONFIG` punch-in/flash/short-9x16. `parseEffectiveEditConfig` returns undefined unless format/killEffect/transition/cover_strategy/intro/outro/hook_text/kill_counter all present — then match_recap/native_hud default false.

### Proxy route table

| Studio | Method | Go | ID guard | Body guard | 503 |
|---|---|---|---|---|---|
| `/api/demos/scan` | POST | `/api/jobs` | n/a | prepareLocalUpload 701MiB | yes |
| `/api/demos/jobs` | GET | `/api/jobs?limit=100` | n/a | n/a | yes |
| `/api/demos/series/{seriesId}` | GET | `/api/jobs?series_id=` | UUID | n/a | yes |
| `/api/demos/{jobId}` | DELETE | `/api/jobs/{id}` | UUID | n/a | yes |
| `/api/demos/{jobId}/status` | GET | `/api/jobs/{id}?view=status` | UUID | n/a | yes |
| `/api/demos/{jobId}/roster` | GET | `.../roster` | UUID | n/a | yes |
| `/api/demos/{jobId}/parse` | POST | `.../parse` steamId→target_steamid | UUID | bounded JSON | yes |
| `/api/demos/{jobId}/plan` | GET | `.../plan` | UUID | n/a | yes |
| `/api/demos/{jobId}/recap-plan` | GET | `.../recap-plan` | UUID | n/a | yes |
| `/api/demos/{jobId}/record` | POST | `.../record` | UUID | bounded | yes |
| `/api/demos/{jobId}/generate` | POST | `.../generate` | UUID | bounded | yes |
| `/api/demos/{jobId}/renders/{variant}` | GET/POST | same | UUID+VARIANT_RE | bounded POST | yes |
| `.../review` | POST | same | UUID+variant | bounded | yes |
| `.../videos/{name}` | GET/DELETE | same | UUID+variant+NAME_RE | n/a | yes |
| `.../covers/{name}` | GET | same | same | n/a | yes |
| `.../publish-assistant` | GET | same | same | n/a | yes |
| `.../tactical*` | GET/POST | same | UUID; round `[1-9][0-9]{0,2}` | bounded POST | yes |
| `.../anticheat` | GET/POST | same | UUID | n/a | yes |
| `.../anticheat/dossier/{steamId}` | GET | Go `{steamid}` | UUID + SteamID64 | n/a | yes |
| `/api/streams` | GET/POST | `/api/stream-jobs` | n/a | upload 2GiB / bounded JSON | yes |
| `/api/streams/{jobId}` | GET | `/api/stream-jobs/{id}` | UUID | n/a | yes |
| `.../source` | GET | `.../source` | UUID | n/a | yes |
| `.../edit-plan` | GET/PUT | same | UUID | bounded PUT | yes |
| `.../renders/{variant}` | GET/POST | same | UUID+variant | bounded POST | yes |
| `.../videos/{clipId}` | GET | same | UUID | n/a | yes |
| `.../delivery/{name}` | GET | same | UUID | n/a | yes |
| `/api/editor/assets` | GET/POST | `/api/editor/assets` | n/a | upload 2GiB | yes |
| `/api/editor/assets/import` | POST | same | n/a | bounded JSON | yes |
| `/api/editor/assets/{id}` | GET | same | UUID | n/a | yes |
| `.../media` | GET | same | UUID | n/a | yes |
| `/api/editor/projects` | GET/POST | same | n/a | POST unbounded | yes |
| `/api/editor/projects/{id}` | GET | same | UUID | n/a | yes |
| `.../plan` | GET/PUT | same | UUID | PUT unbounded at Go | yes |
| `.../preview` | POST | same | UUID | | yes |
| `.../render` | GET/POST | same | UUID | | yes |
| `.../render/video\|cover` | GET | same | UUID | | yes |
| `/api/capabilities` | GET | `/api/capabilities` | n/a | whitelist | yes |
| `/api/songs` | GET | `/api/songs` | n/a | full JSON | yes |
| `/api/songs/{id}/audio` | GET | same | SONG_ID_RE | | via proxyStream |
| `/api/presets` | GET | `/api/presets` | n/a | full JSON | yes |
| `/api/steam/*` | * | `/api/steam/*` | n/a | PUT unbounded json | yes |
| `/api/faceit/*` | * | `/api/faceit/*` | FACEIT_PLAYER_ID_RE on id routes | nickname length | yes |
| `/api/session/bootstrap` | POST | none | loopback+capability | 4KiB form | n/a |

`web/proxy.ts` matcher skips body-buffering for scan/streams/editor assets/bootstrap; those call `localAPIRequestError` in-handler.

### Client state machine

`ReelIntent` (localStorage) → `videoFromIntent` queued → `reconcileOne` GET status+render → `decideReelReconcile` → maybe POST generate/render → `applyView`. Latches: `driveLatch`, `redrivenRevisions`, `pendingCapture`, `jobGoneTicks` (2×404). `shouldReconcileVideoStatus` stops on ready. Series: `getSeries` caches `seriesMatches`; `representativeSeriesStatus` prefers forgeable then pending then failed (`series-grouping.ts:7-14`). Poll: hub 1.5s/10s via `startPollLoop`.

### Entry points (component → API)

| Caller | Calls |
|---|---|
| `app/(app)/clips/page.tsx` | `api.listMatches`, `listVideos`, `streamsApi.listJobs` |
| `clips/nueva/page.tsx` | `scanDemo`, `parseDemo`, `getScan` |
| `clips/[id]/nuevo/page.tsx` | `getMatch`, `findClips`, `findRecapClips` |
| `clips/[id]/publicar/...` | `getVideo`, `getMatch` |
| `series/[id]/page.tsx` | `getSeries`, `listVideos` |
| `streams/page.tsx` | `streamsApi.listJobs/createFromUrl/createFromFile` |
| `streams/[id]/page.tsx` | getJob, edit-plan, render, localStorage draft |
| `cheaters/page.tsx` | `listMatches`, `scanDemo` |
| `produce/short-producer.tsx` | `listPresets`, `createVideo` |
| `produce/full-pov-producer.tsx` | `createVideo` |
| `clips-hub/match-row.tsx` | `deleteMatch` |
| `clips-hub/output-item.tsx` | `retryVideo` |
| `videos/*` | delete/rerender/review/publish-assistant |
| `shell/capture-readiness.tsx` | `getCaptureReadiness` |
| `shell/shell-activity-monitor.tsx` | listVideos/listMatches/listJobs |
| `editor/editor-workspace.tsx` | editorApi + `api.listSongs` |
| `tactical-demo-picker.tsx` | `listMatches` + `fetchTacticalStatus` |

**Direct `fetch` bypassing `web/lib/api/index` (still same-origin `/api`):** `editor.ts`, `streams.ts`, `tactical.ts`, `anticheat.ts`, `faceit.ts`, `steam-account.ts`, `steam-import.ts`, `share-code-resolve.ts`, `real.ts` listSongs/listPresets. No `fetch` under `web/app/(app)` or `web/components` except those clients. Convention breach is the split clients, not page-level fetch.

### Client persistence vs SQLite

| Store | Key | Duplicates |
|---|---|---|
| localStorage | `cliphub.reels.v1` (+ legacy `fragforge.reels.v1`) | reel intents, selected covers |
| localStorage | `cliphub.stream-draft.{jobId}` | stream edit plan vs server `edit_plan` |
| sessionStorage | `cliphub.uploads.v1`, `cliphub.session.v1` | mock only |
| sessionStorage | `cliphub:sw-evicted-reload` | SW eviction flag |
| memory | `pending-upload.ts` File handoff | not durable |
| editor `plan-store` | PUT `/plan` | server project plan |

No IndexedDB. SW is unregistered (`sw-eviction.ts`), not a data store.

### Top 5 structural regression causes

1. **Two sources of truth:** localStorage intents + in-memory latches vs orchestrator jobs/renders (`reel-store.ts`, `real.ts` constructor/reconcile). Visual refactors that remount the client rehydrate intents as queued until poll.
2. **Three status vocabularies** (job / render / Video) with incomplete unions — `review_required` on the job is the smoking gun (`jobs-index.test.ts:50-56` documents the omission).
3. **Mock fallback inside RealApiClient** plus time-projected `ready` (`mock.ts:219-237`) so UI/e2e against fixtures never hit 409 roster, generate, or missing MP4.
4. **Silent map/hydrate defaults** (`map.ts` playedAt/score; `parseEffectiveEditConfig` dropping timings; POST render dropping overlay_theme) — fields vanish without error.
5. **Reconcile/poll stop conditions** (`shouldReconcileVideoStatus`, hub `anyoneWorking`, series pending set) freeze wrong UI after the first successful-looking tick. Tests named mismatch/recovery (`real-mismatch-redrive.test.ts`, `stream-recovery.ts`) cover POST loops, not ready-without-bytes or job `review_required`.

---

## Open questions

- Should Library rows live in SQLite keyed by user, or stay device-local intents? Covers are already client-only.
- Is job-level `review_required` still written in production paths besides composition (`media_worker.go:983`)? If yes, finding 2 is live.
- Confirm whether any render GET compatibility shim adds `failure_reason` (not in `renderVariantResponse`).
- `listPlanReadyMatches` has no UI caller in `web/app` / `web/components` — dead contract?

[INFERENCE] Persist this markdown as `local://audit-web.md` from the parent; this scout has no write tool.