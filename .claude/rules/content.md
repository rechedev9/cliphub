---
paths:
  - "content/**"
  - "docs/content/**"
  - ".mcp.json"
---

# Content product

`content/` is the owner's YouTube pipeline: the `cliphub-content` stdio MCP server, a CLI over the same tools and one SQLite state file. It is never part of the installer. The process, the state tables, the tool table and the scheduled tasks are in [docs/content/README.md](../../docs/content/README.md); the decisions and the order of the remaining PRs are in [ADR 0003](../../docs/adr/0003-content-vertical-and-monorepo.md). Read both before you add a tool. Do not copy them here.

## Boundaries

- It reaches Studio only by spawning `zv-mcp` (`content/src/studio/client.ts`). It never calls the orchestrator API, opens Studio's `jobs.db` or imports Go code. A Studio fact it needs and `zv-mcp` lacks is a new `zv-mcp` tool first: [MCP parity](mcp-parity.md).
- YouTube is the only platform. Do not add TikTok or Instagram code, queues or state.
- The repository is public and the state is the owner's. The state file, `content/config.local.json` and every token stay outside the repo; the config file never holds a token and refuses an unknown key. `content/src/config.test.ts` fails when a source file names a user's home folder.

## Tools over prompts

- A step with one right answer is a TypeScript tool. The model supplies only the creative inputs, and the tool validates them before it records anything.
- A tool replaces its skill step once it passes the golden tests of its spec. For thumbnails that spec is [docs/content/thumbnails.md](../../docs/content/thumbnails.md): the renderer is written against it, and a change to the look changes the spec in the same PR.
- A long job (a render, an upload) waits for MCP Tasks on the server side of the SDK. Do not build it as one blocking tool call.
- A tool lands with its row in the tool table of `docs/content/README.md` and with `INSTRUCTIONS` in `content/src/mcp/server.ts` saying when to call it.

## State

- A map's stage and a Short's status are derived in `content/src/state/model.ts` and never stored. `record_map_event` is the only writer of a step, and it refuses an illegal one. Test: `content/src/mcp/state-tools.test.ts`.
- The schema changes by appending a migration in `content/src/state/store.ts`. A legacy field without a column stays in `legacy_json`, so an import loses nothing.
- `import_legacy_state` only reads the legacy folders and is a dry run unless `write` is true.

## Run and check

- `pnpm --dir content run cli <tool> [json]` runs a tool in process; `pnpm --dir content run mcp` is the server that `.mcp.json` starts.
- Tests spawn a fake Studio MCP (`content/src/studio/fake-studio-server.ts`) and a temporary state file (`content/src/testing/harness.ts`). They never touch the real state or the real Studio.
- `pnpm --dir content run typecheck`, `lint` and `test:unit`. The release workflow does not run them, so `pnpm run check` is the only gate for this package.
