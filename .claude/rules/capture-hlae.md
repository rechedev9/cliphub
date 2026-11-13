---
paths:
  - "cmd/zv-recorder/**"
  - "internal/recording/**"
  - "internal/demostart/**"
  - "desktop/src/hlae-*"
  - "desktop/src/runtime-tools.ts"
  - "desktop/scripts/check-hlae-latest.mjs"
---

# HLAE pin and capture

Capture runs CS2 through HLAE (`zv-recorder`). The history is in `docs/incidents.md`, section **HLAE pin and CS2 updates**. Read the incident named below before you change the code it covers.

## Pin and CS2 updates

- The pin is `desktop/src/hlae-tool.json`. At boot, `provisionHLAE` (`desktop/src/runtime-tools.ts`) uses advancedfx's latest release when it is newer than the pin, and falls back to the pin when the lookup or its digest check fails. A CS2 update while Studio is open needs a Studio restart.
- A release is refused unless the pin is advancedfx's latest release: `.github/workflows/desktop-release.yml` runs `node --no-warnings desktop/scripts/check-hlae-latest.mjs`. Bump the pin and prove it with a real capture before tagging. [Release](release.md).
- Exit code 6 / `capture_incompatible` means HLAE against the CS2 build, not a ClipHub regression. Compare `PatchVersion` in `game\csgo\steam.inf` with the AfxHookSource2 changelog of the pinned release before you bisect ClipHub commits.

## Capture pitfalls

- **Capture tail shortfall tolerance** (under Full Demo audio mastering): do not widen `recording.FullDemoTailPadToleranceFrames` (`internal/recording/full_demo_frames.go`). The incident says why.
- **Black first-person capture** and **picture frozen after a round end**: decodable delivery and correct frame counts do not prove the picture shows the game. `recording.FrozenPictureWarnings` is the check. Do not swap it for `freezedetect` or `framemd5`; the incident **frozen picture certified as a good capture** says why.
- **CS2 crashed in the playdemo rewind to tick 0**: after `unplayable_start`, the worker and `zv short` relaunch `zv-recorder` once with `--tick0-demo-copy`. `internal/demostart` writes that copy. The stored demo and its SHA-256 stay the original.
- **Demo scan failures reach the user with their cause**: a `demo_incompatible` scan on a fresh CS2 demo usually means demoinfocs lags a demo-format change. Check its releases before you touch the parser or the HLAE pin.

Capture needs Steam and CS2 on the machine. Use demo files from outside the repo. The manual check is [Verify in Studio](../skills/verify-in-studio/SKILL.md).
