---
paths:
  - ".github/workflows/**"
  - "desktop/src/hlae-tool.json"
  - "desktop/scripts/check-hlae-latest.mjs"
  - "desktop/package.json"
  - "landing/lib/site.ts"
---

# Release

This file holds the bar a branch clears and the release steps. The checklist that runs the bar and stops at a dry run is [Release check](../skills/release-check/SKILL.md).

## What has to be green before a tag

- The checks for what you touched: [Build and check](../../AGENTS.md#build-and-check).
- The full Go suite, locally: `go test ./... -count=1 -timeout 30m`. The release workflow runs Go with `-short`, which skips the slow FFmpeg renders and `TestNoDeadCode`.
- The pin is the latest official HLAE, proved with a real capture when the pin moves. [HLAE pin and capture](capture-hlae.md).
- A dry run of the release workflow from `main`.

## Steps

1. Bump the version in `desktop/package.json` and in `landing/lib/site.ts` (`DOWNLOAD_URL`, `RELEASE_VERSION`, `INSTALLER_SIZE`). `desktop/scripts/landing-version.test.mjs` fails when the two disagree.
2. Commit `chore(release): prepare Studio X.Y.Z` on `main` and push it.
3. Push an annotated tag `vX.Y.Z` ("ClipHub Studio X.Y.Z"). The workflow builds the installer and publishes the GitHub Release as `latest`, which the in-app updater reads.
4. Once the release is published, deploy ClipMind and the landing from the same commit: [ClipMind and the VPS](clipmind.md). Until then web users run older code than Studio, and the sync contract can refuse Studio.

## The one hosted workflow

`.github/workflows/desktop-release.yml` is the only hosted workflow. It runs the checks (Go with `-short`, web and its ClipMind edition build, desktop, landing export, MCP Apps and the embedded view), builds the installer, and publishes only when the checks pass.

- There is no CI on pushes or pull requests. Do not add a `pull_request` trigger, a branch `push` trigger or a second workflow.
- `dry_run` defaults to true on a manual dispatch: the run builds the installer and does not publish. `publish` runs only on a `v*` tag push or with `dry_run=false`.
- The `checks` and `windows-installer` jobs run only on `main` and `v*` tags, so dispatch the dry run from `main`.
