# Audit slice: db (persistence layer)

Worktree: `C:/Users/reche/Documents/Projects/tickcut-audit`. Read-only. HTTP/frontend only cited where they write dual-truth artifacts.

## Inventory

### SQLite file: `<ZV_DATA_DIR>/jobs.db` (default `./data/jobs.db`)

Opened once in `newSQLiteJobRepository` (`cmd/zv-orchestrator/sqlite_repo.go:36-70`). Shared `*sql.DB` for stream + editor tables (`main.go:142-164`). Driver `modernc.org/sqlite`. `SetMaxOpenConns(1)` (`sqlite_repo.go:41`). Pragmas: `journal_mode=WAL`, `busy_timeout=5000`, `synchronous=NORMAL` (`sqlite_repo.go:42-45`). **No** `PRAGMA foreign_keys=ON`. **No** `PRAGMA user_version`. **No** `CREATE INDEX`. Schema = `CREATE TABLE IF NOT EXISTS` + ad hoc `PRAGMA table_info` / `ALTER TABLE`.

Path: `sqlitePath` (`config.go:66-71`) — bare `sqlite` → `<DataDir>/jobs.db`; `sqlite:<path>` uses that path **as-is** (can leave DataDir).

| Table | PK | Columns | Indexes | Created |
|---|---|---|---|---|
| `jobs` | `id TEXT` | `data BLOB NOT NULL` (full `job.Job` JSON **without** kill plan), `status TEXT NOT NULL`, `created_at INTEGER` UnixNano, `updated_at INTEGER` UnixNano | none (PK only) | `sqlite_repo.go:52-60` |
| `job_kill_plans` | `job_id TEXT` | `plan BLOB NOT NULL` | none; **no FK** to `jobs` | `sqlite_repo.go:73-80` |
| `stream_jobs` | `id TEXT` | `status`, `failure_reason`, `failure_code`, `source_path`, `source_sha256`, `source_url`, `public_source_url`, `title`, `probe TEXT`, `edit_plan TEXT`, `created_at`/`updated_at` UnixNano | none | `sqlite_stream_repo.go:33-48`; ALTER adds `public_source_url`, `failure_code` (`264-294`) |
| `editor_assets` | `id TEXT` | `sha256`, `file_name`, `origin`, `origin_job_id` (no FK), `origin_variant`, `origin_name`, `probe TEXT`, `media_key`, `created_at` UnixMilli | none; **sha256 not UNIQUE** | `sqlite_editor_repo.go:21-35` |
| `editor_projects` | `id TEXT` | `title`, `status`, `failure_reason DEFAULT ''`, `plan_json TEXT`, `created_at`/`updated_at` UnixMilli | none | `sqlite_editor_repo.go:145-156` |

Legacy `jobs.kill_plan` column: moved into `job_kill_plans` then NULLed, **column not dropped** (`sqlite_repo.go:109-150`). Embedded `data.$.kill_plan` stripped in same tx. Idempotent on second open (tested `sqlite_repo_test.go:579-783`).

`ZV_DATABASE_URL=memory`: all four repos are maps; **process death wipes them**; FS artifacts under DataDir still remain (`main.go:136-141`).

### Separate SQLite (not desktop jobs.db)

`internal/telemetry/store.go:22-50,60-151` — `telemetry_events` with CHECKs, indexes, `PRAGMA foreign_keys=ON`, `user_version` 0→1 (DROP+recreate). Owned by `services/telemetry` (`CLIPHUB_TELEMETRY_DATABASE`). Not wired in `zv-orchestrator`.

### JSON / file stores outside jobs.db (under `ZV_DATA_DIR`, default `./data`)

| Path | Owner | Holds | Writer |
|---|---|---|---|
| `.cliphub.lock` | `data_dir_lease.go:11,25-76` | PID + started_at | orchestrator startup |
| `steam/account.json` | `steamresolve/account.go:17-73,200-226` | schema v1, SteamID, auth_code, api_key, known_code, matches≤50 | AccountStore.Save/RememberCode/ReplaceMatches/Clear |
| `faceit/followed.json` | `faceit/follow.go:14-50,156-175` | schema v1, ≤20 players | Follow/Unfollow |
| `obs/journal.jsonl` | `obs/obs.go:167-261` | error events JSONL | Recorder.RecordError |
| `obs/spans.jsonl` | `obs/obs.go:224-289` | duration spans; rotated 8MiB | RecordSpan |
| `obs/metrics.json`, `obs/metrics.prom` | `obs/obs.go:227-229` | derived counters (lossy across processes) | flushLocked |
| `music/` | `config.go:95-100` | catalog audio (not a DB) | fetch scripts / user |

Wired in `main.go:76-92,112-118,173-177`.

### Filesystem artifact trees (`storage.NewLocal(cfg.DataDir)`, `internal/storage/storage.go:31-42`)

Atomic Put (temp+replace). Keys from packages:

- **Demo job** `jobs/<id>/` (`artifacts/keys.go:14-124`): `roster.json`, `recap-plan.json`, `moments/moments.json`, `anticheat.json`, `full-demo-*.json`, `generate-intent.json`, `recording/*` (result, script, segments, capture-selection/progress), `composition/final.mp4`, `tactical/*`, `render/render-progress.json`, `renders/<variant>/` (status, results, videos, logs) + revision namespaces via `renderplan`.
- **Demo file** `demos/<id>.dem` — deleted by `DeleteJob` (`httpapi/handlers.go:2196-2223`).
- **Stream job** `stream-jobs/<id>/` (`streamclips/artifacts.go:11-123`): `source.mp4`, `source-metadata.json`, `edit-plan.json`, `renders/<variant>/status.json` + revision videos/gallery/delivery.
- **Editor project** `editor-jobs/<id>/` (`timelineplan/keys.go:10-64`): `timeline.json`, `renders/status.json`, `renders/revisions/<rid>/{final.mp4,cover.jpg,render-result.json,shortslistosparasubir}`.
- **Editor assets** `editor-assets/<id>/media.mp4` (`mediaassets/types.go:105-107`).
- **Voice profiles** `voice-profiles/<id>/profile.json` + versioned audio (`voiceprofile/store.go:226-239`).

No stream-job Delete API/repo method. No editor project/asset Delete in sqlite/memory repos.

### Who writes SQLite rows

- **jobs**: HTTP Create/Delete/SetParseInputs/UpdateStatus; workers (`parser_worker.go:62-83` GetMeta+UpdateStatus+SetKillPlan); startup sweep UpdateStatus.
- **job_kill_plans**: Create, SetKillPlan, Delete, open-time migrate.
- **stream_jobs**: HTTP Create/SetEditPlan/UpdateStatus; acquire worker SetAcquired; stream render worker UpdateStatus; startup sweep.
- **editor_***: HTTP Create/SetPlan/UpdateStatus; timeline worker Get+UpdateStatus (`timeline_worker.go:88-95`).

---

## Findings

### 1. BLOCKER — `listAllDemoJobs` omits `review_required`
**path:** `cmd/zv-orchestrator/sweep.go:536-552`  
**problem:** Sweep enumerates queued/scanning/scanned/parsing/parsed/recording/recorded/composing/composed/done/failed. `job.StatusReviewRequired` exists (`internal/job/job.go:41-42,56`) and is a live compose completion status (`internal/workers/media_worker.go:983-984`). Generate/render are allowed from that status (`httpapi/handlers.go:1171-1173,1390-1392`).  
**why it matters:** After restart, queued/rendering variant `status.json` and `generate-intent.json` ActiveRunID on review_required jobs are **not** failed/idled. UI can show in-flight generate/render with no worker.  
**fix:** Add `job.StatusReviewRequired` to `listAllDemoJobs` (and any other exhaustive status walk). Add a test that seeds that status + queued render state and asserts sweep.

### 2. BLOCKER — editor projects never reconciled at startup
**path:** `startup_reconciliation.go:32-81` (demo jobs, demo renders, generate, stream only); `timelineplan/types.go:27-30` StatusRendering; `timeline_worker.go:88-95`  
**problem:** No ListByStatus/UpdateStatus sweep for `editor_projects`. Process death leaves `status=rendering` and `editor-jobs/<id>/renders/status.json` attempting.  
**why it matters:** Studio shows an active editor render that cannot complete; retry may 409.  
**fix:** Same pattern as stream: fail or restore from published revision; include editor in `reconcileInterruptedWork`.

### 3. BLOCKER — stream jobs and editor entities have no Delete
**path:** `sqlite_stream_repo.go` (no Delete); `sqlite_editor_repo.go` entire file; `httpapi/handlers.go:75-81` StreamJobRepository has no Delete; grep found no DeleteStream. Demo Delete exists (`handlers.go:2196-2223` + `sqlite_repo.go:394-409`).  
**problem:** Stream/editor rows and `stream-jobs/`, `editor-jobs/`, `editor-assets/` trees accumulate forever.  
**why it matters:** Disk growth; deleted-in-UI-but-still-listed if UI is added later; blocks per-user cleanup.  
**fix:** Idempotent Delete (row + DeleteTree) matching DeleteJob order (FS first, row last).

### 4. BLOCKER — stream edit plan dual truth (DB column vs file)
**path:** `sqlite_stream_repo.go:185-201` SetEditPlan writes `stream_jobs.edit_plan`; `httpapi/stream_handlers.go:309-334` GET prefers row then `streamclips.EditPlanKey`; `382-` Put writes SetEditPlan **then** `writeStreamEditPlanArtifact` (`792-793`). Acquire worker also writes the file (`workers/acquire_worker.go:219-220`).  
**problem:** Put can persist DB then 500 on file write; Get uses DB so UI sees new plan while disk file is stale (or reverse on acquire). currentStreamEditPlan same fallback (`356-376`).  
**why it matters:** Render admission fingerprint vs saved plan can disagree after partial failure.  
**fix:** Single authority (DB) or one tx-like write (write file then SetEditPlan, or stop writing the file). On read, do not silently prefer one side without a fingerprint check.

### 5. WARNING — editor plan dual write; GET/render use DB only
**path:** `editor_handlers.go:328-362` SetPlan then `writeEditorPlanArtifact`; `509-523` loadEditorProject decodes `p.Plan`; `timeline_worker.go:88-95` Get+Decode DB plan; `timelineplan/keys.go:14-16`  
**problem:** Same partial-failure window as stream. File `timeline.json` is not read on GET. Crash after SetPlan before file → DB new, file old.  
**why it matters:** Anything that reads the artifact (debug, future worker) diverges from Studio.  
**fix:** Drop the file, or write file first and treat DB as commit, or hash-compare.

### 6. WARNING — `jobs.data` vs columns can diverge; two update styles
**path:** `sqlite_repo.go:315-376` UpdateStatus `json_set`/`json_remove` **and** `status`/`updated_at` in one statement (atomic). `456-490` mutate (SetParseInputs) unmarshals **entire** meta blob, remarshals (drops unknown JSON keys), writes `data`+`status`+`updated_at`. GetStatus reads **column** `status` and blob `failure_reason` (`227-235`). List **orders** by columns, **returns** blob-decoded status (`250-260`, `275-280`).  
**problem:** No CHECK that `json_extract(data,'$.status') = status`. mutate vs json_set is two contracts. GetStatus vs Get can disagree if a future writer touches one side.  
**why it matters:** List order and poll `?view=status` trust columns; workbench body trusts blob.  
**fix:** One writer helper; CHECK or generated column; prefer columns for status/timestamps and stop mirroring, or always rewrite both in mutate-style with a schema version field.

Covered: UpdateStatus keeps kill plan and mirrors updated_at (`sqlite_repo_test.go:369-440`). Not covered: injected column/blob mismatch; concurrent mutate vs UpdateStatus (serialized only by MaxOpenConns(1)).

### 7. WARNING — no FKs, no indexes, foreign_keys off
**path:** `sqlite_repo.go:41-45,52-80`; contrast `telemetry/store.go:93-98`  
**problem:** `job_kill_plans.job_id` not FK; Delete must manually delete both (`394-409`) — repo does, raw SQL would orphan. List/ListByStatus/startup sweep filter `status`/`updated_at`/`json_extract(data,'$.series_id')` with table scans. editor_assets.sha256 lookup is LIMIT 1 without UNIQUE (`sqlite_editor_repo.go:69-73`).  
**why it matters:** Orphans; duplicate assets (GetBySHA256 nondeterministic); scan cost grows with library size before per-user even lands.  
**fix:** `PRAGMA foreign_keys=ON`; `job_id REFERENCES jobs(id) ON DELETE CASCADE`; UNIQUE(sha256); indexes `(status, updated_at DESC)`, `(updated_at DESC, created_at DESC)`.

### 8. WARNING — migrations not versioned; stream URL migrate not transactional
**path:** jobs migrate is one tx (`sqlite_repo.go:114-149`) — good. Stream `ensureSQLiteStreamSourceColumns` ALTER outside a version (`264-294`). `migrateSQLiteStreamSourceURLs` (`298-346`) SELECT all rows then per-row UPDATE; crash mid-loop leaves mixed secret/public URLs. `CREATE TABLE IF NOT EXISTS` will not add missing `status`/`created_at` if a pre-column DB existed without them (only kill_plan path is handled).  
**why it matters:** Non-idempotent ALTER is OK in SQLite (duplicate column errors if check races); URL migrate is not crash-safe.  
**fix:** `user_version` steps like telemetry; wrap URL migrate in a tx; fail open if required columns missing.

### 9. WARNING — MaxOpenConns(1) + ctx-cancel + long txs
**path:** `sqlite_repo.go:32-41,456-463` BeginTx(ctx) in mutate/Create/Delete/SetKillPlan; inline queue concurrency default 2 + 1 capture (`config.go` ZV_WORKER_CONCURRENCY default 2; `inline_queue.go:198-208`) plus HTTP.  
**problem:** One conn serializes HTTP+workers. Cancelled ctx aborts in-flight query; busy_timeout 5s. Long kill-plan upsert holds the only conn.  
**why it matters:** Studio poll + render worker stall as “locked”; looks like hung UI.  
**fix:** Keep MaxOpenConns(1) for SQLite correctness but never hold tx across subprocess; short statements; do not pass client ctx into multi-statement txs (use context.WithoutCancel for commit).

### 10. WARNING — memory vs SQLite: GetStatus failure_reason
**path:** memory `memory_repo.go:89-104` returns `j.FailureReason` always; sqlite `sqlite_repo.go:227-235` only when `status = failed`.  
**why it matters:** Tests on memory can pass with leftover reasons on non-failed jobs; production poll hides them (sqlite is actually stricter). Inverse: production can hide a stale reason the memory tests still see.  
**fix:** Match sqlite in memory (or both always return stored reason). Table-drive both repos.

### 11. WARNING — memory vs SQLite: ListByStatus payload + order
**path:** memory `166-178` unsorted, `cloneJob` **keeps KillPlan**; sqlite `275-280` + `scanJobs`/`decodeJobMeta` **strips KillPlan**, `ORDER BY updated_at DESC, created_at DESC`. Sweep re-sorts by ID (`sweep.go:80`) so startup is OK. Any other ListByStatus consumer is not.  
**fix:** Strip plan + sort in memory; shared contract test.

### 12. WARNING — memory aliasing / shallow clone
**path:** `cloneJob` `249-255` copies Plan struct only (segment slices shared); GetMeta `75-87` returns map value with KillPlan=nil but `Rules.Weapons` still aliases stored slice; List `119-121` same. sqlite Get/List json.Unmarshal → fresh. Editor memory Create `stored := *a` aliases `OriginJobID` pointer (`memory_editor_repo.go:37-39`).  
**why it matters:** Tests/HTTP mutating returned Rules or KillPlan.Segments corrupt the memory repo; SQLite tests would not catch it; ZV_DATABASE_URL=memory (capture lab) is production-shaped.  
**fix:** Deep-copy slices/pointers; GetMeta should cloneJob then nil plan.

### 13. WARNING — memory Create overwrites; sqlite INSERT fails
**path:** memory job/editor Create assign map key (`memory_repo.go:57`, `memory_editor_repo.go:38`); sqlite INSERT (`sqlite_repo.go:172-176`, `sqlite_editor_repo.go:53-58`). Duplicate UUID: memory silent replace, sqlite unique error (unwrapped on editor).  
**fix:** memory Create should error on existing ID; wrap sqlite errors.

### 14. WARNING — editor List nil vs empty; no ctx.Err on sqlite editor
**path:** sqlite `var out []mediaassets.Asset` (`sqlite_editor_repo.go:87-95`) nil if empty; memory `make(..., 0, ...)` (`memory_editor_repo.go:79`) empty slice. sqlite editor methods do not check `ctx.Err()`; memory does (`25-27`). JSON `null` vs `[]` at HTTP.  
**fix:** `out := []T{}`; check ctx or document QueryContext as sufficient.

### 15. WARNING — stream Create error wrapping; Get rewrites SourceURL
**path:** sqlite Create wraps ValidateSource (`sqlite_stream_repo.go:70-76`); memory returns err (`memory_repo.go:282-286`). scanSQLiteStreamJob / cloneStreamJob copy PublicSourceURL into SourceURL when private empty (`sqlite_stream_repo.go:233-238`, `memory_repo.go:438-441`).  
**why it matters:** errors.Is on validate sentinels fails on sqlite; workers must not log SourceURL after Get (it may be public overlay). Covered partly by `sqlite_stream_repo_test.go:174-228`.

### 16. WARNING — dual truth job status vs render/generate FS; authority matrix
**path:** `sweep.go:110-114` demo render sweep: “Parent job status is deliberately irrelevant”. `182-186` generate: fail job only if ActiveRunID and status parsed/recorded/recording. Stream: FS published render can promote parent to rendered (`sweep.go:453-456`); interrupted rendering fails parent unless completedJobs (`302-307`). DeleteJob: **FS first**, row last (`handlers.go:2211-2221`) — row is retry handle; FS absence is OK. Retry/enqueue uses DB status not FS.  
**Disagreement cases:** (a) review_required skipped (finding 1); (b) editor unswept (finding 2); (c) generate marker repaired but job already failed — OK; (d) job row deleted after FS delete fails — not possible with current order; (e) job row remains after FS gone if DeleteJob crashes before repo.Delete — retry  
**Authoritative:** startup in-flight **DB status** for demo/stream job rows; **FS render state** for variant completion; generate **FS ActiveRunID** plus DB status. On delete, FS then DB. On retry, DB.

### 17. WARNING — generate intent is FS-only; job status is DB
**path:** `generateintent/store.go:34-57,125-132` writes `artifacts.GenerateIntentKey`; never a SQL column. Sweep `sweep.go:186-252`.  
**why it matters:** Per-user later must move or key this file; lost if job row exists and file missing (Begin treats missing as idle).

### 18. WARNING — jobs.db can live outside leased DataDir
**path:** `config.go:66-71`; lease is DataDir only (`data_dir_lease.go:25-32`, `main.go:76-92`).  
**why it matters:** Two processes can share a sqlite file if one uses `sqlite:C:\...\jobs.db` and another DataDir; lock would not cover it.  
**fix:** Resolve sqlite path under DataDir or lease the db file too.

### 19. WARNING — steam/faceit/obs are process-global singletons
**path:** `main.go:112-118`; `account.go:57-73`; `follow.go:36-50`; `obs.go:102-110` DefaultDir `$ZV_DATA_DIR/obs`. Secrets in account.json 0600 (`account.go:219`).  
**why it matters:** Per-user persistence cannot be “add user_id to jobs” only; credentials and follows would leak across users on a shared machine.

### 20. NIT — leftover `jobs.kill_plan` column after migrate
**path:** `sqlite_repo.go:142-146` SET NULL, no DROP COLUMN. Harmless; confuses future ALTER.

### 21. NIT — telemetry is a different product surface
**path:** `services/telemetry/main.go:1-3,49`; `telemetry/store.go`. Do not put user library data there. Useful as migration template (`user_version`, FKs, indexes).

### 22. NIT — no sqlite_editor_repo_test.go
Editor sqlite behavior (nil lists, duplicate sha256, UnixMilli) is untested in orchestrator tests. Stream/job sqlite tests exist.

---

## Persistence map / Contract map

| Entity | Where | Who writes | Notes |
|---|---|---|---|
| Demo job | `jobs` row (`data` JSON + status/created/updated columns) | HTTP, workers, sweep | series_id **only in JSON** |
| Kill plan | `job_kill_plans.plan` | Create, SetKillPlan, migrate | not in `data` after migrate |
| Series | `data.$.series_id` | Create | no series table; ListBySeries json_extract |
| Stream job | `stream_jobs` columns | HTTP, acquire/render workers, sweep | no Delete |
| Stream edit plan | `stream_jobs.edit_plan` **and** `stream-jobs/<id>/edit-plan.json` | SetEditPlan + HTTP/acquire file write | dual |
| Stream render | FS `stream-jobs/<id>/renders/...` | stream worker, sweep | DB status + FS state |
| Editor project | `editor_projects` | HTTP, timeline worker | no Delete, no sweep |
| Editor timeline | `plan_json` **and** `editor-jobs/<id>/timeline.json` | SetPlan + HTTP file | GET uses DB |
| Editor render | FS `editor-jobs/<id>/renders/` | timeline worker | not swept |
| Editor asset | `editor_assets` + `editor-assets/<id>/media.mp4` | HTTP | no unique sha256, no Delete, no FK to jobs |
| Render revisions (demo) | FS under `jobs/<id>/renders/<variant>/` | render worker | parent job status independent |
| Generate intent | FS `jobs/<id>/generate-intent.json` | generateintent.Store, sweep | stripe mutex |
| Steam account | `steam/account.json` | AccountStore | **global singleton** |
| FACEIT followed | `faceit/followed.json` | FollowStore | **global singleton** |
| Telemetry | separate `telemetry_events` DB | telemetry service | not desktop |
| OBS journal | `obs/journal.jsonl` | Recorder | append-only |
| Settings | none in orchestrator SQLite | desktop telemetry `settings.json` is Electron-side, not this DB | no Studio settings table |
| Voice profiles | FS `voice-profiles/` | voiceprofile.Store | not SQLite |
| Music | `<DataDir>/music` | provisioning | global |
| Data lease | `.cliphub.lock` | acquireDataDirLease | one process per DataDir |

### Memory vs SQLite parity (one row per method)

**Job repo (`orchestratorJobRepository`)**

| Method | Parity | Divergence |
|---|---|---|
| Create | partial | Both mint ID/timestamps. Memory overwrites same ID; sqlite UNIQUE fail. Memory checks ctx first. |
| Get | partial | Both ErrNotFound. Memory cloneJob shallow (slices aliased); sqlite fresh unmarshal. |
| GetMeta | **no** | Memory does not clone (Weapons alias); sqlite decodeJobMeta. Both nil KillPlan. |
| GetStatus | **no** | Memory always returns FailureReason; sqlite only if status=failed. Segment count only while recording in both. |
| List | partial | Both cap 50/100, strip plan, order updated then created desc. Memory List does not deep-copy. sqlite `[]job.Job{}` empty non-nil. |
| ListBySeries | yes-ish | Same created_at ASC, id ASC, cap 100, strip plan, empty non-nil. Memory aliases slices. |
| ListByStatus | **no** | Memory unsorted + **keeps KillPlan**; sqlite ordered + strips plan. No cap either side. |
| UpdateStatus | partial | Both ErrNotFound. Both set FailureCode via obs.ClassOf. sqlite also json_remove codes; single SQL. Memory mutex. |
| SetParseInputs | yes-ish | Same scanned/parsed → parsing, ErrConflict. sqlite mutate full rewrite; memory field assign. |
| SetKillPlan | partial | Both ErrNotFound. sqlite tx updates updated_at + upsert plan. Memory stores pointer to copy of Plan header only. |
| Delete | yes | Both idempotent missing-id. sqlite also deletes kill plan in tx. |

**Stream repo**

| Method | Parity | Divergence |
|---|---|---|
| Create | partial | sqlite wraps ValidateSource; memory raw err. Both validate URL, split public/private. |
| Get | yes-ish | ErrNotFound. Both overlay SourceURL=Public when private empty. |
| List | yes | cap 50/100, order updated/created desc, empty `{}` non-nil. |
| ListByStatus | yes-ish | Both sort same way (unlike job memory). Uncapped. |
| UpdateStatus | yes-ish | Both clear SourceURL on failed; sqlite NULLs column; Get still shows public. FailureCode CodeFromReason then ClassOf. |
| SetEditPlan | yes | Normalize+Validate, status ready, clear failure. sqlite SQL; memory mutex. |
| SetAcquired | yes | Probe/sha256, ready, clear URL/failure, fill title if blank (sqlite trim vs strings.TrimSpace). |
| Delete | n/a | **neither implements** |

**Editor assets**

| Method | Parity | Divergence |
|---|---|---|
| Create | **no** | Memory overwrite; sqlite INSERT. Memory copies struct (OriginJobID pointer alias). sqlite UnixMilli. |
| Get | yes | ErrNotFound. |
| GetBySHA256 | partial | No unique; both first-match undefined if dupes. Memory map iteration; sqlite LIMIT 1 no ORDER. |
| List | partial | Default limit 50; **no max cap**. sqlite empty=nil; memory empty slice. Order created_at DESC both. |

**Editor projects**

| Method | Parity | Divergence |
|---|---|---|
| Create | **no** | Memory overwrite vs INSERT. Both set timestamps. sqlite UnixMilli. |
| Get | yes | ErrNotFound; memory clones Plan bytes. |
| List | partial | Default 50 no max. Order updated_at DESC. sqlite empty=nil. |
| UpdateStatus | yes | ErrNotFound via RowsAffected vs map. |
| SetPlan | yes | Marshal document; bump updated_at. |
| Delete | n/a | **neither implements** |

Tests: sqlite job+stream well covered (`sqlite_repo_test.go`, `sqlite_stream_repo_test.go`). Memory job: lifecycle/delete/series only (`memory_repo_test.go`) — **does not cover GetStatus/ListByStatus/clone**. **No sqlite_editor tests.**

### Concurrency

- Callers: HTTP handlers + inline workers (`concurrency` default 2 + serial capture) + startup (before serve).
- sqlite: one conn ⇒ no lost UpdateStatus RMW; mutate holds tx for full blob rewrite. Comment at `sqlite_repo.go:454-456` claims race-free due to single conn — true only while that stays 1.
- `job.Job` values: workers Get/GetMeta by value then UpdateStatus by ID (`parser_worker.go:62-83`) — no shared pointer across goroutines **on sqlite**. Memory repo stores one struct; returned Get/GetMeta can alias inner slices (finding 12).
- Stream: `streamclips.NewJobLocks()` in `main.go:194` for plan/render races; repo still last-writer-wins on columns.
- generateintent: 64 stripe mutexes (`generateintent/store.go:19,121-123`).

### Per-user persistence readiness

There is **no user/account/owner column or table** in jobs.db. Everything is a single-tenant library behind DataDir lease.

Must gain `user_id` (or separate DB file per user):
- `jobs`, `job_kill_plans`, `stream_jobs`, `editor_assets`, `editor_projects`
- FS prefixes `jobs/`, `demos/`, `stream-jobs/`, `editor-jobs/`, `editor-assets/`, `voice-profiles/`
- generate-intent and all render revision trees (job-scoped today, not user-scoped)

Already global singletons (need per-user files or encryption at rest):
- `steam/account.json` (secrets)
- `faceit/followed.json`
- `obs/*`
- `music/` (maybe shared catalog OK)
- `.cliphub.lock` (process, not user)

Switching `ZV_DATABASE_URL=memory` is **not** a user isolation mode; it drops job rows on restart and leaves FS orphans.

Practical sequence: (1) fix review_required + editor sweep + deletes; (2) versioned migrations + FKs; (3) collapse dual edit/timeline plans; (4) then add owner key or per-user DataDir (simplest: `ZV_DATA_DIR` per Windows account, keep schema single-tenant).

---

## Open questions

1. Is `ZV_DATABASE_URL=memory` still a supported Studio mode besides capture-lab seed (`capture_lab_seed.go:51-52`)? If yes, memory/sqlite parity is production, not test-only.
2. Should `timeline.json` / `edit-plan.json` remain after DB became source of truth, or are they debug copies?
3. Is dropping leftover `jobs.kill_plan` column in scope for a user_version=2 migrate?
4. Per-user: separate DataDir vs owner column? Lease+sqlite path currently assume one library per process.
5. Telemetry desktop `settings.json` (Electron) vs orchestrator — out of this slice; confirm no Studio UI settings belong in jobs.db.
