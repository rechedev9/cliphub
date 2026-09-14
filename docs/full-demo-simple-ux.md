# Full Demo: simpler creation

## Product behavior

The long-video form keeps the choices that change the user's result: HUD design,
overlay data source, team voices, between-round effects, and an optional sponsor.
Roster and scoreboard overlays are always generated with the neon-violet theme.
The player's observed crosshair is used throughout. Background music is unavailable
for long videos; the separate Short music workflow remains available.

Between-round effects have one switch, enabled by default with the existing
Dinámico preset. Timing, individual effects, and capture preparation are internal
settings. Technical evidence is available for diagnosis without becoming a list
of prerequisites the user must configure.

Creating a video prepares the current plan before enqueueing it. A missing,
incompatible, stale, or blocked plan must never be substituted with an older
approval. Historical documents remain readable; retired options require a new plan.
On a first-use sponsor placement, choosing a specific round prepares the plan to
discover certified round boundaries without starting capture. Creation then binds
the selected boundary to a newly approved plan.

## Local demo overlays

Local demos use the same violet presentation as FACEIT with match facts from
the parser. They retain their own source identity. FACEIT-only profile statistics
and badges must not be inferred from a local match.

Steam avatar lookup is optional and bounded. Private, non-public, unavailable,
and invalid profiles leave an empty avatar. Failure to obtain an avatar does
not prevent creation. Avatar metadata is captured for the job so rendering does
not depend on a later profile response.
The normal generation endpoint, recording path, and variant rendering all use
the same snapshot admission. Private URLs are removed before persistence.

## UX evidence

The installed ClipHub Studio 3.0.4 was exercised on 14 September 2026 before the
change. The long-video page exposed a ten-design HUD gallery, a custom-HUD switch,
a crosshair selector, music and playlist controls, more than twenty transition
controls, separate roster and scoreboard switches, a theme selector, and editable
technical settings. The initial music and sponsor selections had no files, so
the form displayed missing-asset errors. Creating a video also required a separate
plan-save action.

The implementation is isolated in `codex/full-demo-simple-ux` so the older primary
checkout and its uncommitted changes remain intact. This change does not publish
a release or replace the installed app.

## Verification

- Parsed an installed local `.dem` with `TestRosterScansRealDemo`: passed with a
  nonempty player roster, match score, and consistent player statistics.
- Rendered the real ten-player local roster through the installed Chromium
  overlay renderer. Both 1920x1080 neon-violet intro and scoreboard were visually
  inspected; local match facts and empty portrait slots remain legible.
- One bounded live Steam lookup resolved/downloaded seven public avatars, found
  two explicitly private profiles, and left one unavailable profile blank.
  The intro and scoreboard were rendered with the seven cached images and three
  blank slots. No second network lookup was needed to render the scoreboard.
- Compared JSON emitted by Go `DefaultOptions` with browser normalization:
  identical options and option keys. A freshly saved current plan does not become
  dirty merely because Go omits optional image references.
- `go vet ./...` passed. Worker tests passed with actual editor CLI dry-runs and
  identity-bound HUD telemetry. Historical cache fixtures retain their explicit
  native capture contract; current HUD tests keep their strict evidence checks.
- Production web build, lint, TypeScript checks, and the complete web unit suite
  passed. Browser regressions cover Full Demo and the unchanged Short
  workflow, including canceling preparation, returning, and creating successfully.
- Final Go checks passed for planning, HTTP admission/retries, workers, native
  capture seek timing, and the complete sponsor/audio media canary. The canary
  covers embedded audio, replacement narration, round splitting, end placement,
  and muxed B-frames. The repository-wide run found the obsolete fixtures and
  AAC mismatch described here; the affected checks were rerun after correction.
- Current-policy regressions reject otherwise valid historical approvals with
  retired manual timing, audio fallback/calibration, or output settings, while
  preserving the supported visible choices and historical document hashes.
- Exercised the compiled form at 390, 1024, and 1440 pixels with a 187-character
  unbroken player name. No horizontal overflow; HUD selection, overlay source,
  effects, voices, sponsor upload entry, and the create/back controls remain
  usable. Saved screenshots are in the ignored `.local/full-demo-ux-qa/` folder.

Removing the music bed exposed an AAC duration mismatch in the narration canary.
The mastering stage now pads/trims the audio to the exact timeline sample count
and bounds mux duration. Delivery still verifies the actual video frames, decoded
audio, stream durations, and final loudness; its acceptance checks were not relaxed.

Switching to Short during preparation cancels the pending plan request and
prevents a pending match lookup from subsequently storing a creation intent.
Returning to Full Demo releases its busy state and permits a fresh creation
attempt; a canceled preparation cannot enqueue a video in the background or
navigate away later. An intent already committed remains in the durable queue.

Browser checks use controlled API responses to exercise plan creation, approval,
and failure states. The local demo parser and Chromium overlay images use real
input data. A new complete CS2/HLAE recording and the installed-app release are
outside this verification; the installed 3.0.4 app remains unchanged.
