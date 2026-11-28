---
status: accepted
---

# One MCP server: the channel tools move into `cliphub-studio`

ADR 0003 made the content product its own stdio MCP server, `cliphub-content`, written in TypeScript, so that the owner's channel tools never reach the users of the Studio installer. The owner does not want several MCP servers: an assistant that renders a video, plans the channel and reads a demo should see one server with distinct tools. So ClipHub has **one MCP server, `cliphub-studio` (`cmd/zv-mcp`)**, and the channel tools become owner-only tools inside it. This replaces the "Content product" bullet of ADR 0003 and the parts of its PR plan that build `cliphub-content`; the rest of ADR 0003 stands.

## Decision

- **One server.** `cmd/zv-mcp` serves the Studio tools and the channel tools (essay renders, thumbnails, metrics, the planner, YouTube upload, the map state). The `cliphub-content` server and the root `.mcp.json` that started it are gone. Channel tools carry the `channel_` prefix (`channel_status`, `channel_list_maps`, ...), so they never read as Studio features in the same tool list.
- **Owner gate at runtime.** `zv-mcp` registers the channel tools only when it starts with `CLIPHUB_CONTENT_CONFIG` set to a readable channel config that parses with only known keys. Without it, `tools/list`, `serverInstructions`, `search_api` and `run_script` have no channel tool and no word about the channel. The owner sets the variable in their own `cliphub-studio` registration; the config snippet and `claude mcp add` command that Studio's Ajustes copies never include it. A `cmd/zv-mcp` test compares the tool list with and without the variable.
- **Go, ported into `zv-mcp`.** The channel core (config, map model, store, legacy import) is ported from `content/` to `internal/content`, and the tools live in `cmd/zv-mcp`. The TypeScript package is deleted in the same PR as the port.
- **Studio is reached the way the Studio tools reach it.** Channel tools call the orchestrator through `internal/tuiclient`, never `jobs.db`, and `internal/content` imports neither `internal/store` nor `internal/httpapi`. A Studio fact a channel tool needs is a Studio route and tool first.
- **Render only through Studio.** Every video and essay render goes through the GPU render tools of `zv-mcp` and the Studio render routes. No channel tool drives `cliphub-render` on its own.
- **A separate store.** The channel state stays its own SQLite file (`statePath` in the channel config, default `%LOCALAPPDATA%\cliphub-content\state.db`) with its own migrations in `internal/content`, ported with the same schema so the owner's file keeps working. Studio's store never opens it and the channel tools never open Studio's. It is not an `internal/store` repository, so it has no memory twin; its tests use a temporary file.
- **Tokens stay outside.** YouTube and Google tokens stay where they are today, outside the repo and the installer. The channel config may name the folder that holds them and never holds a token; a test refuses a token-like key, as `content/src/config.test.ts` does today.
- **Long jobs** follow the `zv-mcp` pattern: a start tool that returns an id and a `wait_for_*` that polls it. This removes the wait for MCP Tasks in the TypeScript SDK that held back the render and upload tools.

## Considered options

- **Leave out the channel tools from the release build with a Go build tag.** Rejected: the owner drives Studio through the stable `zv-mcp.exe` that Studio installs and updates, so a tag would force a second binary built by hand; `go vet`, the tests and `TestNoDeadCode` would need a run per tag; and the code is public anyway, so leaving it out of the binary hides nothing. The runtime gate keeps the tools away from users, which is what matters.
- **Gate on a file found on disk, such as `content/config.local.json` in the working folder.** Rejected: the installed `zv-mcp` has no repo folder, and an implicit file in some user's folder would switch the tools on by accident. The gate is one explicit variable.
- **`zv-mcp` serves the channel tools by delegating to the TypeScript core as a child process.** Rejected: it keeps two runtimes, two schemas for every tool across a pipe, approval and errors crossing a process boundary, and a proxy that ships in the installer, so "one server" would hold only for the tool list. The TypeScript core is about 1,500 lines today, and porting it before thumbnails, metrics, the planner and upload land costs less than carrying the bridge.
- **Rewrite `zv-mcp` in TypeScript so the channel code stays where it is.** Rejected: `zv-mcp` is the shipped server with its approval, code mode, Windows job object and render tools in Go.
- **Keep two servers (ADR 0003).** Rejected by the owner.

## Planned PRs, in order

1. This ADR and the docs; the root `.mcp.json` is removed. Until PR 3 lands, the TypeScript state tools run only through `pnpm --dir content run cli`.
2. The render has one door: the `content/` render tools of PR 659 (`start_essay_render` and the others, with `content/src/render/`) are dropped, in PR 659 before it merges or in the PR right after it, and `zv-mcp` gains any essay start, status or cancel tool they had that its four GPU tools lack.
3. The port: `internal/content`, the owner gate, the gate test and the `channel_` state tools in `zv-mcp`; the `content/` package, its workspace entry and its `check` steps are deleted in the same PR, so two stores of the same file never coexist.
4. On from there, ADR 0003's thumbnails, metrics, titles, planner and upload PRs land as `channel_` tools in `zv-mcp`, and its last PR shrinks the scheduled-task prompts.

## Consequences

- The installer's `zv-mcp.exe` carries the channel code, switched off. A user who sets `CLIPHUB_CONTENT_CONFIG` gets tools that act only on their own config, state and tokens.
- The owner's `cliphub-studio` registration needs the variable; a session without it sees Studio only.
- Channel code joins the Go gates (comments, file length, dead code), and the `content` pnpm package leaves the workspace in PR 3.
