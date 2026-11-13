---
paths:
  - "web/app/**"
  - "web/components/**"
  - "cmd/zv-mcp/**"
  - "docs/studio-mcp.md"
---

# MCP parity

Every user-facing feature in the Studio UI has a matching tool in `cmd/zv-mcp`. UI users and assistant users are both first class.

A change that adds or changes a UI feature adds or updates, in the same change:

- the tool under `cmd/zv-mcp`
- `serverInstructions` in `cmd/zv-mcp/main.go`
- the tool table in `docs/studio-mcp.md`

`studio_status` is registered in `cmd/zv-mcp/tools.go`. `CONTRIBUTING.md` and `.github/ISSUE_TEMPLATE/bug-report.md` stay in step with the PR body sections and with what `studio_status` returns.

The recipe is [Feature with MCP](../skills/feature-with-mcp/SKILL.md). [Diff review](../skills/diff-review/SKILL.md) checks the three edits on any UI diff.
