---
name: release-check
description: Pre-release checklist for ClipHub Studio. Rereads the rules, runs the full local checks and the latest-HLAE check, and dispatches a dry run of desktop-release.yml. Use when asked to prepare, check or dry-run a Studio release, or before the version bump and the vX.Y.Z tag. Never publishes or tags.
---

# Release check

This checklist proves the tree. It does not bump the version, tag or publish; those steps are in `docs/incidents.md`, section **Releases**. The bar it checks is [Release](../../rules/release.md).

Run it from a clean `main` that already contains the change.

## 1. Reread the rules

Read `.claude/rules/` against the code and the `docs/incidents.md` sections they cite. Report a rule that drifted from the test, the linter or the incident; its fix goes in a PR before the release. This is the review named in [Maintenance](../../../AGENTS.md#maintenance).

## 2. Full local checks

```bash
go vet ./...
go test ./... -count=1 -timeout 30m
go run ./cmd/zv check
node --test scripts/capturelab/*.test.mjs
pnpm --dir web run typecheck
pnpm --dir web run lint
pnpm --dir web run test:unit
pnpm --dir web run build
pnpm --dir desktop run typecheck
pnpm --dir desktop run lint
pnpm --dir desktop run test:unit
```

Do not use `go test -short` here: the release workflow already runs that, and it skips the slow FFmpeg renders and `TestNoDeadCode`.

## 3. Latest HLAE

```bash
node --no-warnings desktop/scripts/check-hlae-latest.mjs
```

If it fails, the pin in `desktop/src/hlae-tool.json` is behind. Stop: bumping it needs a real capture. [HLAE pin and capture](../../rules/capture-hlae.md).

## 4. Dry run of the release workflow

```bash
gh workflow run desktop-release.yml --ref main -f dry_run=true
gh run list --workflow desktop-release.yml --limit 1
gh run watch <run-id>
```

Keep `dry_run=true` and `--ref main`; [Release](../../rules/release.md) says why.

The check is finished when steps 2 and 3 are green and the dry run is green. Report the run URL and anything that drifted in step 1.
