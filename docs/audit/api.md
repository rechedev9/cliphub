## Inventory

**No `status.go`.** Status lives on `job.Job.Status` (`internal/job/job.go:21-46`), HTTP `GetJob`/`writeJobStatus` (`handlers.go:550-605`), and per-pipeline artifact JSON (render/stream/tactical/anticheat).

**Repos the handler layer actually calls** (`handlers.go:79-138`):
- `JobRepository`: Create, Get, GetStatus, List, ListBySeries, UpdateStatus, SetParseInputs, Delete. **No Save. No SetKillPlan** (workers only).
- `StreamJobRepository`: Create, Get, List, UpdateStatus, SetEditPlan, SetAcquired.
- `EditorAssetRepository` / `EditorProjectRepository`: Create/Get/List; projects also UpdateStatus, SetPlan.
- `Enqueuer`: `Enqueue` / `EnqueueWithTransition` (`inline_queue.go:211-225`).
- `storage.Storage` for demos, plans, progress, render/stream/editor artifacts.
- Side stores: FACEIT follows, Steam accounts, voice profiles, generate intents, music dir.

**Queue:** process-local `inlineQueue` in `cmd/zv-orchestrator` (not Redis). Unique lease is in-memory (`inline_queue.go:548-572`). Capture tasks (`record:demo`) are a serial lane (`inline_queue.go:83-85`). Default retries: parse/scan/anticheat/tactical = 1; everything else = 0 (`inline_queue.go:52-63`). `asynq.MaxRetry(N)` must equal that policy or enqueue fails (`inline_queue.go:243-247`).

**Workers mutate:** `UpdateStatus` / `SetKillPlan` / `SetAcquired` + artifact Puts (`internal/workers/parser_worker.go`, `media_worker.go`, `timeline_worker.go`, `acquire_worker.go`). HTTP never writes kill plans.

**Startup:** `reconcileInterruptedWork` (`startup_reconciliation.go:32-81`) fails in-flight demo jobs + render/generate artifacts + stream renders; re-enqueues acquiring stream jobs (`main.go:176-178`, `390-392`). **Editor projects are not swept.**

### Endpoint table

| Route | Handler | Repo/queue | Artifact reads | Response |
|---|---|---|---|---|
| GET /healthz | Health | none | none | `map{service,status}` |
| GET /metrics | Metrics | none | obs dir | Prometheus text; errors via `http.Error` |
| GET / | Workbench | HTML | | HTML |
| GET /ui/workspace | WorkbenchWorkspace | List | roster/moments/render | HTML |
| GET /ui/jobs | WorkbenchJobs | List | | HTML |
| GET /ui/jobs/{id} | WorkbenchJob | Get | artifacts | HTML |
| POST /ui/jobs | WorkbenchCreateJob | same as CreateJob | | HTML |
| POST /ui/jobs/{id}/parse\|record\|render\|generate | WorkbenchStart* | same as API | | HTML |
| GET /api/capabilities | GetCapabilities | steam account store | disk LookPath | anonymous map |
| GET /api/faceit/players | LookupFaceitPlayer | FACEIT API+cache | none | `{player}` |
| GET /api/faceit/players/{id}/avatar | ProxyFaceitAvatar | in-memory cache | none | image bytes |
| GET /api/faceit/players/{id}/matches | ListFaceitMatches | FACEIT API+cache | none | `{matches}` |
| GET /api/faceit/followed | ListFollowedFaceitPlayers | FollowStore | none | `{enabled,players}` |
| POST /api/faceit/followed | FollowFaceitPlayer | FollowStore.Follow | none | `{player}` |
| DELETE /api/faceit/followed/{id} | UnfollowFaceitPlayer | FollowStore.Unfollow | none | 204 |
| GET /api/loadouts | ListLoadouts | none | none | `{loadouts}` catalog |
| GET /api/presets | ListPresets | none | none | `{default,presets:[]presetSummary}` |
| GET /api/songs | ListSongs | none | **music dir / catalog JSON** | `{songs:[]song}` |
| GET /api/songs/{id}/audio | GetSongAudio | none | **audio file** | bytes |
| GET /api/stream-variants | ListStreamVariants | none | none | `{default,variants}` |
| PUT/GET/DELETE /api/voice-profiles/{id} | Put/Get/DeleteVoiceProfile | voiceprofile.Store | **profile+audio files** | profile JSON / 204 |
| GET .../audio | GetVoiceProfileAudio | store | **audio** | bytes |
| POST /api/steam/sharecode | ResolveShareCode | steam resolver; RememberCode | none | `{status,matchId,outcomeId,tokenId,demoUrl}` |
| GET/PUT/DELETE /api/steam/account | Get/Put/DeleteSteamAccount | AccountStore | **account file** | account payload map |
| POST /api/steam/matches/sync | SyncSteamMatches | AccountStore+HistoryClient | **account file** | account payload |
| POST /api/steam/import | ImportShareCode | persistAndEnqueueDemo | demo Put | `{id,status,matchId}` 201 |
| POST /api/jobs | CreateJob | Create + EnqueueWithTransition | demo Put | `{id,status}` 201 |
| GET /api/jobs | ListJobs | List or ListBySeries | none | `{jobs:[]job.Job}` (no KillPlan, no progress) |
| GET /api/jobs/{id} | GetJob | Get or GetStatus | **progress files** if capturing/rendering | `jobResponse` or `jobStatusResponse` |
| DELETE /api/jobs/{id} | DeleteJob | Delete after DeleteTree | **deletes jobs/<id> + demo** | 204 |
| GET /api/jobs/{id}/plan | GetPlan | Get | none (KillPlan in DB) | `*killplan.Plan` |
| GET /api/jobs/{id}/recap-plan | GetRecapPlan | Get | **recap-plan artifact** | recap plan |
| GET /api/jobs/{id}/roster | GetRoster | Get | **roster artifact only** | streamed JSON |
| GET /api/jobs/{id}/moments | GetMoments | Get | **moments artifact else Build(KillPlan)** | moments doc |
| POST /api/jobs/{id}/parse | StartParse | SetParseInputs + EnqueueWithTransition | none | `{id,status:parsing}` 202 |
| POST /api/jobs/{id}/anticheat | StartAnticheat | none | **anticheat doc R/W** + Enqueue | `{id,status}` 202 |
| GET /api/jobs/{id}/anticheat | GetAnticheat | Get | **anticheat artifact** | `anticheat.Document` |
| GET .../dossier/{steamid} | GetAnticheatDossier | Get | **anticheat artifact** | dossier |
| POST /api/jobs/{id}/tactical | StartTacticalAnalysis | EnqueueWithTransition | **tactical status write** | `artifacts.TacticalStatus` 202 |
| GET .../tactical | GetTacticalDocument | Get | **index JSON** | streamed |
| GET .../tactical/status | GetTacticalStatus | Get | **status JSON** | `TacticalStatus` (none if missing) |
| GET .../tactical/rounds/{n} | GetTacticalRound | Get | **index + positions blob** | `tacticalRoundResponse` |
| GET .../tactical/positions | GetTacticalPositions | Get | **positions blob** | bytes |
| GET .../tactical/aggregate | GetTacticalAggregate | Get | **index** | aggregate JSON |
| GET /api/maps/{map}/radar | GetMapRadar | none | in-process calibration | radar map |
| GET /api/jobs/{id}/final | GetFinal | Get | **final mp4** | video |
| POST /api/jobs/{id}/record | StartRecording | Enqueue Unique MaxRetry(0) | FACEIT overlay if recap | `{id,task,duplicate?}` 202 |
| POST /api/jobs/{id}/generate | StartGenerate | EnqueueWithTransition + generateIntents | render state idle check | `{id,task,variant}` 202 |
| POST /api/jobs/{id}/compose | StartComposition | Enqueue | none | `{id,task}` 202 |
| POST .../renders/{variant} | StartRenderVariant | EnqueueWithTransition Unique | **state.json R/W** | `{id,task,variant,status,status_key,accepted}` |
| GET .../renders/{variant} | GetRenderVariant | Get | **state.json + result + dir list** | `renderVariantResponse` |
| POST .../review | ResolveRenderReview | none | **state.json R/W** | `renderVariantResponse` |
| GET .../publish | GetRenderPublishBoard | Get | **state + result + Exists()** | `renderPublishBoardResponse` |
| GET .../quality | GetRenderQuality | Get | **result JSON** | quality report |
| GET .../pack\|edit-document\|gallery | GetRender* | Get | **current revision keys** | file |
| GET/DELETE .../videos/{name} | Get/DeleteRenderVideo | Get | **video+cover+caption files** | bytes / 204 |
| GET .../publish-assistant | GetPublishAssistant | Get | result + optional trends | `publishAssistantResponse` |
| GET .../covers\|captions/{name} | GetRenderCover/Caption | Get | files | bytes |
| GET .../revisions/{rev}/... | GetRenderRevision* | Get | **immutable revision keys** | bytes |
| POST /api/stream-jobs | CreateStreamJob | Create; URL path EnqueueWithTransition | source Put + default plan file | 201 upload / 202 URL |
| GET /api/stream-jobs | ListStreamJobs | List | none | `{jobs}` |
| GET /api/stream-jobs/{id} | GetStreamJob | Get | none | `streamclips.Job` |
| GET .../source | GetStreamSource | Get | **source file** | bytes |
| GET/PUT .../edit-plan | Get/PutStreamEditPlan | Get / SetEditPlan | **plan file fallback** | `EditPlan` |
| POST .../renders/{variant} | StartStreamRender | EnqueueWithTransition Unique | **render state.json** | `{id,task,variant,status}` |
| GET .../renders/{variant} | GetStreamRender | Get | **state.json else synthesize from job.Status** | `RenderState` |
| GET .../gallery\|videos\|delivery | GetStream* | Get | **state pointers or legacy keys** | files |
| POST /api/editor/assets | CreateEditorAsset | Create / GetBySHA256 | media Put | `mediaassets.Asset` 201 |
| POST /api/editor/assets/import | ImportEditorAsset | same | **demo/stream video key** | Asset 201 |
| GET /api/editor/assets | ListEditorAssets | List | none | `{assets}` |
| GET /api/editor/assets/{id} | GetEditorAsset | Get | none | Asset |
| GET .../media | GetEditorAssetMedia | Get | **media file** | bytes |
| POST /api/editor/projects | CreateEditorProject | Create | **plan artifact** | `editorProjectJSON` 201 |
| GET /api/editor/projects | ListEditorProjects | List | none | `{projects}` summaries |
| GET /api/editor/projects/{id} | GetEditorProject | Get | plan from **row JSON** | project+plan |
| GET/PUT .../plan | Get/PutEditorPlan | Get / SetPlan | **plan file dual-write** | `timelineplan.Document` |
| POST .../preview | PreviewEditorPlan | Get | none | Evaluate() |
| POST .../render | StartEditorRender | UpdateStatus in transition + Unique MaxRetry(0) | **render state file** | `timelineplan.RenderState` 202 |
| GET .../render | GetEditorRender | Get | **render state file** | RenderState |
| GET .../render/video\|cover | GetEditorRender* | Get | **keys from state file** | bytes |

**FS-not-DB (or both) to derive user-visible state:** job progress (`progress.go:36-51`), render variant status/videos (`handlers.go:1696-1904`), stream render (`stream_handlers.go:572-611`), stream edit plan fallback (`stream_handlers.go:307-360`), tactical/anticheat, roster, recap, moments, final mp4, songs, voice, editor media/render, generate intent, capture selection.

---

## Findings

1. **BLOCKER** `handlers.go:2185-2208` + `2176-2183`  
   **Problem:** `DeleteJob` only treats `queued|scanning|parsing|recording|composing` as in-flight. A job in `recorded|composed|done|review_required` can be deleted while `StartRenderVariant`/`StartGenerate` has `queued`/`rendering` in **state.json** and a live worker writing `jobs/<id>`.  
   **Why:** Deletes the artifact tree first, then the row (`2222-2231`). Worker then writes into a removed tree / missing job. User sees 204 then missing/corrupt reel.  
   **Fix:** Refuse delete if any variant state is queued/rendering or generate intent `ActiveRunID` is set; take `renderStateMu` / generate store; or cancel+wait. Include stream analog if a demo-job delete helper is reused.

2. **BLOCKER** `editor_handlers.go:376-437` vs `startup_reconciliation.go:32-81`  
   **Problem:** `StartEditorRender` transition on `decision != nil` **returns nil** (no revert). Admission writes `StatusRendering` + render-state file. Shutdown discard or handler-missing enqueue leaves project **rendering forever**. No editor sweep exists (only demo/stream). Uses `r.Context()` for `UpdateStatus` (`421`) so even a later compensate would be canceled after the HTTP request ends.  
   **Why:** Studio shows a stuck render; retry 409s `already rendering`.  
   **Fix:** On non-nil decision, `UpdateStatus(Failed or Draft)` + fail render-state using `context.Background()` like `persistJobQueueDecision` (`handlers.go:813-832`). Sweep `timelineplan.StatusRendering` at startup.

3. **BLOCKER** `handlers.go:1003` + `inline_queue.go:250-256,548-565`  
   **Problem:** Record uniqueness is SHA-256 of **payload**, not job id. `StartRecording`/`StartGenerate` can both enqueue `record:demo` for the same job if HUD/segments/recap differ. Headers (generate intent) are **not** in the unique key (`inline_queue_test.go` header-only duplicate is same payload). Same payload: generate after record is `ErrDuplicateTask` 202 **without** `Begin` intent (`handlers.go:1144-1148`) — UI thinks one-click generate started; only capture runs.  
   **Why:** Double capture on the serial lane, or generate that never chains a render.  
   **Fix:** Unique key = `jobID` (and maybe stage), independent of HUD/segments; or claim job `recording` in the same transition as enqueue.

4. **WARNING** `handlers.go:904-1024`  
   **Problem:** `StartRecording` uses `Enqueue` **without** `EnqueueWithTransition` and **does not** `UpdateStatus(recording)`. Status stays `parsed|recorded|failed` until the worker (`media_worker.go:808`). Pending unique task is invisible. Restart sweep does **not** fail `parsed` jobs (`sweep.go:67-74`), so a pending record is **lost** (OK to retry) but UI looked idle. If worker already flipped `recording` and dies, sweep fails the job (`sweep.go:35-41`).  
   **Why:** Polling `GetJob` during the queue wait shows parsed + no progress.  
   **Fix:** Transition: write `recording` (or a distinct `queued_record`) atomically with admission; compensate on discard.

5. **WARNING** `progress.go:36-51,117-154` + `handlers.go:575-605`  
   **Problem:** User progress is **files** (`CaptureProgressKey` / segment dir / `RenderProgressKey`) while status is **DB**. `?view=status` uses `GetStatus` (`sqlite_repo.go:228-252`) which only exposes `failure_reason` when `status=failed` and segment count when `recording`; `FailureCode` column/JSON is ignored (`handlers.go:292-296` uses `jobFailureCode(reason,"")`). Mid-write clips can be counted or missed (`progress.go:35`). Render progress omitted unless percent > 0 (`progress.go:144-146`). `ListJobs` never attaches progress (`461-496`).  
   **Why:** Card percent != GET job; list vs detail drift; done/recording with missing files shows status without progress or 404 on artifacts (`GetFinal` `826-855`).  
   **Fix:** Persist progress in DB or always serve the progress document; include `failure_code` in GetStatus; same DTO for list/detail.

6. **WARNING** `handlers.go:1696-1807` + `render_review_state_regression_test.go:220+`  
   **Problem:** **GET mutates** render state: legacy result → review, ready+warnings → review, writes `state.json` under `renderStateMu`. Publish board and GetRenderVariant share this.  
   **Why:** Repeated regressions around review CAS tokens (`TestPublishBoardFirstAccessMaterializesLegacyReviewToken`, `TestReviewReplacementQueueFailureRestoresExactReview`). A poll can change status the user sees from `ready` to `review` without a POST.  
   **Fix:** Migrate once in the worker/startup sweep; GET must be read-only.

7. **WARNING** `stream_handlers.go:361-379` vs `412-543` vs worker `media_worker.go:1298-1331`  
   **Problem:** `StartStreamRender` writes **file** `StatusRendering` but does **not** `streamRepo.UpdateStatus(Rendering)` until the worker claims. `PutStreamEditPlan` only blocks on **job** `StatusRendering`. PUT can land after 202, before claim; worker then fingerprint-mismatches (`validateStreamRenderIntent`). `GetStreamJob` still `ready`; `GetStreamRender` already `rendering` (or synthesized from job if file missing, `572-611`).  
   **Why:** Autosave vs render races (covered in spirit by stream revision tests).  
   **Fix:** Set job status in the same transition as render-state write; PUT must lock and refuse if state.json is rendering.

8. **WARNING** `handlers.go:416-460`  
   **Problem:** `Put(demo)` then `Create` then `EnqueueWithTransition`. Create failure → orphan `demos/<id>.dem`. Task-build error after Create → row stays **queued** until next process sweep fails it (`sweep.go:102-107`). Enqueue rejection is compensated via `persistJobQueueDecision` (good).  
   **Fix:** Single tx: create row + enqueue; delete demo on Create failure; mark failed if task build fails.

9. **WARNING** `anticheat_handlers.go:32-79`  
   **Problem:** Writes `running` document then `Enqueue` **without** transition. Enqueue failure does compensate (`68-75`). Shutdown discard does **not**. Lane stuck `running` up to `anticheatClaimTTL` 30m (`23`). Side-lane, but UI polls this as truth.  
   **Fix:** `EnqueueWithTransition` like tactical (`tactical_handlers.go:79-103`).

10. **WARNING** `editor_handlers.go:326-365` + `168-231`  
    **Problem:** `PutEditorPlan` checks `StatusRendering` **then** locks (`331-355`) — TOCTOU with `StartEditorRender` which locks first (`377`). Dual-write SetPlan then plan file; file failure → 500 with DB already updated. `ImportEditorAsset` uses `artifacts.RenderVariantVideoKey` (`149-156`) not revision-prefixed current key (`handlers.go:2321-2333`).  
    **Why:** Plan/render races; import can 404 or grab the wrong generation after revision swap (`render_revision_gallery_regression_test.go`).  
    **Fix:** Hold `editorPlanMu` before status check; import through `currentRenderVariantSnapshot`.

11. **WARNING** Error contract `handlers.go:2420-2438` vs `writeCodedError`  
    **Problem:** Default errors are `{error: string}` only. Coded errors are a minority (`faceit_not_configured`, `steam_*`, `generate_work_active`, `invalid_source_url`). 503s from Go: session token missing (`routes.go:135`) and invalid listener (`loopback.go:34`) have **no `code`**. FACEIT/Steam 503s use feature codes, not `service_unavailable`. Metrics 500 is raw text (`observability.go:26`). 409s embed `status=%s` free text (`handlers.go:761`).  
    **Why:** `503 {code:service_unavailable}` is the **Next proxy** contract when the orchestrator is down, not the Go API. Pass-through of Go 503 without `code` can be misread as “offline” vs “unconfigured”.  
    **Fix:** Keep proxy mapping; add `code` on every Go 4xx/5xx; never emit `service_unavailable` for “not configured”.

12. **WARNING** `cmd/zv-orchestrator` package boundary  
    **Problem:** SQLite/memory repos, inline queue, sweep, HTTP bind live in `cmd/zv-orchestrator` (`sqlite_repo.go`, `inline_queue.go`, `sweep.go`, `http_runtime.go`, `main.go:285-319`). `internal/httpapi` reimplements domain: review migration, publish board, FACEIT overlay, capture progress, stream plan fallback. Workers duplicate fingerprint/claim logic the handlers also do.  
    **Why:** Handler tests use `fakeRepo` that **aliases** `KillPlan` pointers (`handlers_test.go:202-211`) unlike `cloneJob` (`memory_repo.go:248-255`). Tests cannot catch mutation leaks.  
    **Fix:** Move repos/queue behind `internal/`; clone in test fakes; stop GET-side migrations.

13. **WARNING** `capabilities.go:91-96` vs `handlers.go:1195-1218`  
    **Problem:** Record/yt-dlp gated with 409. `StartComposition` does **not** check `ComposeEnabled`. Unconfigured compose enqueues work no worker consumes (the bug `requireRecordEnabled` claims to have fixed).  
    **Fix:** `requireComposeEnabled` / `requireRenderEnabled`.

14. **WARNING** Series `handlers.go:396-407,461-479` + `sqlite_repo.go:272-277`  
    **Problem:** `series_id` is a string on each job. ListBySeries returns up to 100 meta jobs, **no aggregate status**, no progress, kill plans stripped. Client must fold N independent lifecycles.  
    **Fix:** If Studio shows one series card, compute aggregate server-side with explicit rules (all failed / any recording / all done).

15. **NIT** `job.go:21-46` + handlers returning `job.Status`  
    Integer enum, JSON strings, `unknown` if out of range. Render/stream/tactical statuses are **separate string enums** on artifacts. Same English words (`ready`, `failed`, `rendering`) mean different machines.

16. **NIT** `Health` (`observability.go:14-17`) never touches SQLite. Process up + DB missing still 200.

---

## Persistence map / Contract map

```
CreateJob/ImportSteam
  storage.Put(demos/id.dem) -> repo.Create(queued) -> EnqueueWithTransition(parse|scan)
  reject/discard -> UpdateStatus(failed)

StartParse
  SetParseInputs (status=parsing, gated scanned|parsed) -> Enqueue parse
  reject/discard -> failed

StartRecording
  [no DB write] -> Enqueue Unique(payload) MaxRetry(0)
  worker: progress files -> UpdateStatus(recording) -> clips -> UpdateStatus(recorded|failed)

StartGenerate
  EnqueueWithTransition Unique(payload) -> Begin(intent file) if idle render states
  worker record then chain render Unique(variant)

StartRenderVariant
  lock renderStateMu -> write state.json queued -> Unique(job+variant) MaxRetry(0)
  discard: restore review XOR write failed (CAS vs advanced state)
  worker writes rendering/ready/review/failed on disk; job.Status usually unchanged

StartComposition
  Enqueue (no unique, no transition) -> worker composing -> composed|review_required|done

DeleteJob
  if !jobIsInFlight -> DeleteTree(jobs/id) -> Delete(demo) -> repo.Delete

Stream URL create
  Create(acquiring) -> EnqueueWithTransition MaxRetry(0) -> worker SetAcquired(ready)
  restart: re-enqueue acquiring ids (main.go recoverStreamAcquisitions)

Stream PUT plan
  lock job+streamPlanMu -> SetEditPlan(row) -> Put plan file

Stream render
  write state.json rendering -> Unique(job+variant) [intent in headers]
  worker: lock, fingerprint, UpdateStatus(rendering), commit revision keys, StatusRendered

Editor render
  EnqueueWithTransition Unique MaxRetry(0) -> UpdateStatus(rendering)+state file
  discard: NO COMPENSATION; no startup sweep
```

**Races (status mix):**
- HTTP GET status/progress while worker Puts clips / state.json.
- `done`/`composed`/`ready` in DB/state while mp4 missing → 404 (`GetFinal` `849`, render video `2336`).
- Restart: in-memory Unique gone; sweep fails `queued|scanning|parsing|recording|composing` and queued/rendering **files**; `parsed` pending-record is dropped silently.
- `MaxRetry(0)`: inline `taskIsTerminal` true on first failure (`media_worker.go:175-183`); `markFailed` uses background ctx (`109-114`). Record still does up to 2 in-process CS2 relaunches (`record_failure.go:36-39`). Unique released after handler returns (`inline_queue.go:408-412`) so Retry can enqueue.
- Series: no aggregation.

**Handler-repo contract:** Handlers do not RMW `job.Job` except `SetParseInputs` (tx `mutate`, `sqlite_repo.go:458-493`). `UpdateStatus` is `json_set` on `data` + mirror `status` column (`315-376`) — no version. SQLite `Get` joins `job_kill_plans`; List/GetStatus do not load plans. Memory `Get` clones; HTTP fake repo does not. `KillPlan == nil` is the parse/record gate (`handlers.go:665,914`) — guaranteed nil on List, present on Get after parse.

**Lost update:** render GET materialize vs POST review/correct under one mutex (OK in-process). Generate `WhileIdle` vs render enqueue (`handlers.go:1525-1528`). Stream plan vs render: incomplete (finding 7).

---

## Top 5 structural causes of regressions

Evidence from `render_review_state_regression_test.go`, `render_state_error_boundary_test.go`, `render_revision_gallery_regression_test.go`, `stream_render_revision_regression_test.go`, `startup_reconciliation_regression_test.go`, `handlers.go` size.

1. **State in three places** — `jobs.status` / stream row, `state.json` (and generate intent / tactical / anticheat docs), on-disk media. Tests exist because parent status, variant status, and files desync (completed stream parent vs file; failed rerender must keep last revision).
2. **GET as a writer** — `readOrMaterializeRenderVariantStateLocked` changes review tokens as a side effect of polling.
3. **Admission vs compensation** — only some POSTs use `EnqueueWithTransition`; editor/record/anticheat/compose do not compensate the same way. Review-restore tests encode this.
4. **Process-local queue** — Unique/retry/pending live in RAM; correctness after restart is a **sweep**, not the broker. Sweep coverage is uneven (no editor; record pending while still `parsed`).
5. **`handlers.go` ~2439 lines + stringly statuses** — job/render/stream/tactical/editor each have their own status vocabulary; invariants (delete-while-running, unique key identity, 409 vs 202 duplicate) are copy-pasted and drift.

---

## Open questions

- Should `503 {code:service_unavailable}` ever be produced by zv-orchestrator, or only by Studio proxies? Go currently uses 503 for unconfigured session/FACEIT/Steam.
- Per-user SQLite: handlers have **no user id**. All repos are single-tenant process-local. Adding users requires a tenant key on every table **and** every artifact prefix; otherwise List/Get leak across users.
- Is `parsed` + pending record supposed to be user-visible, or should the API claim `recording` at enqueue?
- Series aggregate rules (any vs all) are unspecified in the API.
- Should Health check SQLite connectivity before the desktop treats the API as ready?
