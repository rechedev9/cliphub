# Full Demo temporary disk usage

On 2026-09-12, Studio 3.0.2 (`8a94581e07da`) captured all 19 rounds of
`grD2.dem` successfully, then failed while mastering the complete video.
Job: `294b12c3-0944-4c73-a12d-a0e341937bac`.

The first causal error in `studio.log`, at 08:51:16 local time, is FFmpeg's
`No space left on device` while writing the final MP4. The input program is
1920x1080 H.264 at 60 fps, with float PCM stereo audio. The failed attempt
had written about 3.77 GiB when the disk filled. The screenshot's generic
failure message hides this underlying storage error.

The downloaded demo's SHA-256 matches the approved job:
`9fa18e5865b4daf8d23690c8dff0f80297ad689b9f9b81a3a5cb6be3044dbff9`.
Its saved capture revision `24838719-eb45-4330-8c98-dcf21cb1a1aa` contains
19 rounds and reports `capture_mode=real`, `capture_verified=true`.

## Cause and change

The editor retained every generated audio bus and prepared timeline item until
the worker removed its entire temporary directory. During mastering, those
files coexisted with the complete lossless-audio program and atomic MP4
attempts. None of the prepared buses or items are read by the mastering stage.

Release generated media at its last successful consumer:

1. After all timeline items and their concat list are committed, remove the
   normalized voice/music WAV files. Each item already contains its mixed PCM.
2. After the full program is atomically committed, remove the prepared NUT
   items. All audio mastering attempts read the committed program.
3. After decoded audio acceptance, exact-frame/full-decode delivery checks and
   completed-evidence validation, remove the program before publication.

The cleanup validates the whole list of paths before deleting any file,
accepts only regular files directly inside the relevant attempt directory,
and reports filesystem failures. Original captures, source assets, approval
documents, logs, and HUD telemetry are retained. No encoder setting, timeline,
audio target, retry limit, or validation tolerance changes.

## Regression validation

- The real FFmpeg media canary fails against the previous retention behavior:
  `prepared audio still consumes disk before program assembly: ...voice-0.wav`.
- With the fix, `go test ./internal/editor -run TestFullDemo -count=1 -timeout 10m`
  passes on the final rebased branch (315.240 seconds). The canary checks that original sources and
  diagnostics remain, removes intermediate items before mastering, then checks
  exact frames, full decode, audio acceptance and sponsor/music semantics.
- Worker-to-editor CLI contract and short-source rejection tests pass:
  `go test ./internal/workers -run 'TestFullDemoWorkerArgumentsThroughEditorCLI|TestFullDemoEditorChecksActualSourceFramesBeforePreparation' -count=1 -timeout 3m`.
- `go vet ./internal/editor` passes.

## Installed E2E

The installed E2E executable was built from the exact installed 3.0.2 release
in a separate worktree. The final `fix/full-demo-disk-usage` branch is rebased
onto `origin/main`; the original dirty checkout is preserved. The original
installed editor is backed up under
`.local/full-demo-disk-20260912/zv-editor-original-3.0.2.exe`.
Only the editor executable is replaced for this local verification; no release
is published and the desktop application remains version 3.0.2.

The application's **Reintentar** button started the real saved Dust2 approval,
with its existing capture, HUD Mono, round transitions and five voice tracks.
The render completed and published revision
`0cf3ac8c-ca8a-4b65-b3fa-b3973d5444f6`. Its strict delivery evidence records
73,637 frames, 1,227.283333 seconds, stereo 48 kHz audio, complete decode and
SHA-256 `853841d933ae607f9db23ac8967c8ced84c1089170847eecb47beb60563fa647`.
Decoded AAC passed the approved loudness policy. The player opened in Studio,
advanced from 00:00 to 00:13, and was paused.

Disk usage was sampled every ten seconds. The render started with 23.88 GiB
free and reached a minimum of 6.67 GiB. The five generated WAV buses were
released before full-program assembly and all 19 prepared items were released
before mastering. After the worker cleaned its attempt directory, 17.95 GiB
was free. All 19 captured sources had the same SHA-256 values before and after.

Studio marks the successful video `LISTO` with `REVISIÓN QA`: its generic
quality scan reports frozen frames. This is consistent with the approved fixed
two-second freeze at each round start and is separate from delivery validation.
Playback is available for inspection; download remains behind the existing QA
review gate. This verification does not silently approve that review.
