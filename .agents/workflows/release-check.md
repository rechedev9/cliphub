# Release check

Read-only checklist before a release. It proves the tree. It does not publish, tag, or tell anyone to publish. Tagging and the GitHub Release are `docs/incidents.md`, section **Releases**. The invariants are [Release](../rules/release.md).

Run this from a clean `main` that already contains the change.

## 1. Review the rules

Read `.agents/rules/` against the code and the `docs/incidents.md` sections they cite. A rule that drifted from the test, the linter or the incident gets its update in the same PR as the fix. This is the release review named in [Maintenance](../../AGENTS.md#maintenance).

## 2. Full local checks

```bash
go vet ./...
go test ./... -count=1 -timeout 30m
go run ./cmd/zv check
pnpm --dir web run typecheck
pnpm --dir web run lint
pnpm --dir web run test:unit
pnpm --dir web run build
pnpm --dir desktop run typecheck
pnpm --dir desktop run lint
pnpm --dir desktop run test:unit
```

`go test -short` is not this step. `-short` skips the slow FFmpeg renders and `TestNoDeadCode`. The `-timeout 30m` flag is required because `internal/editor` nears the 10 minute default on Windows.

## 3. Latest HLAE

```bash
node --no-warnings desktop/scripts/check-hlae-latest.mjs
```

The release needs `desktop/src/hlae-tool.json` to be advancedfx's latest release, proved with a real capture when the pin moves. [HLAE pin and capture](../rules/capture-hlae.md).

## 4. Dry run of the release workflow

From `main`, dispatch the workflow so it runs the checks and builds the installer. `dry_run` stays true:

```bash
gh workflow run desktop-release.yml --ref main -f dry_run=true
```

The `checks` and `windows-installer` jobs run only on `main` and on `v*` tags. The `publish` job runs on a tag push, or when a manual dispatch sets `dry_run` to false. This checklist leaves `dry_run` true. Watch the run with `gh run list --workflow desktop-release.yml` and `gh run view`. A green dry run is the end of this checklist.
