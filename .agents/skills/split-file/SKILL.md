---
name: split-file
description: Split a Go or TypeScript file that is past or against the 1000-line cap, by responsibility, and drop a frozen entry that now fits. Use when a file approaches 1000 lines or TestGoFileLength or oxlint max-lines fails.
---

# Split a file

The cap is a golden rule in `AGENTS.md`. `TestGoFileLength` (`cmd/zv/file_length_test.go`) enforces it for Go. `max-lines` enforces it for TS in `web/.oxlintrc.json`, `desktop/.oxlintrc.json`, `landing/.oxlintrc.json` and `mcp-apps/.oxlintrc.json`.

## Steps

1. Split by responsibility. Each new file, or a new package when there is a real API boundary, owns one concern. Name it for that concern, such as `handlers_library.go` and `handlers_streams.go`. A cut by line count (`handlers2.go`) does not count.
2. Files already past the cap are frozen. Their sizes live in `oversizedGoFiles` in `cmd/zv/file_length_test.go` and in the `overrides` of the oxlint configs above. A frozen file may shrink. It may not grow. When a file now fits under 1000 lines, lower or drop its frozen entry in the same change. Do not raise a max.
3. Move a helper that only tests call into a `_test.go` file. `TestNoDeadCode` treats a production function that only tests call as dead. A helper that only `_windows.go` code calls belongs in a `_windows.go` file.

## Checks

```bash
go test ./cmd/zv -run TestGoFileLength -count=1
go vet ./...
```

When the split touched TS, lint the package that owns the file:

```bash
pnpm --dir web run lint
pnpm --dir desktop run lint
```

Use `pnpm --dir landing run lint` or `pnpm --dir mcp-apps run lint` when that package was the one split. The split is finished when `TestGoFileLength` is green and, for a TS split, that package's `lint` is green.
