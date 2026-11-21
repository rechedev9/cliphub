---
paths:
  - "web/app/**"
  - "web/components/**"
  - "cmd/zv-mcp/**"
  - "docs/studio-mcp.md"
---

# MCP parity

Every user-facing feature in the Studio UI has a matching tool in `cmd/zv-mcp`. UI users and assistant users are both first class. `TestEveryRouteHasAnMCPTool` (`internal/httpapi`) fails when an orchestrator route has a path word that no string in `cmd/zv-mcp` or `internal/tuiclient` names. A route without a tool goes in `routesWithoutMCPTool` only when it is a file a screen draws, has no screen, or is not a user action; the frozen `parityGap` entries only shrink. The test sees words, not tools, so a new feature on an old path still needs [Diff review](../skills/diff-review/SKILL.md).

A change that adds or changes a UI feature updates, in the same change:

- the tool under `cmd/zv-mcp`
- `serverInstructions` in `cmd/zv-mcp/main.go`
- the tool table in `docs/studio-mcp.md`

Do not ship the UI first and the tool in a later PR. A change to what `studio_status` returns also updates the issue docs: [Issues](../../AGENTS.md#issues).

The recipe is [Feature with MCP](../skills/feature-with-mcp/SKILL.md).
