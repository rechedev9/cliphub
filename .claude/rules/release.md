---
paths:
  - ".github/workflows/**"
  - "desktop/src/hlae-tool.json"
  - "desktop/scripts/check-hlae-latest.mjs"
  - "desktop/package.json"
  - "landing/lib/site.ts"
  - "data/music/catalog.json"
  - "desktop/scripts/seed-bundled-music.mjs"
---

# Release

Release steps (version files, the prepare commit, the annotated tag) are in `docs/incidents.md`, section **Releases**. This file is the bar a branch clears before that. The checklist that runs it and stops at a dry run is [Release check](../skills/release-check/SKILL.md).

## What has to be green before a tag

- The checks for what you touched: [Build and check](../../AGENTS.md#build-and-check).
- The full Go suite, locally: `go test ./... -count=1 -timeout 30m`. The release workflow runs Go with `-short`, which skips the slow FFmpeg renders and `TestNoDeadCode`.
- The pin is the latest official HLAE, proved with a real capture when the pin moves. [HLAE pin and capture](capture-hlae.md).
- A dry run of the release workflow from `main`.

## The one hosted workflow

`.github/workflows/desktop-release.yml` is the only hosted workflow. It runs the checks (Go with `-short`, web, desktop), builds the installer, and publishes only when the checks pass.

- There is no CI on pushes or pull requests. Do not add a `pull_request` trigger, a branch `push` trigger or a second workflow.
- `dry_run` defaults to true on a manual dispatch: the run builds the installer and does not publish. `publish` runs only on a `v*` tag push or with `dry_run=false`.
- The `checks` and `windows-installer` jobs run only on `main` and `v*` tags, so dispatch the dry run from `main`.

## Bundled music

The installer bundles the local-only Suno tracks (catalog entries without a `downloadUrl`). Their audio is gitignored; `node desktop/scripts/seed-bundled-music.mjs` fetches it from the `music-assets-1` GitHub release and checks each file against the `sha256` in `data/music/catalog.json`, and `assemble.mjs` refuses a missing or different file.

To add or change a track: add its entry with the `sha256` of the new file, upload every local-only track to a new `music-assets-N` release created with `--prerelease --latest=false` (so the in-app updater never reads it), and move `MUSIC_RELEASE_URL` to that tag in the same PR. A published `music-assets-N` is never edited.
