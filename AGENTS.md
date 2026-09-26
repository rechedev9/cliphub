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
  `loudnessFilter` and `ProgramAACFallbackMaster.filter` clamp where the
  argument is built, and the initial targets come from `aacHeadroomTarget`
  and `initialAACRecoveryMaster`.
  `TestFullDemoFilterBuildersClampOutOfRangeTargets` and
  `TestFullDemoRetargetStaysInRangeAndExhaustsForAnyMeasurement` guard both;
  add any new computed filter option to `computedFilterRanges`.
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

### Capture tail shortfall tolerance (2026-09-22)

A 64 Hz round window can map to a fractional frame count (2040 ticks =
1912.5 frames); `TickFrames` rounds up and HLAE may deliver one frame less.
`recording.FullDemoTailPadToleranceFrames` (2) accepts that shortfall, the
item stream clones the last frame with `tpad` before `trim`, and the pads are
recorded as `capture_tail_pads` in the render evidence and
`full-demo-delivery.json`. Anything larger still fails with the original
coverage error.

- Do not raise the tolerance to hide real capture loss; a larger gap means
  HLAE dropped frames, not rounding.
- Audio needs no pad: every bus is already `apad`/`atrim`-ed to the canonical
  sample count.

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
  `TestEveryChangeableFullDemoOptionIsRead` enforces it with type-checked
  reads for every field the execution gate lets a user change; reads in
  `internal/httpapi` and `cmd/zv` do not count, because they never reach the
  video.
- When a user reports a regression "caused by feature X", check telemetry
  for the last successful run of the same path before X shipped. If every
  run in between failed, the regression window is the whole gap, not X.

### Outro scoreboard timing (2026-09-25)

The outro scoreboard starts no earlier than
`recapplan.ScoreboardAfterLastKillSeconds` after the last kill and ends with
the gameplay span (`generatedFullDemoOverlayEffects`). With the scoreboard
enabled, the planner extends the final round's tail to fit that second plus
`recapplan.ScoreboardSeconds` (`EndReason` `scoreboard-tail`), capped two
seconds before the demo end because Full Demo windows are exact. It extends only a surviving POV and only when safe tail trim is
approved, so an uncertified post-match tail is trimmed instead of failing the
capture. Toggling the scoreboard therefore changes the final capture window.
Real demos can stop about 6.5 s after the match-ending kill (19-round FACEIT
Mirage, 2026-09-25), so the scoreboard there lasts about 3.5 s; the video is
not padded with frozen frames to reach the full 8 s.

## HLAE pin and CS2 updates (`desktop/src/hlae-tool.*`)

### Incident: every capture failed after a CS2 update (Studio 4.0.1, 2026-09-23)

A user reported "Full Demo renders stopped working after the last updates".
Telemetry showed the render never ran: `record:demo` exited 6 after 5 s with
`HLAE hook crashed with a native error dialog ("Error - AfxHookSource2")`.
Nothing in the capture path had changed since the last good run (3.0.8); the
CS2 update of 2026-09-22 (build 14182) broke AfxHookSource2's signature scan in
every HLAE release, including 2.192.2. Studio 5.0.0 pins
`2.192.2-cliphub.1`: the official 2.192.2 archive with `x64/AfxHookSource2.dll`
rebuilt from advancedfx PR #1213 (provenance in `cliphub-build.txt` inside the
zip). `assemble.mjs` stages the pinned archive from `desktop/.hlae-cache/` when
its sha256 matches the pin and only downloads `url` otherwise, so a local
package does not depend on the archive being published.

Rules that follow from it:

- Exit code 6 / `capture_incompatible` means HLAE vs CS2 build, not a ClipHub
  regression. Check the advancedfx issues and releases and the CS2 update time
  against the user's last successful capture before bisecting our commits.
- Studio passes `ZV_HLAE_PATH` for the pinned version and reinstalls the pin
  when its cache digest does not match, so users cannot work around an
  incompatible pin by installing a newer HLAE themselves. The fix always ships
  as a new pin plus a Studio release.
- When advancedfx has no fixed release yet: build `AfxHookSource2` from the
  fix (`cmake --preset x64-release`, then
  `cmake --build build/x64-release --config Release --target AfxHookSource2`),
  replace only `x64/AfxHookSource2.dll` in the latest official zip, and pin the
  result. `treeSha256` is the digest `runtime-tools.ts` computes over the
  `Expand-Archive` output (path, NUL, file sha256, newline per file in
  `localeCompare` order); recompute it, do not reuse the zip sha256.
- Prove the pin with a real capture on the current CS2 build before release:
  reproduce the crash with the old pin, then capture a kills plan, a
  `deathnotices` plan and a Full Demo subset with the new one.
- Go back to an official advancedfx release as soon as one supports the
  current CS2 build.
- Building advancedfx with only VS Build Tools needs `-products *` in the
  `vswhere` calls of its top-level `CMakeLists.txt`, and the shader step fails
  with code 9009 when `NoDefaultCurrentDirectoryInExePath` is set in the
  environment.

### Incident: black first-person capture with the interim pin (Studio 5.0.0, 2026-09-23)

A user's Full Demo render passed every check but the video was black while
the POV player was alive: only the native HUD and crosshair were drawn, and
the world appeared only on the death camera. The delivered 1080p60 file
averaged 2.4 Mb/s, against about 40 Mb/s for a normal capture. The
`2.192.2-cliphub.1` DLL had been built from `8f355239`, an intermediate
commit of PR #1213. Upstream then landed `9ba34af6` ("Fix
QueueCallbackBeforeUi being called in random order from different threads
and in wrong state") and shipped both in official 2.192.3 the same day.
The failure depended on the machine: the pin's own 17-round Full Demo run
on the dev PC captured normally. Studio 5.1.1 pins official 2.192.3.

- Build an interim DLL only from a merged upstream commit, and re-pin the
  official release as soon as it ships, even when the interim build looks
  fine locally.
- Delivery decode proves the file decodes, not that it shows the game. When
  verifying a capture, check its frames and bitrate (`blackdetect`,
  `signalstats`), not only exit codes.

### Incident: DeathMsg.cpp crash after the next CS2 update (Studio 5.3.0, 2026-09-24)

One day after 2.192.3 shipped, CS2 1.41.8.3 (build 25492732) made
AfxHookSource2 stop at a native `Error - AfxHookSource2` dialog reading
`Problem in ...\AfxHookSource2\DeathMsg.cpp:1248` while CS2 was still on the
Valve splash. advancedfx tracked it as issue #1216 and fixed it in official
2.192.4 (AfxHookSource2 0.41.4) the same day. Studio 5.3.1 pins 2.192.4.

- CS2 can break the pin twice in two days. Before bisecting a capture
  failure, compare `PatchVersion` in `game\csgo\steam.inf` with the CS2
  version named in the pinned release's AfxHookSource2 changelog.
- The installed tool folder is not a clean source for `treeSha256`: HLAE
  writes `ffmpeg/ffmpeg.ini` at runtime. Compute the digest over a fresh
  `Expand-Archive` of the release zip.

## Render QA warnings are informational (2026-09-25)

Renders and compositions with QA warnings used to park in `review_required`
until a human pressed "Resolver revisión QA", which stalled every run an AI
agent drove end to end. The gate was removed: warnings are stored on the
render state and shown, but the render is `ready` / `composed`, publishable
and downloadable.

- Do not reintroduce a human sign-off step, a `review_required` status, or a
  warnings-based publish gate. If a warning signals a real defect, fail the
  render with a clear error instead.
- `RenderVariantStatusReview` and `job.StatusReviewRequired` remain only so
  old durable documents decode; stored review states are promoted to `ready`
  on read and by the startup materialization pass.

## Demo scan failures reach the user with their cause (2026-09-26)

A user reported that a `.dem` "was not read": the upload page only ever said
"No se pudo escanear esa demo. Prueba con otro archivo .dem.". Every cause
collapsed into that line: `waitForStatus` threw a bare `job <id> failed`, the
roster scan's reason (`scan roster: parsing demo: demo_incompatible: ...`) got
no `failure_code` because `obs.ClassOf` only matched the marker as a prefix,
and CS:GO (`HL2DEMO`) demos were admitted although demoinfocs v5 always
refuses them.

- Upload rejections carry a code (`csgo_demo`, `not_a_demo`,
  `unreadable_demo`); `parseToEnd` tags every non-cancel parser error
  `demo_incompatible:`; `waitForStatus` rethrows the job's
  `failure_reason`/`failure_code`. `web/lib/demo-parse-flow.ts`
  `demoScanError` maps each code to its own advice.
- A new admission or scan failure needs a code and a line in
  `SCAN_FAILURE_HINTS`, not another generic message.
- A `demo_incompatible` scan on a fresh CS2 demo usually means a CS2 update
  changed the demo format before demoinfocs caught up: check its releases
  before touching our parser code.

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
