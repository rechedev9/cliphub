---
name: feature-with-mcp
description: Add or change a Studio UI feature and its MCP tool, server instructions and docs/studio-mcp.md in the same change. Use when a user-facing Studio screen, flow or control changes.
---

# Feature with MCP

UI users and assistant users are both first class. The rule is [MCP parity](../../rules/mcp-parity.md).

## Steps

1. Change the Studio UI in `web/` (Spanish copy). Scope it for the installed Windows app.
2. Add or update the matching tool in `cmd/zv-mcp`.
3. Update `serverInstructions` in `cmd/zv-mcp/main.go` so an assistant sees the new flow.
4. Update the tool table in `docs/studio-mcp.md`.
5. If the feature changes what `studio_status` returns, or what a bug report must include, update `CONTRIBUTING.md` and `.github/ISSUE_TEMPLATE/bug-report.md` in the same change.

## Checks

Run the packages you touched, then the shared check:

```bash
go test ./cmd/zv-mcp -count=1
go vet ./...
go run ./cmd/zv check
```

When `web/` changed:

```bash
pnpm --dir web run typecheck
pnpm --dir web run lint
pnpm --dir web run test:unit
pnpm --dir web run build
```

One web test: `pnpm --dir web exec node --test <path>.test.ts`.

A UI change is finished when those commands are green and the three MCP edits are in the same diff. The manual check, when the change is user-visible, is [Verify in Studio](../verify-in-studio/SKILL.md).
