# Remove Stream Killfeed And Burned Captions

Date: 2026-07-25

## Problem

Two features of the stream-clip pipeline do not work well enough to ship: killfeed capture and burned-in Spanish subtitles.
Killfeed detection reads the source frame, guesses cues, and needs a manual factual-event import that still produces wrong or empty notices.
Subtitles depend on local Whisper candidates plus an xAI second pass, and the reviewed words still burn in badly.
Both carry a large surface — packages, workers, HTTP endpoints, CLI stages, UI panels, and approval gates — that has to be maintained even when the feature is not used.

The decision is to remove both end to end rather than keep dead or flag-disabled code.

## Scope

In scope: the stream-clip pipeline only.

- Stream killfeed: cue detection, vision-based notice reading, factual event import, frozen-crop and synthetic-notice rendering, and every surface that exposes them.
- Stream burned-in subtitles: local Whisper transcription, the xAI Spanish pass, caption candidates and their review, ASS generation, and the burn pass.
- The xAI integration as a whole, because those two features are its only consumers.

Out of scope, explicitly untouched:

- The demo pipeline's native killfeed (HLAE death notices, `internal/editor/killfeed_probe.go`, the `viral-60-clean` and `full-hud-60` presets, `--portrait-safe-killfeed`).
- The publish-pack caption, meaning the social copy, title, and hashtags produced for an upload, including the demo render caption agent.
- Historical artifacts under `data/`, which stay on disk as they are.

## Resulting Stream Contract

The stream flow goes from `variants -> plan -> killfeed -> transcribe -> captions -> render` to `variants -> plan -> render`, plus the existing cover and gallery steps.

Three CLI stages and their preflights disappear: `zv stream killfeed`, `zv stream transcribe`, and `zv stream captions`.
`zv stream plan` loses `--killfeed-crop`, `--detect-killfeed`, and `--captions`.
`zv stream render` loses the readiness gate that today fails when a clip has audio but no reviewed Spanish words and no reviewed no-speech decision.

The creative brief for streams loses the killfeed-source question and the Spanish-captions and review-policy questions.
It keeps clip bounds and title, crop and framing, music, delivery shape, source-audio treatment, and cover strategy.

## Backward Compatibility

Persisted edit plans that still carry `killfeed_*` fields or a `captions` block keep loading.
The fields are removed from the Go types, so JSON decoding drops them silently and the clip renders without killfeed and without subtitles.
No new validation error is introduced, and no existing stream job breaks.

The legacy killfeed-snapshot migration keyed on `schema_version` `1.0` is removed along with the field it migrated into.

## Deletion Surface

### Go packages removed entirely

- `internal/streamkillfeed/` — detector, extract, ffmpeg, fingerprint, scanner, types.
- `internal/killfeedvision/` — the vision client that reads notices.
- `internal/captions/` — ASS karaoke generation, the xAI STT client, and the Spanish second pass.
- `internal/streamclips/noticeassets/` — embedded weapon icons, flags, and the notice font.

`internal/mediafont` stays: `internal/editor` and `internal/streamclips` still use it.

### Go files removed

- `internal/streamclips/notice_render.go`, `killfeed_analysis.go`, `killfeed_detect.go`, `caption_candidates.go`.
- `internal/streamcli/killfeed_detect.go`, `killfeed_import.go`, `captions_import.go`, `transcribe.go`.
- `internal/workers/stream_caption_worker.go`, `stream_killfeed_worker.go`, `stream_killfeed_local.go`.
- `internal/httpapi/stream_caption_handlers.go`, `stream_killfeed_handlers.go`, `stream_killfeed_analysis_handlers.go`, `stream_killfeed_fingerprint.go`.
- Every dedicated test file for the above.

### Go files edited

- `internal/streamclips/types.go`: drop `KillfeedSeconds`, `KillfeedKills`, `KillfeedCueProvenance`, `KillfeedCrop`, `KillfeedAnalysis`, `KillfeedKill`, `CaptionsPlan`, and the `Captions` field, together with their validation, normalization, provenance lookup, and the `killfeed_artifacts_stale` render error code.
- `internal/streamclips/ffmpeg.go`: drop the subtitle burn pass and the frozen or synthetic killfeed overlay.
- `internal/streamclips/artifacts.go`, `edit_plan_fingerprint.go`, `gallery.go`, `variants.go`: drop caption and killfeed artifact keys and the sidecar entries that reference them.
- `internal/workers/media_worker.go`, `stream_render_attempt.go`, `stream_render_recovery.go`: drop killfeed and caption generation handling, artifact preparation, and staleness recovery.
- `internal/tasks/tasks.go`: drop `stream:captions` and `stream:killfeed`, their payloads, generation headers, and fingerprint validation.
- `internal/httpapi/routes.go`, `handlers.go`, `capabilities.go`, `workbench_htmx.go`: drop the routes, the xAI capability gate, and the workbench caption entry point for streams.
- `internal/obs/obs.go`, `internal/tuiclient/types.go`, `internal/artifacts/keys.go`: drop the stage, class, and artifact identifiers that only these paths emitted.

### CLI

- `cmd/zv/usage.go`, `flow_commands.go`, `workflow_catalog.go`, `workflow_docs.go`, `workflow_entrypoints.go`, `command_validation.go`, `check_config.go`: drop the three stages, their flow steps, and their workflow catalog entries.
- `cmd/zv/capabilities_command.go`: drop `killfeed_detection_ready`, `spanish_captions_ready`, `captions_provider`, and `captions_configuration`.
- `internal/streamcli/streamcli.go`, `support.go`, `preflight.go`, `cover.go`: drop the subcommand dispatch, the removed flags, the detection service method, and the caption readiness checks.

### Web

- Remove `components/streams/killfeed-panel.tsx`, `killfeed-kills-editor.tsx`, `use-killfeed-analysis.ts`, `captions-panel.tsx`, `caption-review-card.tsx`, `use-caption-review.ts`.
- Remove `lib/killfeed-analysis.ts`, `lib/killfeed-plan.ts`, `lib/caption-review.ts`, `lib/api/streams-killfeed-analysis.test.ts`, `lib/api/streams-captions.test.ts`, and the matching unit tests.
- Remove the proxy routes under `app/api/streams/[jobId]/killfeed/`, `killfeed-read/`, `captions/`, and `app/api/streams/killfeed/`.
- Edit `components/streams/stream-editor.tsx`, `render-bar.tsx`, `render-stage.tsx`, `preview-column.tsx`, `crop-picker.tsx`, `analysis-progress.tsx`, `stream-preview.tsx`, and `lib/api/streams.ts`, `lib/api/real.ts`, `lib/api/mock.ts`, `lib/streams/plan.ts`, `lib/stream-recovery.ts`, `lib/stream-preview.ts`, `lib/reel-brief.ts`.
- Remove `components/settings/xai-settings.tsx` and its wiring in `app/(app)/settings/page.tsx` and `lib/desktop-settings.ts`.

The stream editor keeps source, clip timeline, layout, crop, music, render bar, and results.
The killfeed and captions panels are removed from the layout rather than hidden.

### Desktop

- `src/mcp/operations.ts`: drop `streams.configure_captions`, `streams.start_caption_candidates`, `streams.get_caption_candidates`, `streams.review_caption_candidates`, the stream killfeed operations, the weapon catalog listing, and the notice preview; drop the killfeed and caption properties from the edit-plan schema.
- `src/assistant/controller.ts`: drop the killfeed and caption guidance.
- Remove `src/xai-api-key.ts`, `xai-api-key-store.ts`, `xai-connection.ts`, `xai-settings-controller.ts`, `xai-settings-ipc.ts` and their tests, and their registration in `src/main.ts` and `src/preload.ts`.

### Orchestrator and landing

- `cmd/zv-orchestrator/config.go` and `main.go`: drop `XAI_API_KEY`, `xaiEnabled`, and the capability it feeds.
- `landing/app/page.tsx` and `layout.tsx`: drop the xAI subtitle claims in the stream card, the how-it-works step, the hero copy, the metadata description, and the privacy line about caption audio going to xAI.
  The native killfeed claims on the landing describe the demo pipeline and stay.

### Documentation

- `CLAUDE.md`: rewrite the stream flow to `stream variants -> stream plan -> stream render`, drop the killfeed and caption items from the approval gate, drop the Whisper transcription paragraph and the imported-killfeed-facts paragraph.
- `PRODUCT.md`, `desktop/GUIDE.md`, `.codex/GUIDE.md`, `.codex/session-context.md`, `.codex/skills/zackvideo-stream-clips/SKILL.md`, and any `zv skills` entry that teaches the removed stages.
- Specs and plans already under `docs/superpowers/` stay as history and are not edited.

## Testing

- Delete the dedicated unit and integration tests for every removed package and file.
- Trim the journey tests to the new contract: `cmd/zv/app_flow_stream_e2e_test.go`, `cmd/zv/app_stage_contract_test.go`, `internal/streamcli/stream_journey_test.go`, `cmd/zv-orchestrator/stream_e2e_test.go`, `cmd/zv-orchestrator/inline_queue_test.go`.
- Add one behavior test that decodes a legacy edit plan carrying `killfeed_seconds`, `killfeed_kills`, `killfeed_crop`, and a `captions` block, and asserts it validates and renders with no killfeed overlay and no subtitle track.
- Add or adjust a test asserting `zv stream render` accepts a plan for an audible clip with no caption data, which the removed gate used to reject.

## Verification

Run in this order, and treat the manual render as required rather than optional.

1. `& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --build`
2. `pnpm --dir web run lint`, `typecheck`, `test:unit`, `build`
3. `pnpm --dir desktop run lint`, `typecheck`, `test:unit`, `build`
4. `pnpm --dir landing run build`
5. A real `zv stream render` of one short local clip, inspecting the output video to confirm no killfeed overlay and no burned subtitles, and inspecting the pack manifest and gallery for leftover caption sidecars.

## Risks

`internal/workers/media_worker.go` carries by far the most references, and stream render revision, attempt, and recovery logic is entangled with killfeed and caption generation IDs and fingerprints.
Removing these in one pass risks breaking stream rendering in ways the unit tests would not catch.

Mitigation: work in layers, keeping the tree compiling at each step — types, then render composition, then workers, then HTTP API, then CLI, then web and desktop, then docs.

## Out Of Scope Follow-Ups

If killfeed or subtitles are revisited later, git history holds the removed implementation.
Any future attempt should start from a fresh design rather than restoring this code, because the reason for removal is that the approach did not work.
