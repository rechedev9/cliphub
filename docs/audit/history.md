## Inventory

Worktree: `C:/Users/reche/Documents/Projects/tickcut-audit` (branch `audit/data-architecture`, HEAD `6e0779f`). Source: `git log --since=2026-06-01 --format='%h %ad %s' --date=short` (read-only).

**Filter (subject only):** starts with `fix` / `Fix` / `Fixing` (incl. `fix(scope):`) **or** subject contains `regression`, `repair`, `restore`, `stop`, `broken`, `again`. Body-`--grep` was discarded: it pulled chore/feat/merge commits whose *bodies* mentioned fix.

**No subject contained `regression` or `broken` or `again`.** `repair` only in `9ca10b9`.

### Totals

| Bucket | Count |
|---|---|
| Subject starts with fix | 144 |
| Keyword-only (not fix-prefix) | 6 |
| **Matching total** | **150** |

Keyword-only SHAs: `b48f6a1` Stop pinning assistant claim; `f20500d` Stop minting discovery secret; `0d7cd94` Stop tactical round list crushing rows; `5b3098c` refactor stop bundling unused CLI; `feb21a6` feat stop reels stalling at QUEUED; `0e023f2` Restore checksummed installer release flow.

### Exclusive primary area [INFERENCE from subject scope + file majority]

Each matching commit counted once.

| Area | Count |
|---|---|
| queue/worker (record/parse/editor/captions/keydrop/streams pipeline) | 48 |
| web component/UI | 42 |
| electron/desktop | 16 |
| release chores (landing download URLs, hooks, docker, postcss, installer gates) | 14 |
| web api client (`web/lib/api/*`) | 12 |
| httpapi handler | 10 |
| SQLite/repo | 8 |

SQLite/repo exclusive set (files actually named sqlite/memory repo or persist-on-row): `21fb958`, `cd0a4cd`, `b9b21f4`, `c550bd0`, `328cd6f`, `970742d`, `eeeb8ff`, `819c472`.

### Last-40 fix commits (subject `^fix`, merges excluded) — overlapping file areas

`9ca10b9` `0b78339` `169ca68` `5ad15a8` `20600b2` `855636d` `6c25899` `edd6c64` `21fb958` `cd0a4cd` `d71e951` `a2ffde1` `53dc00d` `5675544` `b4e454c` `6212254` `fa6a4a4` `ad19fb9` `3e500a8` `1f4d673` `4986c0c` `8b9f585` `dae80bb` `fa59ae5` `55328c1` `2316ef6` `c34ab2c` `fdfbaa4` `e22af34` `bb11a92` `b9b21f4` `ee22964` `152ee47` `1e71d0e` `661fedc` `f062b82` `5949874` `474ce1a` `dd8b44d` `7b23641`.

| Area | Commits in last 40 touching it |
|---|---|
| queue/worker | 22 |
| web component/UI | 19 |
| web api client | 9 |
| httpapi handler | 8 |
| electron/desktop | 4 |
| SQLite/repo | 3 (`21fb958`, `cd0a4cd`, `b9b21f4`) |
| release chores | 3 |

---

## Findings

### 1. WARNING — TS/Go contract drift (JSON field shape + omitted selection)

**Evidence:** `9ca10b9` `internal/httpapi/handlers.go` StartGenerate/StartRenderVariant (diff hunks ~1079–1497): `segment_ids` now required on generate intent and render task; empty still means all segments. Same commit `web/lib/api/real.ts` / `reel-reconcile.ts`: client previously re-POSTed generate whenever ready render did not match local intent.

`0b78339` `internal/httpapi/handlers.go` ~1036–1070: `music` changed from `string` to `renderMusicRequest` object (`key`/`volume`/`gameVolume`). `web/lib/api/real.ts` switched record POST to `generateUrl` and started sending `music: buildMusicRequest(intent)`.

`cd0a4cd` `internal/httpapi/handlers.go` `jobFailureCode` (~623): Spanish `failure_reason` vs structured `failure_code`; clients were expected to grep messages.

**Why it matters:** Studio and Go can both be “green” while the durable job is the wrong variant/selection/music. Persistence of per-user jobs will freeze the wrong contract.

**Fix:** One typed generate/render request object shared by handler tests and `web/lib/api/types.ts`; never accept a parallel string-or-object field.

### 2. BLOCKER — UI state derived from two sources (job status vs variant vs local reel)

**Evidence:** `9ca10b9` commit body + `web/app/(app)/clips/page.tsx` / `web/components/clips-hub/match-row.tsx`: hub treated `scanned` as parsing (`Parseando POV de -`); `matchRowStage` split `unpicked` vs `parsing`. `web/lib/api/reel-reconcile.ts` in `0b78339`: job-level `recorded`/`composed`/`done` used to mean “this Short is ready to render”; after the fix it only means *some* capture exists → `action: 'record'` so generate can validate.

`b4e454c` body: first-segment POV flake aborted capture; retry wrote all recap segments but Studio re-latched the first-fail card and stopped polling (`web/lib/api/reel-reconcile.ts`, `failed-card.tsx`).

`6212254` body + `web/lib/full-demo.ts`: pending parse painted as “Sin jugadas”; recap-plan not polled until rounds land.

**Why it matters:** Wrong state on screen is the owner’s stated regression class. Dual sources will fight any SQLite “source of truth.”

**Fix:** Hub/Library views must key off durable row fields (status + failure_code + variant state), not reconstruct from the last HTTP error and a local reel intent.

### 3. BLOCKER — SQLite schema evolved in-place (no numbered migration)

**Evidence:** `21fb958` `cmd/zv-orchestrator/sqlite_stream_repo.go`: CREATE adds `failure_code TEXT`; `ensureSQLiteStreamSourceColumns` uses `PRAGMA table_info(stream_jobs)` then `ALTER TABLE stream_jobs ADD COLUMN failure_code TEXT`. `UpdateStatus` writes `streamFailureCode(failureReason, "")` — empty stored code is re-derived from Spanish text.

`cd0a4cd` (hours earlier, same day) only derived code at scan time in sqlite + memory_repo `UpdateStatus`; did not persist a column. Two-step fix = memory vs sqlite divergence window.

`b9b21f4` `cmd/zv-orchestrator/sqlite_repo.go`: “skip JSON-null kill-plan migrate” — prior migrate assumed kill-plan JSON shape.

**Why it matters:** Per-user persistence on this pattern will ship DBs that cannot round-trip new columns; GET/list lie until ALTER runs; memory_repo tests will not catch sqlite scan/INSERT arity bugs.

**Fix:** Versioned migrations applied on boot; memory_repo and sqlite_repo must share the same UpdateStatus/scan field set in one commit.

### 4. WARNING — Queue uniqueness / race after overlay or retry

**Evidence:** `53dc00d` body: `demo_source` on the record payload made Asynq uniqueness source-scoped, so Premier→FACEIT enqueued a second HLAE capture while Library intent still occupied one full-demo slot. Source moved to a sidecar *after* unique record admission.

`b4e454c`: successful retry did not enqueue landscape compose (`internal/workers/media_worker.go` chain after `record:demo → recorded`).

July cluster `6547464` `6e09ebb` `be2dd70` `b811c15` `90e9d89`: orchestrator inline-queue recovery races (same class, older).

**Why it matters:** Duplicate captures and stuck FALLO are user-visible data/state corruption.

**Fix:** Uniqueness keys must ignore editorial overlay fields; chain-render is part of the record success path, not a client reconcile afterthought.

### 5. WARNING — Visual/nav refactor dropped callsites and assets

**Evidence:** `014b91f` (144 files, +7906/−8789): deleted `videos-page-client.tsx`, gutted `matches/[id]/page.tsx` (711 lines), `full-demo/[id]/page.tsx`, `upload/page.tsx`, `streams/page.tsx`; new `web/lib/clips/hub.ts`, produce/*, stream-editor rewrite. Follow-up **same calendar window** `9ca10b9` (59 files) had to restore: segment selection, unpicked POV resume `?job=`, delete on every row, orphan reels, settleHubSnapshot, composing percent, stream autosave/poll.

`a2ffde1`: Studio offered Jcorko chip; plate PNG was never embedded; composite aborted “plate is missing” (`internal/keydropbanner/keydropbanner.go`, `web/public/brand/keydrop/jcorko.png`).

`ad19fb9`: `fix(recording): restore demo voice in beginSoftQuit` — capture-path rewrite dropped voice.

`7b23641` / `f391d3b`: landing hero webcam dropped in a redesign.

**Why it matters:** This is the exact “visual refactor → functional regression” loop. Persistence will not help if the new UI never reads the row.

**Fix:** Treat route table + hub stage machine + generate/render POST as a contract test that must pass on the same PR as the visual cutover.

### 6. WARNING — Client poll/reconcile loops and silent autosave

**Evidence:** `9ca10b9` body: “web reconcile re-drove generate on every mismatching ready render”; latched durable admission errors; stream editor had a 300-attempt poll cap that reported a live render as failed (`web/app/(app)/streams/[id]/page.tsx` `for (;;)` replacing `attempt < 300`); autosave PUT skipped vs silent `.catch`.

`328cd6f` `fix(web,orchestrator): library poll loop survives errors`.

`a4caea2` unrecoverable-reel latch + service worker eviction.

**Why it matters:** Loops write duplicate jobs; caps lie about failure; both poison SQLite history.

**Fix:** Reconcile must be idempotent against job_id+revision; never infer failure from poll budget.

### 7. WARNING — HTTP/error mapping shown as domain state

**Evidence:** `1f4d673` `web/lib/full-demo.ts` + `full-demo/[id]/page.tsx`: 500 from `/plan` painted as «Demo no encontrada». `55328c1`: recap-plan failure lied. `661fedc`: Full Demo list failure vs empty roster. `b6362e6`: orchestrator-down reported as bad demo. `cd0a4cd`/`21fb958`: acquire class not on the job row so UI grepped Spanish.

**Why it matters:** Users (and any future per-user DB consumers) persist the wrong failure_code.

**Fix:** Handlers emit `code` + Spanish `error`; UI switches on `code` only.

### 8. NIT/WARNING — Mock vs real / UI offered, engine refused

**Evidence:** `8b9f585` / `dae80bb` touch `web/lib/api/mock.ts` + `real.ts` + `reel-brief.ts` together (9:16 vs native 1920×1080 / recap landscape). `a2ffde1` UI chip without binary. `169ca68` removed Premier/HLTV source picker after engine required FACEIT (`web/lib/full-demo.ts` `FULL_DEMO_EDIT.demoSource = FACEIT`; `handlers.go` StartRecording `writeCodedError` if not FACEIT).

**Fix:** Mock client and GenerateIntent validation must share the same EditConfig defaults.

---

## 10 most recent fix commits on `web/` | `internal/httpapi` | `cmd/zv-orchestrator`

1. **`9ca10b9`** — Root class: **contract drift + dual-source reconcile loop**. Render task had no `segment_ids` so one-kill Short encoded every recorded segment; client re-drove generate on mismatch. Hub collapsed `scanned` into parsing.
2. **`0b78339`** — **contract drift + reuse**. `StartGenerate` rejected `composed`/`done`; recap invalidate wiped *all* ready variants; music JSON was a string. Client now POSTs `/generate` instead of `/record`.
3. **`169ca68`** — **two data sources**. FACEIT overlay built from follows list, not parsed roster; store was best-effort after enqueue. UI source picker removed; generate/record now 400/422 without full FACEIT roster snapshot.
4. **`21fb958`** — **missing migration**. `failure_code` not a column; GET/list could not select by class. Added column via PRAGMA/ALTER.
5. **`cd0a4cd`** — **contract drift** (same bug, earlier). Code derived only from Spanish reason at read; memory_repo `UpdateStatus` now sets FailureCode.
6. **`a2ffde1`** — **dropped asset**. Jcorko style in UI, PNG not in composite embed → job abort.
7. **`53dc00d`** — **queue uniqueness race**. Overlay source in Asynq unique key → second HLAE capture.
8. **`5675544`** — **dropped compositor path + clock skew**. White drawtext instead of plates; voice mix not trimmed to capture duration (`internal/httpapi/full_demo_faceit.go` + worker/editor).
9. **`b4e454c`** — **dual-source latch + dropped chain**. POV flake fail latched in Studio; retry recorded rounds without compose enqueue.
10. **`6212254`** — **refactor-dropped flow seams**. Full Demo QA/read-only contract, recap-plan poll, PlayerPicker fork, Library identity — post-UX-critique holes in the pages `014b91f` later replaced.

---

## What `014b91f` broke and why `9ca10b9` existed

**`014b91f`** (2026-09-02, feat #129): nav collapsed into “Clips y vídeos” hub. Deleted or emptied Partidas/Feed/Library/Upload/Full Demo constructors (`matches/[id]/page.tsx` −711, `videos-page-client.tsx` deleted, `full-demo/[id]/page.tsx` −236, `upload/page.tsx` −594). New hub (`web/lib/clips/hub.ts`, `clips/page.tsx`, produce/*, stream editor). Desktop `main.ts` allowlist updated. **No Go generate/render contract change in that commit** (only `web/lib/api/types.ts` +10).

**Why it broke:**
- Hub staged jobs off a boolean `parsing` derived from match status, so `scanned` (no POV) looked like an infinite parse (`9ca10b9` match-row UnpickedBlock vs ParsingBlock).
- Produce page used `matchPlanReady` and fast-polled forever (`clips/[id]/nuevo/page.tsx`).
- Partial `Promise.allSettled` failures replaced the whole snapshot with `[]` (`clips/page.tsx` fetchSnapshot before `settleHubSnapshot`).
- Stream editor rewrite: poll cap, silent autosave, optimistic render state not rolled back, redundant PUTs.
- Short producer still assumed “render all recorded segments” because Go task had no selection — pre-existing, detonated when hub Shorts posted a subset.

**`9ca10b9`** (2026-09-03): patched Go + web together. That pairing is the regression signature: **UI cutover without extending the job/render row contract**.

---

## Hot spots

Last 40 fix commits, files ranked by how many of those commits touched them.

| # | File | Fix count | Sample subjects |
|---|---|---|---|
| 1 | `web/lib/api/real.ts` | 8 | 9ca10b9 render loop; 0b78339 generate reuse; b4e454c FALLO latch; 6212254 UX seams; 8b9f585 9:16 mix; dae80bb recap landscape; 53dc00d overlay unique; 152ee47 library download |
| 2 | `internal/workers/media_worker.go` | 8 | 9ca10b9 segment select; 0b78339 invalidate one variant; 169ca68 FACEIT; 5ad15a8 capture recovery; 6c25899 OOM/NVENC; 53dc00d uniqueness; 5675544 overlay/voice; b4e454c chain render |
| 3 | `web/e2e/full-demo.spec.ts` | 8 | 0b78339; 6212254; fa6a4a4; 1f4d673; fa59ae5; 55328c1; 2316ef6; 661fedc |
| 4 | `web/lib/full-demo.ts` | 7 | 169ca68 FACEIT-only; 6212254; fa6a4a4; 1f4d673 500≠missing; 8b9f585; fa59ae5 HUD copy; 55328c1 recap-plan lie |
| 5 | `web/lib/full-demo.test.ts` | 7 | same cluster as full-demo.ts |
| 6 | `internal/parser/segmentation.go` | 6 | 5ad15a8 recovery; 20600b2 POV verify; 855636d post-kill; fa6a4a4; fdfbaa4 freeze nades; 1e71d0e TickEnd |
| 7 | `internal/parser/segmentation_test.go` | 6 | same |
| 8 | `internal/httpapi/handlers.go` | 6 | 9ca10b9 segment_ids; 0b78339 generate statuses/music; 169ca68 FACEIT gate; cd0a4cd failure_code; 53dc00d sidecar; e22af34 volumes |
| 9 | `web/app/(app)/full-demo/[id]/page.tsx` | 6 | 169ca68; 6212254; fa6a4a4; 1f4d673; 55328c1; fdfbaa4 |
| 10 | `internal/workers/media_worker_test.go` | 6 | worker cluster |
| 11 | `internal/httpapi/handlers_test.go` | 5 | 9ca10b9; 0b78339; 169ca68; cd0a4cd; 53dc00d |
| 12 | `internal/recording/scriptgen.go` | 5 | 20600b2; fa6a4a4; ad19fb9 voice; 3e500a8 comms; b4e454c flake |
| 13 | `web/lib/api/types.ts` | 5 | 9ca10b9; 0b78339; 6212254; 8b9f585; 152ee47 |
| 14 | `internal/editor/ffmpeg.go` | 4 | 6c25899; edd6c64 concat; 5675544; e22af34 mix |
| 15 | `web/lib/reel-brief.ts` | 4 | 8b9f585; dae80bb; fa59ae5; bb11a92 |
| 16 | `web/components/full-demo/demo-picker.tsx` | 4 | 6212254; fa6a4a4; 2316ef6 dropzone; 661fedc list vs roster |
| 17 | `web/app/(app)/matches/[id]/page.tsx` | 4 | 6212254; 8b9f585; dae80bb; e22af34 — then deleted/gutted by 014b91f |
| 18 | `web/components/videos/ready-card.tsx` | 4 | 6212254; dae80bb; e22af34; 152ee47 |
| 19 | `internal/workers/media_worker_generate_test.go` | 4 | 0b78339; 169ca68; 53dc00d; b4e454c |
| 20 | `web/lib/api/failure-reason.ts` | 3 | 9ca10b9; b4e454c; fa6a4a4 |
| 21 | `web/lib/api/reel-reconcile.ts` | 3 | 9ca10b9; 0b78339; b4e454c |
| 22 | `web/lib/api/reel-identity.ts` | 3 | 53dc00d; dae80bb; fdfbaa4 |
| 23 | `web/components/full-demo/capture-bar.tsx` | 3 | 169ca68; 6212254; fa6a4a4 |
| 24 | `internal/httpapi/full_demo_faceit.go` | 3 | 169ca68; 53dc00d; 5675544 |
| 25 | `internal/tasks/tasks.go` | 3 | 9ca10b9; 53dc00d; e22af34 |

Also 3×: `cmd/zv-orchestrator/sqlite_stream_repo.go` only in 21fb958+cd0a4cd among last 40 (plus `b9b21f4` sqlite_repo.go).

---

## Persistence map / Contract map

```
Studio UI (hub / produce / stream editor)
  → web/lib/api/real.ts  (local reel intent + poll)
  → Next /api/demos/* proxy
  → internal/httpapi/handlers.go  (generate/record/render JSON)
  → tasks payload (segment_ids, music, edit)
  → workers/media_worker.go
  → artifacts (generate intent, render variant state, FACEIT sidecar)
  → cmd/zv-orchestrator sqlite_repo / sqlite_stream_repo / memory_repo
```

Breaks observed on this map:
- **Intent vs variant vs job.status** are three stores; UI historically picked job.status (`0b78339`, `b4e454c`, `9ca10b9`).
- **stream_jobs.failure_code** lagged Spanish `failure_reason` (`cd0a4cd` then `21fb958`).
- **Full Demo source/theme** lived in UI state then in Asynq unique key then in a sidecar (`53dc00d`, `169ca68`).
- **Kill plan** JSON-null migrate (`b9b21f4`) shows sqlite_repo cannot assume blob shape.

---

## Open questions

1. Is there a schema_migrations table at all, or only `PRAGMA table_info` + `ALTER` helpers like `ensureSQLiteStreamSourceColumns`? History of last 40 only shows the latter.
2. After `014b91f`, do Full Demo routes (`web/app/(app)/full-demo/*`) still exist as shims or only hub produce? `169ca68` still patched `full-demo/[id]/page.tsx` the same day as the hub merge — possible dead path vs live path split.
3. `memory_repo` vs `sqlite_stream_repo` field parity: `cd0a4cd` fixed memory+scan derivation; `21fb958` added the column. Any other fields (segment_ids, overlay source) still memory-only?
4. Should per-user persistence store **generate intent + render variant + hub snapshot** as one row, given every blocker here was those three disagreeing?
5. `5b819f7` (2026-07-25 v4 design system) is the previous visual cutover; many July `fix(web)` HUD commits follow it. Confirm whether 014b91f repeated that pattern without Go contract tests.
