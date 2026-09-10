# Agent notes for ClipHub

Lessons from real user incidents. Read before touching the areas below.

## Full Demo audio mastering (`internal/editor/full_demo_audio*.go`)

### Incident: loudnorm target driven out of range (Studio 3.0.0, 2026-09-10)

A 14-minute 1080p60 Full POV render failed twice at 81 % with:

```
[Parsed_loudnorm_0] Value -10.180000 for parameter 'TP' out of range [-9 - 0]
Error applying option 'TP' to filter 'loudnorm': Result too large
```

The native AAC master loop in `masterFullDemoProgram` re-targeted
`TargetTPDBTP` downward by the decoded true-peak overshoot on every attempt
without any bound. Large overshoots pushed the third attempt below loudnorm's
accepted range, ffmpeg refused the filter, and the render aborted before ever
reaching the Media Foundation AAC recovery that exists precisely for this case.
The same user had already hit the un-recovered variant in 2.4.62
(`audio_loudness_failed ... after three masters`), so the recovery path was
built, but the retargeting loop could still bypass it. Fixed in `aa079cdb`.

Rules that follow from it:

- **Every value fed into an ffmpeg filter must be clamped to the filter's
  documented range** before building the command. Known ranges:
  `loudnorm` `I` in `[-70, -5]`, `TP` in `[-9, 0]`, `LRA` in `[1, 50]`.
  Use `nextMasterTarget` for master retargeting; do not mutate
  `attemptTarget` inline.
- **Feedback loops that adjust a parameter from a measurement need a bound
  and an exit condition.** If the clamped next target equals the current one,
  stop retrying and hand over to the next strategy (`recoverFullDemoAAC`)
  instead of repeating a target that already failed.
- **Do not add a new fallback stage without proving the earlier stages can
  reach it.** Write a unit test that drives the loop with the measured values
  from the incident (see `TestFullDemoMasterRetargetStaysWithinLoudnormRange`)
  and asserts the hand-over happens.
- Media Foundation (`aac_mf`) is a Windows OS component, not something the
  installer can bundle. It is present on all standard Windows editions and
  the shipped FFmpeg exposes it; only "N/KN" editions lack it. Do not
  propose bundling it.

### How to diagnose a user's render failure

- The UI shows a generic message ("No se pudo completar el video en este
  equipo"). The real ffmpeg stderr is kept in remote telemetry.
- Query it with `bash scripts/telemetry-query.sh incident CH-XXXX-...`
  (needs `~/.config/cliphub/telemetry-agent.env`). `stats 24` gives
  fingerprints without messages. Look at `pipeline.error` events with
  `class=render:variant` and read `message`.
- Map the UI percentage to the pipeline stage using the weights in
  `internal/editor/short_pack.go` (`renderShort`) and
  `internal/editor/progress.go`; 76-82 % is the loudness/AAC master loop,
  83-86 % is AAC recovery, 87 %+ is delivery verification.
- Working-directory logs (`out/logs/program-*.txt`) are deleted with the
  temp workdir on failure unless `ZV_MEDIA_WORK_DIR` is set.

## Full Demo overlay format (`internal/demooverlay`, `recapplan.Options`)

### Incident: FACEIT demo rendered with demo-facts-only overlays (Studio 3.0.1, 2026-09-10)

The first Full Demo render with a custom HUD (Circuit) came out with the
legacy intro/outro layout instead of the FACEIT one. The HUD was not the
cause: the editorial plan flow (`f2b74939`) had replaced the always-on
`demoSource: faceit` of the old Studio chain with a plan field
`overlays.source` that defaults to `"demo"`, and that field was the only
input to the overlay layout. `source_kind` ("Origen de la demo") was
validated but never read. Every render between the two flows had failed for
unrelated reasons, so the format change surfaced days later on the first
successful render and was blamed on the newest feature.

Rules that follow from it:

- `recapplan.Options.OverlaySource()` is the single resolver: the demo origin
  (`source_kind`) owns the intro/outro layout; `overlays.source` only adds
  FACEIT enrichment to a plain demo. `web/lib/full-demo-plan.ts`
  `fullDemoOverlaySource` mirrors it and both are covered by tests.
- Do not add a plan option that the render never reads. If a field exists in
  the wire and the UI, something must consume it or it must be removed.
- When a user reports a regression "caused by feature X", check telemetry
  for the last successful run of the same path before X shipped. If every
  run in between failed, the regression window is the whole gap, not X.

## Local test environment caveat

`TestFullDemoConcatsTwoFixtureRounds`,
`TestFullDemoOverlayCompositesOntoFixtureCapture` and the
`internal/demooverlay` `RenderPNGs` tests fail on machines with
FFmpeg 9.x because `-filter_complex_script` was removed. The shipped FFmpeg
is 8.1.2, where they pass. Treat those as pre-existing when running
`go test ./internal/editor/ ./internal/demooverlay/` locally with a newer
FFmpeg, and do not "fix" them by changing the shipped command unless the
bundled FFmpeg is upgraded.

## Releases

- Version lives in `desktop/package.json` and `landing/app/page.tsx`
  (`DOWNLOAD_URL`, `RELEASE_VERSION`). Bump both, commit
  `chore(release): prepare Studio X.Y.Z`, then push an annotated tag
  `vX.Y.Z` ("ClipHub Studio X.Y.Z"). `.github/workflows/desktop-release.yml`
  builds the installer and publishes the GitHub Release as `latest`, which
  is what the in-app updater reads.
