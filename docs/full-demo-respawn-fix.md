# Full Demo: respawn acquisition and fixed freeze

## Failure reproduced from local evidence

The reported Mirage demo (SHA256 `eb8853783b98a7899ab772eb75efb14729b9c628bb050aec3a37339f7638b8b1`) failed twice before `record-start-round-004`. The target was donk666; the observer was still on another player.

The previous voice-aware planner extended R4 to round_start, tick 25341. Independent parsing confirmed the target was dead through 25340 and alive from 25341. Camera warmup was therefore trying to select a dead pawn; the exact capture boundary gave the asynchronous spectator selection no time after respawn. This path predates optimization #158.

Native validation then exposed a second issue: CS2 prefixes `-condebug` lines with timestamps and HLAE wraps long JSON events across lines, including inside JSON keys. The evidence reader skipped those events and rejected an otherwise verified run as missing settings restoration.

## Fixed policy

- New plans keep exactly the **last two seconds before freeze_end** in every admitted round.
- Voices never extend the interval. Old draft fields are canonicalized to freeze=2, max=2, keep-freeze-voice=false, context=0.
- Manual edits can shorten the end but cannot change the start or remove part of the two-second freeze.
- POV warmup remains outside the approved video/audio interval. Sources without enough freeze for both safe acquisition and exactly two seconds are blocked explicitly, rather than silently changing the duration or dropping live action.
- The UI removes adjustable freeze, voice-extension and manual-start controls, and migrates old drafts.
- Legacy documents remain readable. Admission and script generation require the new planner and fixed-freeze coverage; reapproval is explicit, not an automatic rewrite of saved coverage.

## Recorder and evidence

The runtime can pause unrecorded preroll to acquire the target, requires two consecutive first-person/SteamID observations, then resumes before the exact approved start. Acquisition retries are bounded; timeout, missing target or a missed boundary fail closed. Pausing never occurs inside a recording. Strict live POV checks and certified death-tail trimming remain intact.

The reader strips only the recognized CS2 timestamp envelope and reassembles bounded token-bound JSON continuations. Unknown tokens, partial records, malformed/trailing JSON and oversized events remain rejected. Attestation output now uses real newlines, and successful settings restoration precedes the success marker.

An adjacent frame-boundary test exposed millisecond rounding selecting the first sponsor frame as a cover. Full Demo cover seeks now round down at microsecond precision instead of stepping past the approved gameplay frame.

## Verification evidence

Local diagnostics are kept under `.local/full-demo-rootcause/` (not committed; no recordings or user settings belong in Git):

- Native CS2/HLAE capture with the corrected recorder: **19/19 certified segments**, `capture_mode=real`, `capture_verified=true`, no capture warnings, and both runtime settings and settings files restored. This run preceded the final editorial request to fix freezes to two seconds; its preview retains the old longer editorial windows.
- Final two-second policy replanned against the same independent demo facts: **19/19 rounds at exactly 2.00 seconds**, no blockers. See `fixed-2s-real-plan.txt`.
- MIRV scenarios cover death, seek, respawn, deferred selection, refused pause/selection, missing target, strict live drift, restoration failure and a no-acquisition-gate negative control. Successful operations retain their approved start ticks.
- Go tests cover fixed policy at 64/100/128 Hz, voice/draft/manual overrides, insufficient freeze, audio/sample clocks, legacy retry admission and native console wrapping.
- Browser tests cover the fixed, noneditable controls and the existing Full Demo constructor. Synthetic media canaries retain video/audio/sponsor/cover boundary checks.

The diagnostic preview is not a Studio library render and does not include the final team-voice/overlay composition. The installed Studio binary is not replaced by these source changes.
