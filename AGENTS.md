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

## Local test environment caveat

`TestFullDemoConcatsTwoFixtureRounds` and
`TestFullDemoOverlayCompositesOntoFixtureCapture` fail on machines with
FFmpeg 9.x because `-filter_complex_script` was removed. The shipped FFmpeg
is 8.1.2, where they pass. Treat those two as pre-existing when running
`go test ./internal/editor/` locally with a newer FFmpeg, and do not "fix"
them by changing the shipped command unless the bundled FFmpeg is upgraded.

## Releases

- Version lives in `desktop/package.json` and `landing/app/page.tsx`
  (`DOWNLOAD_URL`, `RELEASE_VERSION`). Bump both, commit
  `chore(release): prepare Studio X.Y.Z`, then push an annotated tag
  `vX.Y.Z` ("ClipHub Studio X.Y.Z"). `.github/workflows/desktop-release.yml`
  builds the installer and publishes the GitHub Release as `latest`, which
  is what the in-app updater reads.
