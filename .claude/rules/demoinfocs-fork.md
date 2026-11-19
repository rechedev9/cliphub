---
paths:
  - "go.mod"
  - "internal/parser/**"
  - "internal/demosafe/**"
  - "internal/tactical/**"
  - "internal/workers/library_reanalysis*"
  - "cmd/zv-demo-players/**"
---

# demoinfocs fork

ClipHub reads demos with our fork of demoinfocs-golang, not with upstream. Imports keep the upstream path (`github.com/markus-wa/demoinfocs-golang/v5`); the `replace` in `go.mod` points it at a tag of `github.com/rechedev9/demoinfocs-golang/v5`. `TestDemoinfocsComesFromTheFork` (`cmd/zv/demo_parser_guard_test.go`) fails when the `replace` is dropped or points at a local folder.

## Where it lives

- Repository: `rechedev9/demoinfocs-golang`, clone in `C:\Users\<user>\projects\demoinfocs-golang` (`origin` is the fork, `upstream` is markus-wa).
- Maintained branch: `cliphub/v5`, cut from upstream `v5.3.0`. The fork's `main` mirrors upstream, which is on v6 and has a different module path. Moving to v6 is its own task.
- Tags: `v<upstream base>.<patch+1>-cliphub.<n>`, for example `v5.3.1-cliphub.1`. Never move a tag; the Go module proxy caches it.

## Fix the parser

1. Commit on `cliphub/v5` with a unit test that fails without the fix. `go test -short ./pkg/...` runs without the upstream demo submodule (`test/cs-demos`, GitLab LFS); the full suite needs it.
2. Push the branch, push a new annotated tag, and set the `replace` to it. `go mod tidy` updates `go.sum`.
3. Prove it on the demo that failed: `go run ./cmd/zv-demo-players --demo <match.dem>` with main's `go.mod` and with the branch's.
4. A new tag changes `parserVersion` (`internal/workers/library_reanalysis.go`), so library demos that failed as `demo_incompatible` scan again once after the upgrade.

The bug that pushed us to the fork is in `docs/incidents.md`, **a pro demo panicked the parser when a player started a plant**.

## Upstream releases

A `demo_incompatible` scan on a fresh CS2 demo usually means a CS2 update changed the demo format. Check upstream's v5 releases first: merge the upstream tag into `cliphub/v5`, keep our commits, tag, and bump the `replace`. Fix it in the fork only when upstream has no fix.
