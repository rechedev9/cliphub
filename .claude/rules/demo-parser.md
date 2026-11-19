---
paths:
  - "go.mod"
  - "internal/parser/**"
  - "internal/demosafe/**"
  - "internal/tactical/**"
  - "internal/workers/library_reanalysis*"
  - "cmd/zv-demo-players/**"
---

# Demo parser

ClipHub reads demos with our own parser, `github.com/rechedev9/demo-parser-rsn` (import `.../pkg/demoinfocs`). It started as a fork of markus-wa/demoinfocs-golang v5.3.0 and now differs from it on purpose: odd demo data reads as zero instead of panicking. `go.mod` requires a published tag. `TestDemoParserIsATaggedRelease` (`cmd/zv/demo_parser_guard_test.go`) fails when upstream demoinfocs is linked again, when the parser is replaced by a local folder, or when `demoParserModule` in `internal/workers/library_reanalysis.go` names another module.

## Where it lives

- Repository: `rechedev9/demo-parser-rsn`, default branch `cliphub/v5`. Clone in `C:\Users\<user>\projects\demo-parser-rsn`: `origin` is ours, `upstream` is markus-wa with `tagOpt --no-tags`.
- A clone fetches upstream's old v0 and v1 tags, which the Go proxy would read as versions of our module and which block our next tag name. Delete them locally (keep ours), push one tag by name, and never `git push --tags`.
- Versions are semver tags on `cliphub/v5`, from `v1.0.0`. Never move a tag; the Go proxy caches it.

## How it reads demo data

- A `PropertyValue` getter never panics: no value reads as zero (`Handle()` as `constants.InvalidEntityHandleSource2`), and integer getters accept every width a decoder produces.
- A property the class lacks reads as zero through `sendtables.ValueOrZero`; `onPropUpdate` and `BindProperty` skip it. Use them instead of `PropertyValueMust` or `Property(name).X` in new code.
- A pawn, controller, player or event descriptor found by handle or ID is checked for nil before use.
- Malformed data (a truncated bitstream, a bad proto) still fails the parse; ClipHub classifies it.

## Change the parser

1. Commit on `cliphub/v5` with a unit test that fails without the change. `go test -short ./pkg/...` runs without upstream's demo submodule (`test/cs-demos`); the full suite needs it.
2. Push the branch and one new annotated tag, then `go get github.com/rechedev9/demo-parser-rsn@<tag>` here.
3. Prove it on real demos: build `zv` with main's `go.mod` and with the branch's, run `zv analysis tactical --demo <match.dem> --out <file>` with each, and compare the JSON (only `generated_at` may differ, unless the change is meant to move the output).
4. A new tag changes `parserVersion`, so library demos that failed as `demo_incompatible` scan again once after the upgrade.

A `demo_incompatible` scan on a fresh CS2 demo usually means a CS2 update changed the demo format. Upstream may already have the fix: port it to `cliphub/v5` by hand, since imports and code have diverged. The incidents behind this are in `docs/incidents.md`, **a pro demo panicked the parser when a player started a plant**.
