---
name: split-file
description: Split a Go or TypeScript file by responsibility to stay under the 1000-line cap. Use when TestGoFileLength fails, oxlint max-lines fails, a file you are editing nears 1000 lines, or a frozen file (oversizedGoFiles, an oxlint max-lines override) would grow.
---

# Split a file

`TestGoFileLength` (`cmd/zv/file_length_test.go`) enforces the cap for Go. `max-lines` enforces it for TS in `web/.oxlintrc.json`, `desktop/.oxlintrc.json`, `landing/.oxlintrc.json` and `mcp-apps/.oxlintrc.json`.

## Steps

1. Split by responsibility. Each new file, or a new package when there is a real API boundary, owns one concern and is named for it, such as `handlers_library.go` and `handlers_streams.go`. A cut by line count (`handlers2.go`) does not count.
2. Files already past the cap are frozen at their size: `oversizedGoFiles` in `cmd/zv/file_length_test.go` for Go, a `max-lines` entry under `overrides` in the package's oxlint config for TS. A frozen file may shrink, never grow. Do not raise a max and do not add a new frozen entry.
3. When a frozen file shrinks, lower its entry in the same change. When it fits under the cap, drop the entry.
4. Keep the dead-code rule while you move code: a helper that only tests call goes to a `_test.go` file, and a helper that only `_windows.go` code calls goes to a `_windows.go` file.

## Checks

```bash
go test ./cmd/zv -run TestGoFileLength -count=1
go vet ./...
```

When functions moved between files, also run `go test ./cmd/zv -run TestNoDeadCode -count=1`. For a TS split, lint the package that owns the file: `pnpm --dir <pkg> run lint` (`web`, `desktop`, `landing` or `mcp-apps`).

The split is finished when those checks and the tests of the package you split are green.
