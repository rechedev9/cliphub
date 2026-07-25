# Remove Stream Killfeed And Burned Captions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the stream-clip killfeed pipeline and the stream-clip burned-in subtitle pipeline end to end, together with the xAI integration that existed only to serve them.

**Architecture:** This is a pure removal, done in layers so the tree compiles after every task: edit-plan types, then render composition, then workers, then task queue and orchestrator config, then HTTP API, then CLI, then web, then desktop, then landing and documentation.
Persisted edit plans keep loading because the removed fields simply disappear from the Go structs and JSON decoding drops unknown keys.
The demo pipeline's native killfeed and the publish-pack caption text are untouched throughout.

**Tech Stack:** Go 1.26.5, Next.js 15 / React 19 (`web/`), Electron + TypeScript (`desktop/`), Next.js (`landing/`), pnpm 11.9.0, Node 24, FFmpeg.

Source spec: `docs/superpowers/specs/2026-07-25-remove-stream-killfeed-and-captions-design.md`.

## Global Constraints

- Work directly on `main`. Never open a pull request for this work.
- Commit with `committer "message" file1 [file2 ...]`, never `git add` plus `git commit`, and never `.` as a path.
- Delete files with `trash <path>`, never `rm`, `Remove-Item`, or `del`.
- Never bypass `.githooks/pre-commit` with `--no-verify` or `core.hooksPath`. It is the only gate this repository has.
- Never add "Generated with Claude Code" or `Co-Authored-By` lines to commits.
- Bare `bash` is a broken WSL shim. Invoke shell gates as `& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh ...`.
- Run JavaScript package commands as `pnpm --dir web|desktop|landing <script>`. There is no root workspace.
- Do not add dependencies and do not run `go mod tidy`.
- Do not touch the demo pipeline's native killfeed: `internal/editor/killfeed_probe.go`, the `KillfeedSource` preset field in `internal/editor/preset.go`, `--portrait-safe-killfeed`, `PortraitSafeKillfeed`, `recording.StreamConfig`, and the `viral-60-clean` / `full-hud-60` presets all stay.
- Do not touch the publish-pack caption: `renderplan.AgentKindCaptionCandidates`, `renderplan.RenderVariantArtifactCaption`, `artifacts.RenderVariantCaptionKey`, and the `Handlers.StartCaptionAgent` / `GetCaptionAgent` / `GetRenderCaption` endpoints all stay.
- Keep `internal/mediafont`. It is still used by `internal/editor/filter.go` and `internal/streamclips/ffmpeg.go`.
- Do not edit anything under `data/`, `web/.next/`, `desktop/build-resources/`, or `desktop/dist-installer/`. Those are artifacts.
- Do not edit existing files under `docs/superpowers/specs/` or `docs/superpowers/plans/` other than this plan's own checkboxes. They are history.

---

## File Structure

Packages deleted whole:

- `internal/streamkillfeed/` — stream killfeed cue detection and frame extraction.
- `internal/killfeedvision/` — xAI vision client that read notices out of frames.
- `internal/captions/` — ASS karaoke subtitle generation, xAI speech-to-text, Spanish second pass.
- `internal/streamclips/noticeassets/` — embedded weapon icons, flags, and the notice font.

Files deleted:

- `internal/streamclips/notice_render.go`, `killfeed_analysis.go`, `killfeed_detect.go`, `caption_candidates.go`.
- `internal/streamcli/killfeed_detect.go`, `killfeed_import.go`, `captions_import.go`, `transcribe.go`.
- `internal/workers/stream_caption_worker.go`, `stream_killfeed_worker.go`, `stream_killfeed_local.go`.
- `internal/httpapi/stream_caption_handlers.go`, `stream_killfeed_handlers.go`, `stream_killfeed_analysis_handlers.go`, `stream_killfeed_fingerprint.go`.
- `desktop/src/xai-api-key.ts`, `xai-api-key-store.ts`, `xai-connection.ts`, `xai-settings-controller.ts`, `xai-settings-ipc.ts`.
- `web/components/streams/killfeed-panel.tsx`, `killfeed-kills-editor.tsx`, `use-killfeed-analysis.ts`, `captions-panel.tsx`, `caption-review-card.tsx`, `use-caption-review.ts`.
- `web/lib/killfeed-analysis.ts`, `killfeed-plan.ts`, `caption-review.ts`.
- `web/components/settings/xai-settings.tsx`.
- `web/app/api/streams/[jobId]/killfeed/`, `killfeed-read/`, `captions/`, and `web/app/api/streams/killfeed/`.
- Every `*_test.go` and `*.test.ts` file dedicated to the above.

Files trimmed but kept: `internal/streamclips/{types,ffmpeg,artifacts,edit_plan_fingerprint,gallery,variants}.go`, `internal/workers/{media_worker,stream_render_recovery}.go`, `internal/tasks/tasks.go`, `internal/httpapi/{routes,handlers,capabilities,workbench_htmx}.go`, `internal/obs/obs.go`, `internal/tuiclient/types.go`, `internal/artifacts/keys.go`, `internal/streamcli/{streamcli,support,preflight,cover}.go`, `cmd/zv/*`, `cmd/zv-orchestrator/{config,main}.go`, the web stream editor and its API client, `desktop/src/mcp/operations.ts`, `desktop/src/assistant/controller.ts`, `landing/app/{page,layout}.tsx`, and the documentation set.

---

## Task 1: Edit-Plan Types

Strip killfeed and caption data out of the durable stream edit plan, and prove that a plan file written before this change still loads.

**Files:**
- Modify: `internal/streamclips/types.go`
- Test: `internal/streamclips/legacy_plan_test.go` (create)
- Test: `internal/streamclips/clip_edit_test.go`, `killfeed_provenance_test.go` (delete the second, trim the first)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: an `streamclips.EditPlan` with no `KillfeedCrop`, `KillfeedAnalysis`, or `Captions` field, and an `streamclips.ClipRange` with no `KillfeedSeconds`, `KillfeedKills`, `KillfeedCueProvenance`, `CaptionWords`, or `CaptionReviewed` field.
  Later tasks rely on those fields being gone.

- [ ] **Step 1: Write the failing test**

Create `internal/streamclips/legacy_plan_test.go`:

```go
package streamclips

import (
	"encoding/json"
	"testing"
)

// TestLegacyPlanDropsKillfeedAndCaptions pins the compatibility promise of the
// removal: an edit plan persisted while killfeed and burned captions still
// existed keeps loading and validating, and simply renders without them.
func TestLegacyPlanDropsKillfeedAndCaptions(t *testing.T) {
	const legacy = `{
	  "schema_version": "1.0",
	  "source": {"path": "stream.mp4"},
	  "variant": "streamer-vertical-stack-40-60",
	  "killfeed_crop": {"x": 0.7, "y": 0.05, "width": 0.28, "height": 0.2},
	  "killfeed_analysis": {"generation_id": "0f1c9a24-2f21-4a7b-9a52-2ab3f1e0c111", "fingerprint": "b1946ac92492d2347c6235b4d2611184"},
	  "captions": {"enabled": true, "language": "es"},
	  "clips": [{
	    "id": "clip-001",
	    "start_seconds": 10,
	    "end_seconds": 25,
	    "killfeed_seconds": [12.5, 18.25],
	    "killfeed_kills": [[{"attacker": "a", "victim": "b", "weapon": "ak47", "attacker_side": "CT"}], []],
	    "caption_words": [{"word": "hola", "start_seconds": 11, "end_seconds": 11.4}],
	    "caption_reviewed": true
	  }]
	}`

	var plan EditPlan
	if err := json.Unmarshal([]byte(legacy), &plan); err != nil {
		t.Fatalf("decode legacy plan: %v", err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("validate legacy plan: %v", err)
	}
	if len(plan.Clips) != 1 {
		t.Fatalf("clips = %d, want 1", len(plan.Clips))
	}

	// Re-encoding must not carry the dropped keys back out to disk.
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("encode plan: %v", err)
	}
	var round map[string]any
	if err := json.Unmarshal(encoded, &round); err != nil {
		t.Fatalf("decode round trip: %v", err)
	}
	for _, key := range []string{"killfeed_crop", "killfeed_analysis", "captions"} {
		if _, present := round[key]; present {
			t.Errorf("re-encoded plan still carries %q", key)
		}
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails for the right reason**

Run: `go test ./internal/streamclips -run TestLegacyPlanDropsKillfeedAndCaptions -count=1`

Expected: FAIL, because the fields still exist and the re-encoded plan still contains `killfeed_crop`, `killfeed_analysis`, and `captions`.

- [ ] **Step 3: Delete the killfeed and caption declarations from `types.go`**

Remove these declarations entirely: `KillfeedCueOrigin` with `KillfeedCueAutomatic` and `KillfeedCueManual`, `KillfeedCueProvenance`, `KillfeedKill`, `CaptionWord`, `CaptionsPlan`, `RenderErrorCodeKillfeedArtifactsStale`, and the methods and helpers `(KillfeedKill).validate`, `killfeedSnapshotsToEvents`, `(ClipRange).KillfeedProvenanceAt`, `(EditPlan).CaptionsNeedBackend`, `normalizeKillfeedKills`, `normalizeKill`, `normalizeKillfeedSeconds`, `normalizeKillfeedPlanEntries`, `normalizeKillfeedCueProvenance`, and `mergeKillfeedKills`.

Remove these struct fields: `EditPlan.KillfeedCrop`, `EditPlan.KillfeedAnalysis`, `EditPlan.Captions`, and on `ClipRange` the `KillfeedSeconds`, `KillfeedKills`, `KillfeedCueProvenance`, `CaptionWords`, and `CaptionReviewed` fields.
Also remove `KillfeedAnalysisMetadata` if it is declared in this file.

Remove every validation and normalization branch that referenced them, including the `killfeed_crop` validation, the `killfeed_analysis` requires `killfeed_crop` rule, the per-clip cue and provenance validation loops, the index-alignment check between `KillfeedKills` and `KillfeedSeconds`, and the `legacyKillfeedSnapshots` migration keyed on `SchemaVersion == "" || SchemaVersion == "1.0"`.

Keep `SchemaVersion` itself, and keep every non-killfeed, non-caption validation exactly as it is.

- [ ] **Step 4: Delete the dedicated provenance test and trim the clip-edit test**

```powershell
trash internal/streamclips/killfeed_provenance_test.go
```

In `internal/streamclips/clip_edit_test.go`, delete any test function whose subject is killfeed cues, killfeed kills, cue provenance, caption words, or caption review.
Leave every other test untouched.

- [ ] **Step 5: Run the package tests**

Run: `go test ./internal/streamclips -count=1`

Expected: the package will still fail to build, because `ffmpeg.go`, `artifacts.go`, `killfeed_analysis.go`, `killfeed_detect.go`, `notice_render.go`, `caption_candidates.go`, `edit_plan_fingerprint.go`, `gallery.go`, and `variants.go` still reference the deleted symbols.
That is expected here and is fixed in Task 2, so do not stop to patch them one by one.
Confirm the only errors are undefined-symbol errors naming the symbols deleted in Step 3, and that none of them names a symbol you did not intend to delete.

- [ ] **Step 6: Do not commit yet**

This task leaves the package uncompilable by design, and the pre-commit hook would reject it.
Task 2 finishes the package and both tasks commit together at the end of Task 2.

---

## Task 2: Stream Render Composition

Remove killfeed overlay rendering and subtitle burning from the stream clip composer, and delete the artifact keys that only served them.

**Files:**
- Delete: `internal/streamclips/notice_render.go`, `notice_render_test.go`, `killfeed_analysis.go`, `killfeed_analysis_test.go`, `killfeed_detect.go`, `killfeed_detect_test.go`, `caption_candidates.go`, `noticeassets/` (whole directory)
- Modify: `internal/streamclips/ffmpeg.go`, `ffmpeg_test.go`, `artifacts.go`, `artifacts_test.go`, `edit_plan_fingerprint.go`, `edit_plan_fingerprint_test.go`, `gallery.go`, `gallery_test.go`, `variants.go`
- Modify: `internal/artifacts/keys.go`, `keys_test.go`

**Interfaces:**
- Consumes: the trimmed `EditPlan` and `ClipRange` from Task 1.
- Produces: `streamclips.FFmpegInputs` with no `KillfeedNoticePaths` field, and `BuildFFmpegArgs(in FFmpegInputs, plan EditPlan, clip ClipRange) ([]string, error)` with the same signature but no killfeed branch.
  The artifact key helpers `CaptionCandidatesKey`, `CaptionCandidateGenerationKey`, `KillfeedAnalysisKey`, `KillfeedAnalysisGenerationKey`, `KillfeedEventRowKey`, `RenderCaptionKey`, and `RenderRevisionCaptionKey` no longer exist.
  `artifacts.RenderVariantCaptionKey` in `internal/artifacts/keys.go` is the demo publish caption and stays.

- [ ] **Step 1: Delete the killfeed-only and caption-only files**

```powershell
trash internal/streamclips/notice_render.go internal/streamclips/notice_render_test.go internal/streamclips/killfeed_analysis.go internal/streamclips/killfeed_analysis_test.go internal/streamclips/killfeed_detect.go internal/streamclips/killfeed_detect_test.go internal/streamclips/caption_candidates.go internal/streamclips/noticeassets
```

- [ ] **Step 2: Strip the killfeed and caption paths out of `ffmpeg.go`**

Delete the `KillfeedNoticePaths` field from `FFmpegInputs`, and the exported constants `KillfeedSampleDelaySeconds` and `KillfeedNoticeHeight`.

Delete `buildKillfeedFilterGraph`, `killfeedBaseY`, `killfeedFadeFilter`, `killfeedEntranceSuppressed`, `killfeedSlideX`, `killfeedStackY`, `killfeedFreezeOffset`, the `noticeLifetime` type, and every other unexported helper in this file whose only callers were those functions.

In `BuildFFmpegArgs`, delete the `plan.KillfeedCrop == nil && len(clip.KillfeedSeconds) > 0` guard, the `len(in.KillfeedNoticePaths)` alignment guard, the notice input loop, and the `noticeInputBase` bookkeeping.

Reduce `buildFilterGraph` so it always returns `buildStandardFilterGraph(layout, plan, clip, bannerFontPath, textPaths, duration)`, then inline it into its single caller if that leaves a one-line wrapper.

Delete the subtitle burn pass and any `ass=` or `subtitles=` filter construction in this file.

- [ ] **Step 3: Strip the artifact keys**

In `internal/streamclips/artifacts.go`, delete `CaptionCandidatesKey`, `CaptionCandidateGenerationKey`, `KillfeedAnalysisKey`, `KillfeedAnalysisGenerationKey`, `KillfeedEventRowKey`, `RenderCaptionKey`, and `RenderRevisionCaptionKey`, plus any unexported helper left with no caller.

In `internal/streamclips/edit_plan_fingerprint.go`, delete every killfeed and caption contribution to the fingerprint.
The fingerprint must still change when clips, crop, layout, music, text, or audio change.

In `internal/streamclips/gallery.go`, delete the caption sidecar entries and the killfeed columns from the generated review gallery.

In `internal/streamclips/variants.go`, delete any per-variant killfeed geometry or caption safe-area constant that no longer has a consumer.
Keep the layout geometry itself.

In `internal/artifacts/keys.go`, delete only stream caption and stream killfeed keys.
Keep `RenderVariantCaptionKey`.

- [ ] **Step 4: Trim the package tests**

In `ffmpeg_test.go`, `artifacts_test.go`, `edit_plan_fingerprint_test.go`, and `gallery_test.go`, delete every test function and golden expectation covering killfeed overlays, notice rendering, caption burning, or the deleted keys.
Update any remaining golden FFmpeg argument string that previously included killfeed or subtitle filters so it matches the new output.

- [ ] **Step 5: Build and test the package**

Run: `go build ./internal/streamclips/... && go test ./internal/streamclips ./internal/artifacts -count=1`

Expected: PASS, including `TestLegacyPlanDropsKillfeedAndCaptions` from Task 1.

- [ ] **Step 6: Commit Tasks 1 and 2 together**

```powershell
committer "Remove killfeed and burned captions from the stream edit plan and composer" internal/streamclips internal/artifacts
```

Note: `committer` rejects `.` but accepts explicit directory paths.
If the pre-commit hook rejects the commit because other packages no longer build, that is expected at this point only if the hook builds the whole module.
In that case, continue to Task 3 and make this one commit cover Tasks 1 through 3 instead.

---

## Task 3: Workers And Removed Backend Packages

Delete the three backend packages and the three stream workers, and cut their call sites out of the media worker.

**Files:**
- Delete: `internal/captions/` (whole), `internal/killfeedvision/` (whole), `internal/streamkillfeed/` (whole)
- Delete: `internal/workers/stream_caption_worker.go`, `stream_killfeed_worker.go`, `stream_killfeed_worker_test.go`, `stream_killfeed_local.go`, `stream_killfeed_local_test.go`, `stream_killfeed_artifact_test.go`
- Modify: `internal/workers/media_worker.go`, `stream_render_recovery.go`, `stream_render_recovery_test.go`, `stream_render_attempt.go`, `stream_render_attempt_regression_test.go`, `stream_render_intent_test.go`, `stream_render_revision_cleanup_regression_test.go`, `stream_worker_test.go`, `media_worker_test.go`, `media_worker_segment_select_test.go`, `obs.go`
- Modify: `internal/obs/obs.go`

**Interfaces:**
- Consumes: the trimmed `streamclips` package from Task 2.
- Produces: `workers.StreamRenderWorkerConfig` with no caption or killfeed fields, and no exported `PrepareLocalKillfeedAnalysis` method on the stream render worker.
  Task 6 relies on `PrepareLocalKillfeedAnalysis` being gone.

- [ ] **Step 1: Delete the packages and worker files**

```powershell
trash internal/captions internal/killfeedvision internal/streamkillfeed internal/workers/stream_caption_worker.go internal/workers/stream_killfeed_worker.go internal/workers/stream_killfeed_worker_test.go internal/workers/stream_killfeed_local.go internal/workers/stream_killfeed_local_test.go internal/workers/stream_killfeed_artifact_test.go
```

- [ ] **Step 2: Cut the caption and killfeed machinery out of `media_worker.go`**

Delete these functions and methods: `transcribeCaptionsWithXAI`, `renderClipKillfeedNotices`, `writeKillfeedNoticePNG`, `(*StreamRenderWorker).extractCaptionSourceAudio`, `(*StreamRenderWorker).extractSpeechEnhancedCaptionAudio`, `captionRecoveryWindows`, the `captionRecoveryWindow` type, `(*StreamRenderWorker).extractCaptionRecoveryWindow`, `(*StreamRenderWorker).recoverCaptionTranscript`, `normalizeRecoveredCaptionCues`, `captionTranscriptLooksPartial`, `betterCaptionTranscript`, `(*StreamRenderWorker).transcribeCaptionCues`, `(*StreamRenderWorker).burnCaptionCues`, `(StreamRenderWorkerConfig).captionsConfigured`, and `(*StreamRenderWorker).PrepareLocalKillfeedAnalysis`.

Delete the `internal/captions`, `internal/killfeedvision`, and `internal/streamkillfeed` imports.

In the stream render path, delete the step that renders notice PNGs and passes `KillfeedNoticePaths` into `streamclips.FFmpegInputs`, and delete the step that transcribes, validates, burns, and publishes `.ass` caption artifacts.
Delete the `errStreamKillfeedArtifactsStale` sentinel and every branch that returns or wraps it.

Delete the caption and killfeed fields from `StreamRenderWorkerConfig`, including the xAI key, the Whisper model paths, the VAD model path, and any caption-related timeouts.

Keep `(*RecordWorker).record` and `normalizedRecordingStream` exactly as they are: they are the demo pipeline.

- [ ] **Step 3: Simplify stream render recovery**

In `internal/workers/stream_render_recovery.go`, delete the `errors.Is(cause, errStreamKillfeedArtifactsStale)` branch and the `streamclips.RenderErrorCodeKillfeedArtifactsStale` assignment in `writeRecoverableStreamRenderState`.
If that leaves the function with no recoverable error codes at all, keep the function and its generic path rather than deleting it, so the render state shape stays stable.

In `stream_render_attempt.go`, delete any caption or killfeed generation ID and fingerprint that the attempt record carries.

- [ ] **Step 4: Drop the dead observability labels**

In `internal/obs/obs.go`, delete the `stage` and `class` label constants that only the removed caption and killfeed failure paths emitted.
Leave every label that any surviving path still records.

- [ ] **Step 5: Trim the worker tests**

In `media_worker_test.go`, `media_worker_segment_select_test.go`, `stream_worker_test.go`, `stream_render_recovery_test.go`, `stream_render_attempt_regression_test.go`, `stream_render_intent_test.go`, and `stream_render_revision_cleanup_regression_test.go`, delete every test function whose subject is caption transcription, caption burning, caption recovery, killfeed analysis, killfeed notices, or killfeed artifact staleness.
Trim the fixtures the remaining tests build so they no longer set the removed config or plan fields.

- [ ] **Step 6: Build and test**

Run: `go build ./... && go test ./internal/workers ./internal/obs -count=1`

Expected: `go build ./...` still fails in `internal/httpapi`, `internal/streamcli`, `cmd/zv`, and `cmd/zv-orchestrator`, which later tasks fix.
`go test ./internal/workers ./internal/obs -count=1` must PASS.

- [ ] **Step 7: Commit**

```powershell
committer "Remove stream caption and killfeed workers and their backends" internal/captions internal/killfeedvision internal/streamkillfeed internal/workers internal/obs
```

If the pre-commit hook rejects this because the module does not build yet, fold Tasks 3 through 6 into a single commit taken at the end of Task 6, and note that in the task list.

---

## Task 4: Task Queue And Orchestrator Configuration

Remove the two Asynq task types and the xAI credential the orchestrator threaded through them.

**Files:**
- Modify: `internal/tasks/tasks.go`, `internal/tasks/tasks_test.go`
- Modify: `cmd/zv-orchestrator/config.go`, `config_test.go`, `main.go`, `inline_queue_test.go`, `stream_e2e_test.go`

**Interfaces:**
- Consumes: the trimmed workers from Task 3.
- Produces: no `tasks.TypeGenerateStreamCaptions`, `tasks.TypeGenerateStreamKillfeed`, `tasks.NewGenerateStreamCaptionsTask`, `tasks.NewGenerateStreamKillfeedTask`, `tasks.StreamCaptionGenerationFromTask`, or `tasks.StreamKillfeedGenerationFromTask`.
  Task 5 relies on those being gone.

- [ ] **Step 1: Strip `internal/tasks/tasks.go`**

Delete `TypeGenerateStreamCaptions`, `TypeGenerateStreamKillfeed`, `streamCaptionGenerationHeader`, `streamKillfeedGenerationHeader`, `GenerateStreamCaptionsPayload`, `GenerateStreamKillfeedPayload`, `NewGenerateStreamCaptionsTask`, `NewGenerateStreamKillfeedTask`, `StreamCaptionGenerationFromTask`, `StreamKillfeedGenerationFromTask`, and the `KillfeedGeneration` and `KillfeedFingerprint` fields plus their validation on the stream render intent payload.

Keep `PortraitSafeKillfeed`, `NewRecordDemoTask`, `NewGenerateRecordDemoTask`, and `newRecordDemoTask`: they belong to the demo pipeline.
Keep `sha256HexPattern` only if another validation still uses it.

- [ ] **Step 2: Strip the xAI credential**

In `cmd/zv-orchestrator/config.go`, delete `XAIAPIKey`, `xaiAPIKeyEnvironmentVariable`, `clearXAIAPIKeyEnvironment`, `(config).xaiEnabled`, and the `XAIEnabled` value passed into `httpapi.Capabilities`.
If `clearEnvironmentVariable` has no other caller, delete it too.

In `cmd/zv-orchestrator/main.go`, delete the registration of the `stream:captions` and `stream:killfeed` handlers, the xAI key wiring into `StreamRenderWorkerConfig`, and the call to `clearXAIAPIKeyEnvironment`.

- [ ] **Step 3: Trim the tests**

In `internal/tasks/tasks_test.go` and `cmd/zv-orchestrator/config_test.go`, delete the tests for the removed task constructors, headers, payload validation, and the xAI environment variable.

In `cmd/zv-orchestrator/inline_queue_test.go` and `stream_e2e_test.go`, delete the caption and killfeed stages from the exercised flow so the end-to-end path is acquire, plan, render.

- [ ] **Step 4: Build and test**

Run: `go build ./internal/... ./cmd/zv-orchestrator/... && go test ./internal/tasks ./cmd/zv-orchestrator -count=1`

Expected: the build still fails in `internal/httpapi` until Task 5.
Run `go test ./internal/tasks -count=1` on its own and expect PASS; defer the orchestrator tests to Task 5.

- [ ] **Step 5: Commit**

```powershell
committer "Remove stream caption and killfeed tasks and the xAI credential" internal/tasks cmd/zv-orchestrator
```

---

## Task 5: HTTP API

Delete the stream caption and killfeed endpoints, their handlers, and the xAI capability gate.

**Files:**
- Delete: `internal/httpapi/stream_caption_handlers.go`, `stream_caption_handlers_test.go`, `stream_killfeed_handlers.go`, `stream_killfeed_handlers_test.go`, `stream_killfeed_analysis_handlers.go`, `stream_killfeed_analysis_handlers_test.go`, `stream_killfeed_fingerprint.go`, `stream_killfeed_fingerprint_test.go`, `stream_render_revision_regression_test.go` (only if its whole subject is killfeed revisions; otherwise trim it)
- Modify: `internal/httpapi/routes.go`, `handlers.go`, `handlers_test.go`, `handlers_generate_test.go`, `capabilities.go`, `capabilities_test.go`, `workbench_htmx.go`, `stream_handlers.go`, `middleware.go`
- Modify: `internal/tuiclient/types.go`, `internal/tuiclient/client_test.go`

**Interfaces:**
- Consumes: the trimmed tasks package from Task 4.
- Produces: an `httpapi.Capabilities` struct with no `XAIEnabled` field, and a router with no `/api/stream-jobs/{id}/captions`, `/captions/review`, `/killfeed`, `/killfeed/apply`, `/killfeed-read`, `/api/stream-killfeed/weapons`, or `/api/stream-killfeed/notice-preview` routes.
  Task 7 and Task 8 rely on those routes being gone.

- [ ] **Step 1: Delete the handler files**

```powershell
trash internal/httpapi/stream_caption_handlers.go internal/httpapi/stream_caption_handlers_test.go internal/httpapi/stream_killfeed_handlers.go internal/httpapi/stream_killfeed_handlers_test.go internal/httpapi/stream_killfeed_analysis_handlers.go internal/httpapi/stream_killfeed_analysis_handlers_test.go internal/httpapi/stream_killfeed_fingerprint.go internal/httpapi/stream_killfeed_fingerprint_test.go
```

- [ ] **Step 2: Delete the routes**

In `internal/httpapi/routes.go`, delete these seven registrations:

```go
r.Get("/api/stream-killfeed/weapons", h.ListStreamKillfeedWeapons)
r.Post("/api/stream-killfeed/notice-preview", h.PreviewStreamKillfeedNotice)
r.Post("/api/stream-jobs/{id}/captions", h.StartStreamCaptionCandidates)
r.Get("/api/stream-jobs/{id}/captions", h.GetStreamCaptionCandidates)
r.Post("/api/stream-jobs/{id}/captions/review", h.ReviewStreamCaptionCandidates)
r.Post("/api/stream-jobs/{id}/killfeed", h.StartStreamKillfeedAnalysis)
r.Get("/api/stream-jobs/{id}/killfeed", h.GetStreamKillfeedAnalysis)
r.Post("/api/stream-jobs/{id}/killfeed/apply", h.ApplyStreamKillfeedAnalysis)
r.Post("/api/stream-jobs/{id}/killfeed-read", h.ReadStreamKillfeed)
```

Also delete `r.Post("/ui/jobs/{id}/agent/captions", h.WorkbenchStartCaptionAgent)` only if that handler targets streams.
Verify first: if it dispatches `renderplan.AgentKindCaptionCandidates` for a demo render, it is the publish caption and must stay.

Keep `/api/jobs/{id}/renders/{variant}/agent/captions` and `/api/jobs/{id}/renders/{variant}/captions/{name}`.

- [ ] **Step 3: Strip `handlers.go` and `capabilities.go`**

In `internal/httpapi/handlers.go`, delete the `killfeedFrame` and `killfeedTimeline` function fields on `Handlers`, their default assignments in the constructor, the `extractKillfeedFrame` and `extractKillfeedTimeline` methods, and the `timedKillfeedRows` type.

In `internal/httpapi/capabilities.go`, delete the `XAIEnabled` field, the `captionsEnabled` method, the `xai_enabled` entry in the capabilities JSON, and the `requireCaptionBackend` guard that writes `409` with the "generating caption candidates needs xAI" message.

In `internal/httpapi/stream_handlers.go`, delete the caption and killfeed fields from the stream job response payloads and any PUT edit-plan validation that referenced them.

In `internal/httpapi/workbench_htmx.go` and `middleware.go`, delete only the stream caption and killfeed references.

In `internal/tuiclient/types.go`, delete the caption and killfeed fields from the mirrored API types.

- [ ] **Step 4: Trim the tests**

Delete every test function in `handlers_test.go`, `handlers_generate_test.go`, `capabilities_test.go`, and `client_test.go` that exercises a removed route, the xAI gate, or a removed payload field.
Inspect `stream_render_revision_regression_test.go`: if every case is about killfeed generation revisions, delete the file with `trash`; otherwise delete only those cases.

- [ ] **Step 5: Build and test**

Run: `go build ./... && go test ./internal/httpapi ./internal/tuiclient ./cmd/zv-orchestrator -count=1`

Expected: the build still fails in `internal/streamcli` and `cmd/zv` until Task 6.
Run `go test ./internal/httpapi ./internal/tuiclient -count=1` on its own and expect PASS.

- [ ] **Step 6: Commit**

```powershell
committer "Remove stream caption and killfeed HTTP endpoints and the xAI gate" internal/httpapi internal/tuiclient
```

---

## Task 6: CLI

Reduce the stream flow to `variants -> plan -> render` and drop the removed flags, stages, and capability fields.

**Files:**
- Delete: `internal/streamcli/killfeed_detect.go`, `killfeed_import.go`, `captions_import.go`, `transcribe.go`, `transcribe_test.go`
- Modify: `internal/streamcli/streamcli.go`, `streamcli_test.go`, `support.go`, `preflight.go`, `preflight_test.go`, `cover.go`, `stream_journey_test.go`
- Modify: `cmd/zv/usage.go`, `flow_commands.go`, `flow_commands_test.go`, `flow_run.go`, `flow_run_test.go`, `flow_validation_test.go`, `workflow_catalog.go`, `workflow_docs.go`, `workflow_entrypoints.go`, `command_validation.go`, `check_config.go`, `capabilities_command.go`, `capabilities_command_test.go`, `app_flow_stream_e2e_test.go`, `app_stage_contract_test.go`, `app_test_support_test.go`, `app_workflows_e2e_test.go`, `short_command.go`, `short_commands_test.go`, `skills_commands_test.go`

**Interfaces:**
- Consumes: the trimmed `streamclips` and `workers` packages.
- Produces: a `zv stream` command that accepts only `variants`, `plan`, and `render`; a `streamService` interface with no `DetectKillfeed` and no `Transcribe` method; and a `zv capabilities --format json` payload whose `stream` object has no `killfeed_detection_ready`, `spanish_captions_ready`, `captions_provider`, or `captions_configuration` keys.

- [ ] **Step 1: Delete the stage files**

```powershell
trash internal/streamcli/killfeed_detect.go internal/streamcli/killfeed_import.go internal/streamcli/captions_import.go internal/streamcli/transcribe.go internal/streamcli/transcribe_test.go
```

- [ ] **Step 2: Reduce the `zv stream` dispatcher**

In `internal/streamcli/streamcli.go`, delete the `"killfeed"`, `"transcribe"`, and `"captions"` cases from the subcommand switch, and the `runStreamKillfeed`, `runStreamTranscribe`, and `runStreamCaptions` references.

Delete `DetectKillfeed` and `Transcribe` from the `streamService` interface, the `localStreamService` implementations, the `streamTranscribeRequest` and `streamTranscriptReview` types, and the `CaptionsPath` field on the render result struct.

In the `stream plan` flag set, delete `--killfeed-crop`, `--detect-killfeed`, and `--captions`, together with the crop parsing, the `--detect-killfeed requires --killfeed-crop` error, the cue detection call, the provenance construction, and the `plan.Captions = ...` assignment.
Delete `--ffmpeg` only if killfeed detection was its only consumer; if `stream plan` still probes media with FFmpeg, keep it.

In the `stream render` path, delete the `plan.CaptionsNeedBackend()` readiness error, `streamPlanNeedsExactKillfeedArtifacts`, and the `PrepareLocalKillfeedAnalysis` call along with the `RequireAppliedKillfeedAnalysis` field it set.
Delete the block that copies `.ass` caption artifacts into the publish pack.

- [ ] **Step 3: Simplify the cover timestamp**

In `internal/streamcli/cover.go`, delete `killCountAt` and reduce `streamCoverTimestamp` to the existing fallback so it always returns the clamped `renderedDuration * 0.35`:

```go
// streamCoverTimestamp picks a stable, non-black frame from the rendered clip.
// The first third of the clip avoids both the opening fade and the tail.
func streamCoverTimestamp(plan streamclips.EditPlan, clipID string, renderedDuration float64) float64 {
	return clampStreamCoverTimestamp(renderedDuration*0.35, renderedDuration)
}
```

If `plan` and `clipID` become unused, drop them from the signature and update the single call site.
Delete `streamCoverAfterKillSeconds`.

- [ ] **Step 4: Update the usage strings**

In `internal/streamcli/support.go`, delete `streamKillfeedUsage`, `streamTranscribeUsage`, and `streamCaptionsUsage`, and delete the caption sentence from the render usage text.

In `cmd/zv/usage.go`, delete the `zv stream killfeed`, `zv stream transcribe`, and `zv stream captions` lines, drop those three names from `streamUsage`, and remove `[--captions]` and the killfeed flags from the `zv stream plan` line.
Remove `--events`, `--killfeed-crop`, and `--words` from the flag documentation block.

- [ ] **Step 5: Update the flow and workflow catalogs**

In `cmd/zv/flow_commands.go`, in the `stream` flow: change `Description` to `"stream/VOD clips"`, change the `doctor` stage goal to `"verify FFmpeg readiness"`, remove the killfeed and caption clauses from the `creative-brief` `Decision` string, remove the killfeed and caption flags and decisions from `plan-preflight` and `plan`, delete the `killfeed-preflight`, `enrich`, `transcribe-preflight`, `transcribe`, `captions-preflight`, and `captions` stages, change the `render` goal to `"render video, audio, cover, manifest, and gallery"`, and change the `review` goal to drop "sidecar captions".

Leave the `demo` flow's killfeed wording alone: it describes HLAE capture.

In `cmd/zv/workflow_catalog.go`, delete the `stream-killfeed`, `stream-transcribe`, and `stream-captions` entries, remove those names from the two `case` lists, and change the `stream-render` description to `"Render stream clips into an upload-ready local pack."`.

In `cmd/zv/workflow_docs.go`, `workflow_entrypoints.go`, `command_validation.go`, and `check_config.go`, delete the corresponding entries so the catalog check passes.

- [ ] **Step 6: Trim the capabilities command**

In `cmd/zv/capabilities_command.go`, delete `KillfeedDetectionReady`, `SpanishCaptionsReady`, `CaptionsProvider`, `CaptionsConfiguration`, the `XAI_API_KEY` probe, and the two `fmt.Fprintf` lines that print them.

- [ ] **Step 7: Trim the CLI tests**

Delete every test case in `streamcli_test.go`, `preflight_test.go`, `stream_journey_test.go`, `flow_commands_test.go`, `flow_run_test.go`, `flow_validation_test.go`, `capabilities_command_test.go`, `app_flow_stream_e2e_test.go`, `app_stage_contract_test.go`, `app_workflows_e2e_test.go`, `skills_commands_test.go`, and `short_commands_test.go` that names a removed stage, flag, workflow, or capability field.
Update `app_test_support_test.go` fixtures so the stream journey is variants, plan, render.

Add one case to `streamcli_test.go` proving the removed render gate is gone:

```go
func TestStreamRenderAcceptsAudibleClipWithoutCaptions(t *testing.T) {
	// A plan whose clip has audio and no caption data used to be rejected by the
	// Spanish-caption readiness gate. Rendering must now accept it.
	plan := newTestEditPlanWithAudio(t)
	planPath := writeTestPlan(t, plan)

	var stdout, stderr bytes.Buffer
	code := runStream([]string{"render", "--input", testMediaPath(t), "--plan", planPath, "--out", t.TempDir(), "--dry-run", "--format", "json"}, &stdout, &stderr, newTestStreamService(t))
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
}
```

Adapt the helper names to whatever `streamcli_test.go` already provides rather than inventing new helpers.

- [ ] **Step 8: Build and run the whole Go gate**

Run: `& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --no-format --build`

Expected: PASS. The module builds, `go vet` is clean, `zv check` accepts the trimmed catalog, and every Go test passes.

- [ ] **Step 9: Commit**

```powershell
committer "Reduce the zv stream flow to variants, plan and render" internal/streamcli cmd/zv
```

---

## Task 7: Web UI

Remove the killfeed and captions panels, their API clients and proxy routes, and the xAI settings card.

**Files:**
- Delete: `web/components/streams/killfeed-panel.tsx`, `killfeed-kills-editor.tsx`, `use-killfeed-analysis.ts`, `captions-panel.tsx`, `caption-review-card.tsx`, `use-caption-review.ts`, `web/components/settings/xai-settings.tsx`
- Delete: `web/lib/killfeed-analysis.ts`, `killfeed-analysis.test.ts`, `killfeed-plan.ts`, `killfeed-plan.test.ts`, `caption-review.ts`, `caption-review.test.ts`, `lib/api/streams-killfeed-analysis.test.ts`, `lib/api/streams-captions.test.ts`
- Delete: `web/app/api/streams/[jobId]/killfeed/`, `web/app/api/streams/[jobId]/killfeed-read/`, `web/app/api/streams/[jobId]/captions/`, `web/app/api/streams/killfeed/`
- Modify: `web/components/streams/stream-editor.tsx`, `render-bar.tsx`, `render-stage.tsx`, `preview-column.tsx`, `crop-picker.tsx`, `analysis-progress.tsx`, `stream-preview.tsx`
- Modify: `web/lib/api/streams.ts`, `lib/api/real.ts`, `lib/api/mock.ts`, `lib/streams/plan.ts`, `lib/streams/plan.test.ts`, `lib/stream-recovery.ts`, `lib/stream-recovery.test.ts`, `lib/stream-preview.ts`, `lib/stream-preview.test.ts`, `lib/reel-brief.ts`, `lib/reel-brief.test.ts`, `lib/desktop-settings.ts`, `lib/desktop-settings.test.ts`, `app/(app)/settings/page.tsx`, `app/(app)/streams/page.tsx`, `app/api/streams/[jobId]/edit-plan/route.ts`

**Interfaces:**
- Consumes: the API surface from Task 5. Any client call to a deleted route must go.
- Produces: a `StreamEditPlan` type in `web/lib/api/streams.ts` with no `killfeed_crop`, `killfeed_analysis`, or `captions` properties, and a `StreamClip` type with no `killfeed_seconds`, `killfeed_kills`, `caption_words`, or `caption_reviewed` properties.

Read `web/CLAUDE.md` before starting, and `web/design.md` before touching layout.

- [ ] **Step 1: Delete the components, libs, and proxy routes**

```powershell
trash web/components/streams/killfeed-panel.tsx web/components/streams/killfeed-kills-editor.tsx web/components/streams/use-killfeed-analysis.ts web/components/streams/captions-panel.tsx web/components/streams/caption-review-card.tsx web/components/streams/use-caption-review.ts web/components/settings/xai-settings.tsx web/lib/killfeed-analysis.ts web/lib/killfeed-analysis.test.ts web/lib/killfeed-plan.ts web/lib/killfeed-plan.test.ts web/lib/caption-review.ts web/lib/caption-review.test.ts web/lib/api/streams-killfeed-analysis.test.ts web/lib/api/streams-captions.test.ts "web/app/api/streams/[jobId]/killfeed" "web/app/api/streams/[jobId]/killfeed-read" "web/app/api/streams/[jobId]/captions" web/app/api/streams/killfeed
```

- [ ] **Step 2: Strip the API client types**

In `web/lib/api/streams.ts`, delete `KILLFEED_SIDES`, `KillfeedSide`, `KillfeedKill`, `KillfeedReadEvent`, `KillfeedReadResult`, `KillfeedReadEventReference`, `KILLFEED_ANALYSIS_STATUS`, `KillfeedAnalysisStatus`, `KillfeedTimeBase`, `KillfeedRowEvidence`, `KillfeedAnalysisEvent`, `KillfeedAnalysisClip`, `KillfeedAnalysisState`, `StreamCaptionWord`, `StreamCaptions`, `CAPTION_GENERATION_STATUS`, `CaptionGenerationStatus`, `CAPTION_CLIP_STATUS`, `CaptionGenerationState`, and the `killfeedArtifactsStale` render error code.

Delete the `killfeed_seconds`, `killfeed_kills`, `caption_words`, and `caption_reviewed` properties from the clip type, and `killfeed_crop`, `killfeed_analysis`, and `captions` from the plan type.

In `web/lib/api/real.ts` and `mock.ts`, delete every method that called a deleted route and every mock fixture field that no longer exists.

- [ ] **Step 3: Strip the plan helpers**

In `web/lib/streams/plan.ts`, delete the `initialStreamClipEnd` import from the deleted `killfeed-plan.ts` and inline that helper here if it is still needed, delete the xAI-missing upstream error code, delete the `captions: { enabled: false, language: 'es' }` default, delete `captionGenerationIsPending`, `killfeedAnalysisIsPending`, `captionDraftsFromState`, `detectedKillfeedEventCount`, `CaptionWordEntry`, `CaptionSegment`, `groupCaptionWords`, the caption line-break constant, and the killfeed and caption entries in the plan fingerprint tuple.

In `web/lib/stream-recovery.ts`, `stream-preview.ts`, and `reel-brief.ts`, delete the killfeed and caption branches.

In `web/lib/desktop-settings.ts` and `app/(app)/settings/page.tsx`, delete the xAI key settings surface.

- [ ] **Step 4: Strip the components**

In `web/components/streams/stream-editor.tsx`, delete the killfeed and captions panel imports, their state, their handlers, and their slots in the layout.
Close the resulting gap so the remaining panels (source, clip timeline, layout, crop, music, render bar, results) read as a deliberate layout rather than one with a hole in it.

In `render-bar.tsx`, `render-stage.tsx`, `preview-column.tsx`, `crop-picker.tsx`, `analysis-progress.tsx`, and `stream-preview.tsx`, delete the killfeed crop mode, the caption readiness badges, the killfeed analysis progress rows, and the caption progress rows.

In `app/(app)/streams/page.tsx`, delete the killfeed and caption references.

In `app/api/streams/[jobId]/edit-plan/route.ts`, delete the caption and killfeed fields from the request and response validation.

- [ ] **Step 5: Trim the remaining tests**

In `web/lib/streams/plan.test.ts`, `stream-recovery.test.ts`, `stream-preview.test.ts`, `reel-brief.test.ts`, and `desktop-settings.test.ts`, delete every case covering the removed helpers, fields, and settings.

- [ ] **Step 6: Run the web gates in the pre-commit order**

```powershell
pnpm --dir web run lint
pnpm --dir web run typecheck
pnpm --dir web run test:unit
pnpm --dir web run build
```

Expected: all four PASS.

- [ ] **Step 7: Look at the stream editor**

Start the local stack with `.\scripts\local-studio.ps1`, open the stream editor for an existing stream job, and confirm the layout has no empty column, no orphaned heading, and no leftover progress row where the killfeed or captions panels used to be.
Fix any visual gap before committing.

- [ ] **Step 8: Commit**

```powershell
committer "Remove the stream killfeed and captions panels from the web UI" web
```

---

## Task 8: Desktop

Remove the MCP operations, the assistant guidance, and the xAI settings plumbing from Studio.

**Files:**
- Delete: `desktop/src/xai-api-key.ts`, `xai-api-key.test.ts`, `xai-api-key-store.ts`, `xai-api-key-store.test.ts`, `xai-connection.ts`, `xai-connection.test.ts`, `xai-settings-controller.ts`, `xai-settings-controller.test.ts`, `xai-settings-ipc.ts`, `xai-settings-ipc.test.ts`
- Modify: `desktop/src/mcp/operations.ts`, `operations.test.ts`, `surface-coverage.test.ts`, `discovery.ts`, `discovery.test.ts`, `assistant/controller.ts`, `controller.test.ts`, `main.ts`, `preload.ts`, `process-session.test.ts`

**Interfaces:**
- Consumes: the routes removed in Task 5 and the web settings surface removed in Task 7.
- Produces: an MCP operation registry with no `streams.configure_captions`, `streams.start_caption_candidates`, `streams.get_caption_candidates`, `streams.review_caption_candidates`, and no stream killfeed operations.

Read `desktop/GUIDE.md` before starting.

- [ ] **Step 1: Delete the xAI plumbing**

```powershell
trash desktop/src/xai-api-key.ts desktop/src/xai-api-key.test.ts desktop/src/xai-api-key-store.ts desktop/src/xai-api-key-store.test.ts desktop/src/xai-connection.ts desktop/src/xai-connection.test.ts desktop/src/xai-settings-controller.ts desktop/src/xai-settings-controller.test.ts desktop/src/xai-settings-ipc.ts desktop/src/xai-settings-ipc.test.ts
```

In `desktop/src/main.ts` and `preload.ts`, delete the imports, the IPC channel registrations, and the exposed preload methods for those modules.
Keep every other IPC channel intact.

- [ ] **Step 2: Strip the MCP operations**

In `desktop/src/mcp/operations.ts`, delete the operations `streams.configure_captions`, `streams.start_caption_candidates`, `streams.get_caption_candidates`, `streams.review_caption_candidates`, the stream killfeed operations, the weapon catalog listing, and the notice preview.

Delete the `captions`, `killfeed_crop`, `killfeed_seconds`, and `killfeed_kills` properties from the stream edit-plan input schema, the shared kill-notice schema, and the generation-UUID description that mentions captions and killfeed.

Remove `captions`, `subtitles`, `subtitulos`, `transcription`, `xai`, `grok`, `stt`, and `killfeed` from the keyword lists of the surviving stream operations.
Keep those keywords on `renders.start_caption_agent` and `renders.caption_candidates`, which are the demo publish caption, and keep `caption` in the artifact `kind` enum for the same reason.

In `desktop/src/mcp/discovery.ts`, delete the xAI capability probe.

- [ ] **Step 3: Strip the assistant guidance**

In `desktop/src/assistant/controller.ts`, delete the killfeed and stream-caption guidance so the agent no longer offers stages that do not exist.

- [ ] **Step 4: Trim the tests**

In `operations.test.ts`, `surface-coverage.test.ts`, `discovery.test.ts`, `controller.test.ts`, and `process-session.test.ts`, delete every case naming a removed operation, schema property, keyword, or xAI module.
`surface-coverage.test.ts` asserts the MCP surface matches the API surface, so it must be updated to the trimmed route list from Task 5.

- [ ] **Step 5: Run the desktop gates**

```powershell
pnpm --dir desktop run lint
pnpm --dir desktop run typecheck
pnpm --dir desktop run test:unit
pnpm --dir desktop run build
```

Expected: all four PASS.

- [ ] **Step 6: Commit**

```powershell
committer "Remove stream caption and killfeed operations and xAI settings from Studio" desktop
```

---

## Task 9: Landing And Documentation

Stop advertising and documenting features that no longer exist.

**Files:**
- Modify: `landing/app/page.tsx`, `landing/app/layout.tsx`
- Modify: `CLAUDE.md`, `PRODUCT.md`, `desktop/GUIDE.md`, `.codex/GUIDE.md`, `.codex/session-context.md`, `.codex/skills/zackvideo-stream-clips/SKILL.md`
- Modify: any `zv skills` entry under the skills source directory that teaches the removed stages

**Interfaces:**
- Consumes: the final CLI contract from Task 6.
- Produces: documentation that matches the shipped commands.

- [ ] **Step 1: Update the landing copy**

In `landing/app/page.tsx`, rewrite the stream card body at line 93 to drop the xAI subtitle claim, delete the `signal: "XAI WORD TIMESTAMPS"` line, rewrite the how-it-works step at line 102 to drop "and optional xAI captions", rewrite the hero paragraph at line 155 to drop "and optional xAI subtitles", and delete the footer sentence "Optional stream-caption audio goes only to xAI when enabled."

In `landing/app/layout.tsx`, rewrite the metadata description at line 23 to drop "and optional xAI subtitles".

Leave every native-killfeed claim alone: lines 86, 216, 299, and 305 of `page.tsx` and the `opengraph-image.tsx` text describe the demo pipeline, which still ships.

- [ ] **Step 2: Update `CLAUDE.md`**

In the "Codex Desktop: CLI-first" section, replace the stream flow sentence with:

```text
Use `stream variants -> stream plan -> human review -> stream render` for VODs.
```

In the "Approval And Media" section, delete the "factual killfeed policy" and "Spanish captions and review policy" clauses from the stream brief sentence, delete the paragraph beginning "Local Whisper transcription produces `requires_review` evidence", and delete the paragraph beginning "Match imported killfeed facts to detected cues".

In the sentence listing what the persisted stream edit plan is canonical for, delete `captions` and `killfeed`.

Leave the demo pipeline's HUD, death notices, and preset guidance untouched.

- [ ] **Step 3: Update the remaining documentation**

In `PRODUCT.md`, `desktop/GUIDE.md`, `.codex/GUIDE.md`, `.codex/session-context.md`, and `.codex/skills/zackvideo-stream-clips/SKILL.md`, delete the stream killfeed and subtitle stages, flags, and approval questions, and update the stream pipeline diagram to `stream video -> persisted edit plan -> render -> publish pack`.

Run `.\bin\zv.exe skills list --format json` first to find every skill that teaches the removed stages, and update each one.

Write these edits with one full sentence per line, preserving the existing Markdown structure.

- [ ] **Step 4: Verify the docs match the binary**

```powershell
.\scripts\build.ps1
.\bin\zv.exe flows show stream --format json
.\bin\zv.exe workflows list --format json
```

Expected: the stream flow shows only `variants`, `plan`, and `render` stages plus their preflight, doctor, brief, and review steps, and no workflow named `stream-killfeed`, `stream-transcribe`, or `stream-captions` appears.

- [ ] **Step 5: Run the landing build**

Run: `pnpm --dir landing run build`

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
committer "Update landing copy and documentation for the stream flow" landing CLAUDE.md PRODUCT.md desktop/GUIDE.md .codex
```

---

## Task 10: Full Verification

Prove the removal end to end rather than assuming it.

**Files:** none modified unless a check fails.

- [ ] **Step 1: Confirm no references survive**

```powershell
git grep -n -E -i "streamkillfeed|killfeedvision|internal/captions|caption_words|killfeed_seconds|killfeed_crop|XAI_API_KEY" -- ":!data" ":!docs/superpowers" ":!web/.next" ":!desktop/build-resources" ":!desktop/dist-installer"
```

Expected: no hits.
A hit in `internal/editor`, `internal/recording`, or `internal/renderplan` means the demo pipeline was cut by mistake — restore it.

- [ ] **Step 2: Run the full Go gate**

Run: `& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --build --security`

Expected: PASS. `--security` is warranted because this task removed filesystem, subprocess, and credential handling.

- [ ] **Step 3: Run every package gate in pre-commit order**

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

Expected: all PASS.

- [ ] **Step 4: Render one real stream clip**

Use an existing local source under `data/stream-clips/`.

```powershell
.\bin\zv.exe stream plan --input <stream.mp4> --out <run>\edit-plan.json --variant streamer-vertical-stack-40-60 --format json
.\bin\zv.exe stream render --input <stream.mp4> --plan <run>\edit-plan.json --out <run>\render --format json
```

Then open the produced MP4 and confirm by eye: no killfeed overlay, no burned subtitles, no frozen crop strip.
Inspect `<run>\render\shortslistosparasubir\` and confirm the pack manifest lists no `captions/` sidecar and the gallery renders without a killfeed column.

- [ ] **Step 5: Render one legacy plan**

Copy an existing edit plan that still contains `killfeed_seconds` and a `captions` block, for example from `data/stream-clips/PlacidUgliestGoatBatChest-tCG6W0i6lSt0DLAX/vertical-run/edit-plan.json`, into a scratch run directory and render it.

Expected: the render succeeds, the removed fields are ignored, and the output has no killfeed and no subtitles.
This is the compatibility promise from the spec, verified on a real file rather than only in a unit test.

- [ ] **Step 6: Report**

State plainly which checks ran and their results.
If any step failed, say so with the output rather than reporting completion.

---

## Self-Review Notes

Spec coverage was checked section by section.
Every deletion listed in the spec's "Deletion Surface" maps to a task: Go packages and files to Tasks 2 and 3, task queue and orchestrator to Task 4, HTTP API to Task 5, CLI to Task 6, web to Task 7, desktop to Task 8, landing and documentation to Task 9.
The spec's backward-compatibility promise is covered by the Task 1 unit test and the Task 10 Step 5 real-file render.
The spec's two required new tests are Task 1 Step 1 and Task 6 Step 7.
The spec's verification list is Task 10.
The spec's layering mitigation is the task order itself.
