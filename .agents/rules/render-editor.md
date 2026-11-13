# Render and the editor

`zv-editor` is the FFmpeg render. Full Demo audio mastering and overlay format are domain rules in `docs/incidents.md` (those two headings, plus **How to diagnose a user's render failure** for telemetry). Do not copy them here.

## FFmpeg 8.1.2

Studio ships FFmpeg 8.1.2. `TestFullDemoConcatsTwoFixtureRounds`, `TestFullDemoOverlayCompositesOntoFixtureCapture` and the `internal/demooverlay` `RenderPNGs` tests fail on FFmpeg 9.x because `-filter_complex_script` was removed. The shipped binary is where they pass. The measurements and the "do not retarget the command" rule are in `docs/incidents.md`, **Local test environment caveat**.

## Pipeline golden test

`internal/pipeline` is the one deterministic Short pipeline test: plain `go test`, golden files in `internal/pipeline/testdata/`, no CS2, network, real clock or video encode, and no scripts beside it. Extend that test instead of adding another end-to-end suite. It does not replace the manual Studio check.

When a change moves the output, rewrite the goldens and review the diff like code. The recipe is [Update pipeline goldens](../skills/update-golden/SKILL.md). `go test ./...` must not gain e2e suites, verification scripts, evidence folders or reports. Unit tests are welcome where they test real logic.

## Render-attempt ownership

A render attempt writes durable state only while it still owns that state. Failed, cached and successful Full Demo attempts use the same fence.

- Full Demo: `RenderWorker.writeOwnedRenderState` (`internal/workers/full_demo_state.go`) holds `generateIntents.WhileIdle` across the ownership check and the pointer swap, together with HTTP admission and the capture handoff. If the stored variant is a different approved plan (`renderplan.SameFullDemoRequest` is false), the write returns `recapplan.ErrPlanStale` (`full_demo_plan_stale`, "Render task no longer owns the approved plan") and leaves the stored state in place. Admitting a duplicate task whose stored plan differs returns the same code ("Another render owns this variant with a different approved plan") in `internal/httpapi/handlers.go`. A discarded enqueue must not write a failure over a state the worker or another request already advanced. `SameFullDemoRequest` (`internal/renderplan/full_demo_state.go`) compares the document and approval plan hashes, and ignores approval timestamps and plan IDs.
- Stream clips: `writeOwnedStreamRenderAttempt` (`internal/workers/stream_render_attempt.go`) writes a variant only while `RenderState.AttemptID` equals the task's attempt id. A task that has lost ownership returns without writing (`errStreamRenderSuperseded`). A superseded attempt closes its own state as failed and does not overwrite the winner's state, the parent job, or the canonical artifact. Recoverable failures persist `streamclips.RenderErrorCodeSuperseded` (`render_superseded`) through that same fence (`internal/workers/stream_render_recovery.go`).
- Tests: `internal/workers/full_demo_state_test.go`, `internal/httpapi/full_demo_handlers_test.go`, `TestSupersededVariantTaskClosesOnlyItsAttemptAfterWinnerCompletes` and `internal/workers/stream_render_intent_test.go`.

## Interrupted work on startup

`reconcile.InterruptedWork` (`internal/reconcile/startup_reconciliation.go`) runs from `cmd/zv-orchestrator/main.go` before the orchestrator serves. It repairs demo jobs, demo render states, generate runs, stream jobs, stream render states, stream acquisitions and editor renders whose in-process work died with the previous desktop process. A nil stream repository or a nil editor-project repository fails before any partial repair. A non-nil error means the process does not serve: clients would see an active state with no queue owner. Tests live in `internal/reconcile/startup_reconciliation_regression_test.go` and `internal/reconcile/restart_reconciliation_integration_test.go` (`TestSQLiteRestartReconcilesPersistedInlineWork`).

## Full Demo artifact cache

`fullDemoCachedArtifacts` (`internal/workers/full_demo_cache.go`) reuses a finished render only when all of these hold:

- `SameFullDemoRequest` matches the approved snapshot on the variant state. A changed approval hash, or a legacy state with no Full Demo snapshot, is a miss.
- The video bytes match `Delivery.ContentSHA256`.
- Every document from `editor.FullDemoDocumentFiles` (`internal/editor/full_demo_documents.go`: `full-demo-approved.json`, `full-demo-effective.json`, `full-demo-audio.json`, `full-demo-loudness.json`, `full-demo-delivery.json`, and `full-demo-hud.json` when a HUD is present) exists beside the video, is valid JSON at or under 4 MiB, and deep-equals the expected document. Missing, altered or invalid JSON is a miss, not a successful read of a different render.

A queued or failed variant may still reuse those bytes when the checks pass. The contract is `TestFullDemoCacheRequiresCommittedBytesAndDocuments` (`internal/workers/full_demo_cache_test.go`).
