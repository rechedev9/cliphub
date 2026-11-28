---
paths:
  - "content/**"
  - "docs/content/**"
---

# Content product

`content/` is the owner's YouTube pipeline and is never part of the installer. What it is, its process, its state tables, its tools and its scheduled tasks are in [docs/content/README.md](../../docs/content/README.md); the decisions are in [ADR 0003](../../docs/adr/0003-content-vertical-and-monorepo.md), and [ADR 0004](../../docs/adr/0004-one-mcp-server.md) makes `cliphub-studio` the one MCP server and orders the port. Read them before you add a tool. This file holds only what to do and not do in the code; do not copy those two here.

## Boundaries

- Do not add an MCP server, a `.mcp.json` or a new tool to the TypeScript package. A new channel tool waits for the port of ADR 0004 and lands as a `channel_` tool of `zv-mcp`.
- Do not render from `content/`. Renders go through the GPU render tools of `zv-mcp`.
- Do not call the orchestrator API, open Studio's `jobs.db` or import Go code. Studio is reached through `content/src/studio/client.ts` alone, and a Studio fact that `zv-mcp` lacks is a new `zv-mcp` tool first: [MCP parity](mcp-parity.md).
- Do not add code, queues or state for a platform the README rules out.
- The repository is public and the state is the owner's: no state file, `content/config.local.json` or token is committed, and no config key holds a token. `content/src/config.test.ts` fails when a source file names a user's home folder or the config takes an unknown key.

## Tools over prompts

- Decide whether a step is a tool or a prompt by "The rule" of the README. Do not leave a step with one right answer in a skill or in a scheduled-task prompt.
- The thumbnail renderer is written against [docs/content/thumbnails.md](../../docs/content/thumbnails.md). A change to the look changes that spec in the same PR.
- Do not build a long job (a render, an upload) as one blocking tool call. "MCP tools today" in the README says what it waits for.
- A tool lands with its row in the tool table of the README and with `INSTRUCTIONS` in `content/src/mcp/server.ts` saying when to call it.

## State

- Do not store a stage or a status, and do not write a step of a map outside `record_map_event`. The README's "State" says why. Test: `content/src/mcp/state-tools.test.ts`.
- Change the schema by appending a migration in `content/src/state/store.ts`. Do not edit one that has run.

## Run and check

- `pnpm --dir content run cli <tool> [json]` runs a tool in process. No MCP client starts `pnpm --dir content run mcp` any more.
- Tests spawn a fake Studio MCP (`content/src/studio/fake-studio-server.ts`) and a temporary state file (`content/src/testing/harness.ts`). They never touch the real state or the real Studio.
- `pnpm --dir content run typecheck`, `lint` and `test:unit`. The release workflow does not run them, so `pnpm run check` is the only gate for this package.
