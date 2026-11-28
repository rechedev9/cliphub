---
paths:
  - "cmd/zv-editor/**"
  - "internal/editor/**"
  - "internal/pipeline/**"
  - "internal/workers/**"
  - "internal/renderplan/**"
  - "internal/recapplan/**"
  - "internal/reconcile/**"
  - "internal/demooverlay/**"
  - "internal/captureretention/**"
  - "internal/customhud/**"
  - "internal/hudthemes/**"
  - "internal/htmlstill/**"
  - "internal/radaroverlay/**"
  - "cmd/zv-recorder/**"
---

# Render and the editor

`zv-editor` is the FFmpeg render. The domain rules are in `docs/incidents.md`: read **Full Demo audio mastering** before you change the audio chain, **Full Demo overlay format** before you change overlays, and **How to diagnose a user's render failure** for telemetry. Do not copy them here.

## FFmpeg 8.1.2

Studio ships FFmpeg 8.1.2. Some Full Demo and `internal/demooverlay` tests fail on FFmpeg 9.x because `-filter_complex_script` was removed. Treat those failures as pre-existing and do not change the shipped command to fix them. `docs/incidents.md`, **Local test environment caveat**.

## Filter graph cost

- A part crops to the output aspect before it scales to the output size. Scaling up and then cropping spends Lanczos on pixels the crop throws away. A zoom keeps its per-frame scale. `TestBaseGeometryCropsBeforeScalingForEveryFormat` (`internal/editor/scale_filters_test.go`).
- Overlays that cover a whole part or item blend with `overlay=format=yuv420`. `format=auto` converts every frame to RGBA and back, even while the overlay is disabled. Only the single-frame cover card keeps `format=auto`. `TestFullDemoCompilationFilterOverlayWindowGraph`.
- The editor's NVENC preset is `p4` (`internal/editor/video_encode.go`). On CS2 footage it encodes about a third faster than `p5` at the same bitrate, and the quality difference cannot be seen. `p3` and faster presets lose more quality and spend more bits. Measure SSIM and bitrate before you move it.

## Render deadline

A `RenderWorker` render's deadline scales with its program length (a Full Demo's approved timeline, else the captured plays, since a summary has no length cap) and never drops below the configured one; a run its deadline cut fails with `render_timeout`. Do not go back to one fixed deadline for every render. `docs/incidents.md`, **a slow PC's Full Demo render was cut at the fixed deadline**.

## Pipeline golden test

`internal/pipeline` is the one deterministic Short pipeline test: plain `go test`, golden files in `internal/pipeline/testdata/`, no CS2, network, real clock or video encode, and no scripts beside it.

- Extend that test instead of adding another end-to-end suite. Do not add e2e suites, verification scripts, evidence folders or reports to the repo.
- When a change moves the output, rewrite the goldens and review the diff like code: [Update pipeline goldens](../skills/update-golden/SKILL.md).
- It does not replace the manual check: [Verify in Studio](../skills/verify-in-studio/SKILL.md).

## Render-attempt ownership

A render attempt writes durable state only while it still owns that state. A task that lost ownership must not write a failure over a state the worker or another request already advanced.

- Full Demo: `RenderWorker.writeOwnedRenderState` (`internal/workers/full_demo_state.go`) is the fence for failed, cached and successful attempts. A stored variant with a different approved plan returns `recapplan.ErrPlanStale` and keeps its state. `renderplan.SameFullDemoRequest` decides "same plan".
- Stream clips: `writeOwnedStreamRenderAttempt` (`internal/workers/stream_render_attempt.go`) writes only while `RenderState.AttemptID` is the task's attempt. A superseded attempt closes only its own state.
- Editor timeline: `TimelineRenderWorker.ownedState` (`internal/workers/timeline_worker.go`) lets a task write its failure or published render, and the project row, only while `timelineplan.RenderState.AttemptID` is the payload's attempt; every writer goes through `StartAttempt`, `Fail` and `Publish` (`internal/timelineplan/render_state.go`). Test: `internal/workers/timeline_render_owner_test.go`.
- Tests: `internal/workers/full_demo_state_test.go`, `internal/httpapi/full_demo_handlers_test.go`, `internal/workers/stream_render_intent_test.go`.

## Interrupted work on startup

`reconcile.InterruptedWork` (`internal/reconcile/startup_reconciliation.go`) runs before the orchestrator serves and repairs work that died with the previous desktop process. A non-nil error means the process does not serve. Do not downgrade it to a log line: clients would see an active state with no queue owner. Tests live in `internal/reconcile/`.

## Full Demo artifact cache

`fullDemoCachedArtifacts` (`internal/workers/full_demo_cache.go`) reuses a finished render only when the approved plan matches, the video bytes match `Delivery.ContentSHA256`, and every document from `editor.FullDemoDocumentFiles` exists beside the video and deep-equals the expected one. Anything missing or altered is a miss. The contract is `TestFullDemoCacheRequiresCommittedBytesAndDocuments`.

Which captures an edit reuses and which ones `captureretention` frees: `docs/incidents.md`, **Editing a finished video**.

## Full Demo plans, HUD and overlays

- An approved plan keeps its hash. A new optional plan field is omitted from the wire when absent (`omitempty`), so a document approved before it existed stays approvable. Decoding is strict (`decodeStrict` in `internal/recapplan/validation.go` refuses unknown fields), so a retired field needs an explicit drop there, as `dropRetiredSponsor` does, or older documents stop decoding. A missing, stale or blocked plan is never replaced with an older approval.
- The capture hash leaves overlays out. A HUD theme, its composition, team names, transitions, bumpers and overlay timing change only the render and reuse the capture; a capture choice (the HUD capture profile, `capture.cliphub_radar`) moves the capture hash.
- The custom HUD capture sets its CS2 cvars through HLAE, verifies their readback during the capture and restores the saved values after. A new cvar joins that same snapshot, readback and restore.
- Adding a recorded field to the HUD telemetry (`internal/customhud`) moves the telemetry version; a fact derived at render time does not.
- The selected player's SteamID sets both the capture and the HUD focus; roster order and other players' deaths never change it. A player portrait is never inferred from a name: without one, the slot shows the CT or T label.
- An overlay never invents a value: a missing fact hides the element (alive counts, win probability, FACEIT statistics). A Short without radar data renders without the radar, while a Full Demo radar track that is missing or does not match its SHA-256 fails the render.
- The intro and scoreboard pages render in Studio's own Chromium through `--cliphub-render-overlay` (`ZV_OVERLAY_RENDERER_PATH`); do not add a Python or second browser runtime. Own HUD themes stay on the PC, outside library sync, and a plan copies the theme whole.
- That renderer draws only a 1920x1080 or 1080x1920 page, so a sprite sheet (the Full Demo SUBSCRIBE button, the Short's like and subscribe card) renders its cells across several such pages stacked into one PNG (`renderSpriteSheet` in `internal/demooverlay`). Each `editor.SpriteClip` holds its last cell until the card leaves, so the Short card stays liked and SUBSCRIBED; the first bell swing adds the generated marimba note unless the plan mutes every audio source. Tests: `internal/demooverlay/short_subscribe_test.go`, `TestLikeThenSubscribeHoldAndDingInTheCompilation` (`internal/editor`).
