---
paths:
  - ".github/workflows/**"
  - "desktop/src/hlae-tool.json"
  - "desktop/package.json"
---

# Release

Release steps (version files, the prepare commit, the annotated tag) are in `docs/incidents.md`, section **Releases**. This file is the bar a branch has to clear before that. Publishing is that section's job. The checklist that runs the checks and stops at a dry run is [Release check](../skills/release-check/SKILL.md).

## What has to be green

- The checks for what you touched, from [the index](../../AGENTS.md#build-and-check).
- The full Go suite, locally: `go test ./... -count=1 -timeout 30m`. The release workflow only runs `-short`, which skips the FFmpeg renders and `TestNoDeadCode`.
- The latest official HLAE, proved with a real capture when the pin moves. [HLAE pin and capture](capture-hlae.md).

## Dry run

From `main`:

```bash
gh workflow run desktop-release.yml --ref main -f dry_run=true
```

`dry_run` defaults to true: the run builds the installer and does not publish. `publish` runs only on a tag push or with `dry_run=false`. The `checks` and `windows-installer` jobs run only on `main` and `v*` tags, so dispatch the dry run from `main`.

Do not add a `pull_request` trigger to a test or build workflow. Hosted tests run on release, `workflow_dispatch` or a nightly `schedule`.
