---
paths:
  - "web/app/**"
  - "web/components/**"
  - "cmd/zv-mcp/**"
  - "docs/studio-mcp.md"
---

# MCP parity

Every user-facing feature in the Studio UI has a matching tool in `cmd/zv-mcp`. UI users and assistant users are both first class. No test enforces this; [Diff review](../skills/diff-review/SKILL.md) checks it.

A change that adds or changes a UI feature updates, in the same change:

- the tool under `cmd/zv-mcp`
- `serverInstructions` in `cmd/zv-mcp/main.go`
- the tool table in `docs/studio-mcp.md`

Do not ship the UI first and the tool in a later PR. A change to what `studio_status` returns also updates the issue docs: [Issues](../../AGENTS.md#issues).

The recipe is [Feature with MCP](../skills/feature-with-mcp/SKILL.md).
