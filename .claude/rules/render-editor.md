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
---

# Render and the editor

`zv-editor` is the FFmpeg render. The domain rules are in `docs/incidents.md`: read **Full Demo audio mastering** before you change the audio chain, **Full Demo overlay format** before you change overlays, and **How to diagnose a user's render failure** for telemetry. Do not copy them here.

## FFmpeg 8.1.2

Studio ships FFmpeg 8.1.2. Some Full Demo and `internal/demooverlay` tests fail on FFmpeg 9.x because `-filter_complex_script` was removed. Treat those failures as pre-existing and do not change the shipped command to fix them. `docs/incidents.md`, **Local test environment caveat**.

## Pipeline golden test

`internal/pipeline` is the one deterministic Short pipeline test: plain `go test`, golden files in `internal/pipeline/testdata/`, no CS2, network, real clock or video encode, and no scripts beside it.

- Extend that test instead of adding another end-to-end suite. Do not add e2e suites, verification scripts, evidence folders or reports to the repo.
- When a change moves the output, rewrite the goldens and review the diff like code: [Update pipeline goldens](../skills/update-golden/SKILL.md).
- It does not replace the manual check: [Verify in Studio](../skills/verify-in-studio/SKILL.md).

## Render-attempt ownership

A render attempt writes durable state only while it still owns that state. A task that lost ownership must not write a failure over a state the worker or another request already advanced.

- Full Demo: `RenderWorker.writeOwnedRenderState` (`internal/workers/full_demo_state.go`) is the fence for failed, cached and successful attempts. A stored variant with a different approved plan returns `recapplan.ErrPlanStale` and keeps its state. `renderplan.SameFullDemoRequest` decides "same plan".
- Stream clips: `writeOwnedStreamRenderAttempt` (`internal/workers/stream_render_attempt.go`) writes only while `RenderState.AttemptID` is the task's attempt. A superseded attempt closes only its own state.
- Tests: `internal/workers/full_demo_state_test.go`, `internal/httpapi/full_demo_handlers_test.go`, `internal/workers/stream_render_intent_test.go`.

## Interrupted work on startup

`reconcile.InterruptedWork` (`internal/reconcile/startup_reconciliation.go`) runs before the orchestrator serves and repairs work that died with the previous desktop process. A non-nil error means the process does not serve. Do not downgrade it to a log line: clients would see an active state with no queue owner. Tests live in `internal/reconcile/`.

## Full Demo artifact cache

`fullDemoCachedArtifacts` (`internal/workers/full_demo_cache.go`) reuses a finished render only when the approved plan matches, the video bytes match `Delivery.ContentSHA256`, and every document from `editor.FullDemoDocumentFiles` exists beside the video and deep-equals the expected one. Anything missing or altered is a miss. The contract is `TestFullDemoCacheRequiresCommittedBytesAndDocuments`.
