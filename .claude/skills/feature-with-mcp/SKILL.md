---
name: feature-with-mcp
description: Add or change a Studio UI feature together with its MCP tool, serverInstructions and the docs/studio-mcp.md table. Use when a task adds or changes a user-facing screen, flow, button or option under web/app or web/components, or adds or changes a tool in cmd/zv-mcp.
---

# Feature with MCP

The rule is [MCP parity](../../rules/mcp-parity.md): the UI and the MCP tool change together.

## Steps

1. Change the Studio UI in `web/` (Spanish copy). Scope it for the installed Windows app.
2. Add or update the matching tool in `cmd/zv-mcp`.
3. Update `serverInstructions` in `cmd/zv-mcp/main.go` so an assistant sees the new flow.
4. Update the tool table in `docs/studio-mcp.md`.
5. If the feature changes what `studio_status` returns, or what a bug report must include, update `CONTRIBUTING.md` and `.github/ISSUE_TEMPLATE/bug-report.md` in the same change.

## Checks

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

Also run the Go tests of any other package you touched. Then check the change by hand: [Verify in Studio](../verify-in-studio/SKILL.md).

The feature is finished when those commands are green, steps 2 to 4 are in the same diff as the UI change, and the hand check passed.
