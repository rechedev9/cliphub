# Agent notes for ClipHub

Rules learned from real user incidents. Read the section before touching its
area. The account of each incident (symptom, cause, fix, measurements) is in
`docs/incidents.md` under the same headings.

## Full Demo audio mastering (`internal/editor/full_demo_audio*.go`)

- **Clamp every value fed into an ffmpeg filter to the filter's documented
  range** before building the command. `loudnorm`: `I` in `[-70, -5]`, `TP` in
  `[-9, 0]`, `LRA` in `[1, 50]`. Use `nextMasterTarget` for master
  retargeting; do not mutate `attemptTarget` inline. Add any new computed
  filter option to `computedFilterRanges`.
- **A feedback loop that adjusts a parameter from a measurement needs a bound
  and an exit condition.** If the clamped next target equals the current one,
  or a native master lands no closer to the acceptance window
  (`fullDemoAACMissDB`) than the one before, hand over to
  `recoverFullDemoAAC` instead of retrying.
- **Do not add a fallback stage without a test proving the earlier stages can
  reach it**, driven with the measured values of the incident (see
  `TestFullDemoMasterRetargetStaysWithinLoudnormRange`).
- Before tuning the retarget, check `normalization_type` of the master pass
  itself (the `measured_*:linear=true` filter, rerun to `-f null`). The
  measurement logs always say `dynamic` because they run without `measured_*`.
- Media Foundation (`aac_mf`) is a Windows component present on every edition
  except "N/KN". Do not propose bundling it.

### How to diagnose a user's render failure

- The UI shows a generic message. The real ffmpeg stderr is in remote
  telemetry: `bash scripts/telemetry-query.sh incident CH-XXXX-...` (needs
  `~/.config/cliphub/telemetry-agent.env`); `stats 24` gives fingerprints.
  Read `message` of the `pipeline.error` events with `class=render:variant`.
- Map the UI percentage to the stage with the weights in
  `internal/editor/short_pack.go` (`renderShort`) and
  `internal/editor/progress.go`: 76-82 % is the loudness/AAC master loop,
  83-86 % is AAC recovery, 87 %+ is delivery verification.
- Working-directory logs (`out/logs/program-*.txt`) are deleted with the temp
  workdir on failure unless `ZV_MEDIA_WORK_DIR` is set.

### Capture tail shortfall tolerance

`recording.FullDemoTailPadToleranceFrames` (2) accepts the one-frame shortfall
of a fractional round window and pads it with `tpad`.

- Do not raise the tolerance to hide real capture loss: a larger gap means
  HLAE dropped frames, not rounding.
- Audio needs no pad: every bus is already `apad`/`atrim`-ed.

## Full Demo overlay format (`internal/demooverlay`, `recapplan.Options`)

- `recapplan.Options.OverlaySource()` is the single resolver: the demo origin
  (`source_kind`) owns the intro/outro layout; `overlays.source` only adds
  FACEIT enrichment to a plain demo. `web/lib/full-demo-plan.ts`
  `fullDemoOverlaySource` mirrors it; keep both in sync.
- Do not add a plan option that the render never reads.
  `TestEveryChangeableFullDemoOptionIsRead` enforces it; reads in
  `internal/httpapi` and `cmd/zv` do not count.
- When a user blames a regression on feature X, check telemetry for the last
  successful run of that path before X shipped. If every run in between
  failed, the regression window is the whole gap.
- Toggling the outro scoreboard changes the final capture window: the planner
  extends the last round's tail (`EndReason` `scoreboard-tail`), only for a
  surviving POV with safe tail trim approved. The video is never padded with
  frozen frames to reach the full scoreboard length.

## HLAE pin and CS2 updates (`desktop/src/hlae-tool.*`)

CS2 updates break AfxHookSource2 often (three times in four days in 2026-09).

- Exit code 6 / `capture_incompatible` or an `Error - AfxHookSource2` dialog
  means HLAE vs CS2 build, not a ClipHub regression. Before bisecting our
  commits, compare `PatchVersion` in `game\csgo\steam.inf` with the CS2
  version in the pinned release's AfxHookSource2 changelog, and check the
  advancedfx issues and releases.
- Studio always runs the latest official HLAE: at boot `provisionHLAE`
  (`desktop/src/runtime-tools.ts`) reads advancedfx's `releases/latest`
  (`hlae-latest.ts`), verifies the archive against the sha256 GitHub
  publishes, and falls back to the bundled pin when the lookup, download or
  verification fails. The lookup runs only at boot. Studio passes
  `ZV_HLAE_PATH` and reinstalls on a digest mismatch, so users cannot swap in
  their own HLAE.
- `desktop-release.yml` runs `desktop/scripts/check-hlae-latest.mjs`, which
  fails the release unless `hlae-tool.json` is advancedfx's latest release.
  Bump the pin and prove it with a real capture on the current CS2 build
  (a kills plan, a `deathnotices` plan and a Full Demo subset) before tagging.
- `treeSha256` is the digest `runtime-tools.ts` computes over a fresh
  `Expand-Archive` of the release zip, not the zip sha256 and not the
  installed folder (HLAE writes `ffmpeg/ffmpeg.ini` there at runtime; it is
  excluded from the digest).
- Build an interim `AfxHookSource2.dll` only when advancedfx has no fixed
  release, only from a merged upstream commit, and go back to the official
  release as soon as it ships. `docs/incidents.md` has the build steps.
- Delivery decode proves the file decodes, not that it shows the game. When
  verifying a capture, check frames and bitrate (`blackdetect`,
  `signalstats`): the black-capture incident delivered 2.4 Mb/s against about
  40 Mb/s for a normal 1080p60 capture.

## Full Demo render lab (`zv-editor lab`)

A full render takes 20+ minutes, so check a render change one stage at a time
first.

1. With Studio (or `zv serve`) running, `zv full-demo lab-bundle --job <uuid>`
   writes `<data>/lab/<job>-gameplay-pov-60/` (about 5 GB for a full match).
   It needs one earlier render of the variant and never writes render state.
2. `zv-editor lab <mode> --bundle <dir>` runs one stage through the production
   code and writes `lab-evidence.json` under `<bundle>/lab-work/`:
   - `plan`: timeline items, overlay windows, transitions, loudness targets.
   - `commands`: every FFmpeg argv of the program, built but not run.
   - `item --index N --seconds 8`: renders one item prefix and measures
     frames, bitrate, luma and loudness, plus three PNG stills.
   - `audio`: program audio and the real master/AAC recovery loop.
   - `delivery --file <mp4>`: strict delivery check on any delivered file.

Without a real job, `zv-editor lab synth --out <dir> [--rounds N] [--size WxH]`
writes a synthetic bundle in seconds (`--plan` takes an approved snapshot with
the native HUD, no media assets and no FACEIT overlay).

- A synthetic bundle proves timing, commands, overlays and loudness
  mechanics, not how CS2 footage or real voice looks or sounds.
- The neon overlays need Studio's Chromium renderer: set
  `ZV_OVERLAY_RENDERER_PATH`, or every mode stops.
- `synth` writes the Full Demo editor arguments itself. When
  `RenderWorker.writeEditorInputs` changes them, update both
  (`TestSyntheticLabBundleArgumentsMatchAFullDemoRender`).
- Use `item` and `audio` as the runtime evidence for a render change, and
  look at the stills. A change the lab cannot reach (capture, HLAE,
  publication) still needs a real render.
- Nothing deletes `<data>/lab`; remove old bundles by hand.

## Render QA warnings are informational

- Do not reintroduce a human sign-off step, a `review_required` status or a
  warnings-based publish gate. If a warning signals a real defect, fail the
  render with a clear error instead.
- `RenderVariantStatusReview` and `job.StatusReviewRequired` remain only so
  old durable documents decode.

## Demo scan failures reach the user with their cause

- A new admission or scan failure needs a code (like `csgo_demo`,
  `not_a_demo`, `unreadable_demo`) and a line in `SCAN_FAILURE_HINTS`
  (`web/lib/demo-parse-flow.ts`), not another generic message.
- A `demo_incompatible` scan on a fresh CS2 demo usually means a CS2 update
  changed the demo format before demoinfocs caught up: check its releases
  before touching our parser code.

## Local test environment caveat

`TestFullDemoConcatsTwoFixtureRounds`,
`TestFullDemoOverlayCompositesOntoFixtureCapture` and the
`internal/demooverlay` `RenderPNGs` tests fail with FFmpeg 9.x because
`-filter_complex_script` was removed. The shipped FFmpeg is 8.1.2, where they
pass. Treat them as pre-existing and do not change the shipped command unless
the bundled FFmpeg is upgraded.

## Measuring performance

Use `bin/chperf` (C, `sh tools/chperf/build.sh`) before claiming anything is
faster or slower: `chperf snapshot` for the current picture, `chperf run` and
`chperf compare` for before/after (`chperf bench` for Go benchmarks).
`chperf captures` flags raw takes whose bitrate matches the black-capture
incident. See the `chperf` skill for what each number means.

## Releases

- Version lives in `desktop/package.json` and `landing/app/page.tsx`
  (`DOWNLOAD_URL`, `RELEASE_VERSION`). Bump both, commit
  `chore(release): prepare Studio X.Y.Z`, then push an annotated tag
  `vX.Y.Z` ("ClipHub Studio X.Y.Z"). `.github/workflows/desktop-release.yml`
  builds the installer and publishes the GitHub Release as `latest`, which
  is what the in-app updater reads.
