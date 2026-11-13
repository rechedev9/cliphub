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

Capture runs CS2 through HLAE (`zv-recorder`). The history is `docs/incidents.md`, section **HLAE pin and CS2 updates**. This file names the invariants and where they are enforced.

## Pin and CS2 updates

- Studio boots the latest official HLAE when GitHub's `releases/latest` is newer than the bundled pin, and falls back to the pin when the lookup or its digest check fails. `provisionHLAE` is in `desktop/src/runtime-tools.ts`. The lookup runs at boot; a CS2 update while Studio is open is picked up by restarting Studio.
- A release is refused unless the pin is advancedfx's latest release: `node --no-warnings desktop/scripts/check-hlae-latest.mjs`, which `.github/workflows/desktop-release.yml` runs. Bump the pin and prove it with a real capture before tagging. See [Release](release.md).
- Exit code 6 / `capture_incompatible` means HLAE versus the CS2 build. Compare `PatchVersion` in `game\csgo\steam.inf` with the pinned release's AfxHookSource2 changelog before bisecting ClipHub commits.

## Capture pitfalls

Each of these is a rule in `docs/incidents.md`. Read that section before changing the tolerance, the certificate or the relaunch.

- **Capture tail shortfall tolerance** (under Full Demo audio mastering). The constant is `recording.FullDemoTailPadToleranceFrames` in `internal/recording/full_demo_frames.go`. The incident states why the tolerance stays where it is.
- **Black first-person capture** and **picture frozen after a round end**. Delivery decode and frame counts do not prove the picture shows the game. `recording.FrozenPictureWarnings` is the check; the incident **frozen picture certified as a good capture** records why `freezedetect` and `framemd5` are the wrong measures.
- **CS2 crashed in the playdemo rewind to tick 0**. After `unplayable_start`, the worker and `zv short` relaunch once with `--tick0-demo-copy` (`internal/demostart`). The stored demo and its SHA-256 stay the original.
- **Demo scan failures reach the user with their cause**. A `demo_incompatible` scan on a fresh CS2 demo is usually demoinfocs lagging a demo-format change, which is the parser, not the HLAE pin.

Capture needs Steam and CS2 on the machine. Use demo files from outside the repo. A new Studio profile shows the first-run guide and fetches HLAE on first boot. The manual check is [Verify in Studio](../skills/verify-in-studio/SKILL.md).
