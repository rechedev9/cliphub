# Release

Release steps (version files, the prepare commit, the annotated tag) are in `docs/incidents.md`, section **Releases**. This file is the bar a branch has to clear before that. Publishing is that section's job. The checklist that runs the checks and stops at a dry run is [Release check](../workflows/release-check.md).

## What has to be green

- The checks for what you touched, from [the index](../../AGENTS.md#build-and-check).
- The full Go suite, locally, with `-timeout 30m`: `go test ./... -count=1 -timeout 30m`. `go test -short` skips the slow FFmpeg renders and `TestNoDeadCode`. `.github/workflows/desktop-release.yml` itself runs `go test -short ./... -count=1 -timeout 20m` on `windows-latest`, plus `go vet ./...`, `go run ./cmd/zv check`, the capture-simulator contract, and web and desktop `typecheck`, `lint` and `test:unit`. The full suite is the local run, before a release.
- The latest official HLAE. The workflow runs `node --no-warnings desktop/scripts/check-hlae-latest.mjs` and fails the release unless `desktop/src/hlae-tool.json` is advancedfx's latest release. Bump the pin and prove it with a real capture before tagging. The pin rules are [HLAE pin and capture](capture-hlae.md) and `docs/incidents.md` (**HLAE pin and CS2 updates**, including **a release shipped on a pin CS2 had already broken**).

## Dry run

From `main`:

```bash
gh workflow run desktop-release.yml --ref main -f dry_run=true
```

`dry_run` defaults to true. The workflow's `publish` job runs on a tag push, or when a manual run sets `dry_run` to false (`if: github.event_name == 'push' || inputs.dry_run == false` in `.github/workflows/desktop-release.yml`). A manual run with the default builds the installer and does not publish. The `checks` and `windows-installer` jobs run only for `refs/heads/main` and `v*` tags, so the dry run is dispatched from `main`.

There is no CI on pushes or pull requests. This workflow is the only hosted one. Do not add a second workflow.
