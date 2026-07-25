# Remove The Embedded Codex Agent

Date: 2026-07-26

## Problem

FragForge Studio ships an embedded assistant — FragForge Agent — that runs against the user's personal Codex session, plus a typed MCP operation gateway that lets it act on the app, plus a headless Codex worker that proposes title, caption and hashtag candidates for a demo render.

Luis wants all three gone. Studio becomes a pure GUI: the pipeline is driven through the interface and the `zv` CLI, and publish text is written by hand.

## Scope

In scope, all three removed together:

- **The embedded assistant.** `desktop/src/assistant/` (app-server client and transport, controller, history, native approval), `desktop/src/assistant-ipc.ts`, their wiring in `main.ts` and `preload.ts`, and on the web side `components/assistant/` and `lib/assistant.ts`, including the right-hand rail and its collapse-state cookie.
- **The typed MCP operation gateway.** `desktop/src/mcp/` in full: the operation catalog, the orchestrator client, discovery, and the surface-coverage test. Also `desktop/src/studio-operations/`, which holds the preview-then-privileged-revalidate boundary and whose only importers are the assistant controller and the gateway's own tests.
- **The headless publish-text agent.** `internal/workers/agent_worker.go`, `internal/renderplan/agent.go`, `tasks.TypeCodexAgent` and its payload, `artifacts.RenderVariantAgentResultKey`, the routes that start and read it, and the orchestrator configuration that enables it.

Out of scope, explicitly untouched:

- **The development-side Codex harness.** `scripts/codex-*.sh` and `.codex/` (prompts, skills, `GUIDE.md`, `config.toml`). That is the tool Luis and coding agents use on this repository; it is not shipped with the app.
- **The publish-pack caption artifact.** `RenderVariantArtifactCaption`, `artifacts.RenderVariantCaptionKey`, `GET /api/jobs/{id}/renders/{variant}/captions/{name}`, and the publish board and upload targets that reference them all stay. The removed agent produced *candidates*; the artifact itself is written by the publish flow and is unrelated.
- **`ZV_MUTATION_TOKEN`.** It is not the assistant's. The web proxy uses it to reach the orchestrator, so it stays exactly as it is.

## What Studio Becomes

The right-hand assistant rail disappears from the app shell, and the content column takes back its width. The command strip loses its assistant toggle. The settings page loses the sentence about the integrated agent using a personal Codex session, and the render view loses any control that starts candidate generation.

No placeholder replaces the rail. A rail that only says the assistant is unavailable is worse than no rail: it occupies screen width and advertises a feature that does not exist.

## The Approval Surface

The gateway carried a Studio-specific approval: the user confirmed the exact preview of a costly or destructive operation before an agent could run it. With no agent-initiated operations, there is nothing left for that gate to guard, so it goes with the gateway.

The approvals that remain are the human ones that were never about agents: the creative brief before any non-dry-run capture or render, and the thumbnail selection before a pack is called upload-ready. Neither is weakened by this change.

## Configuration

Removed from `cmd/zv-orchestrator`: `ZV_CODEX_PATH`, `ZV_CODEX_MODEL`, `ZV_AGENT_TIMEOUT`, the `agentWorkerEnabled` predicate and the `tasks.TypeCodexAgent` handler registration, and `ZV_DISCOVERY_SECRET` with `validDiscoverySecret` and `clearDiscoverySecretEnvironment`.

`ZV_DISCOVERY_SECRET` goes because it is the desktop-only credential the MCP discovery path used. `ZV_MUTATION_TOKEN` stays because the web proxy depends on it.

Removed from the desktop shell: the Codex OAuth connection flow and any stored session it kept.

## The Gateway Decision, And How To Undo It

Deleting the gateway was contested, so it was evaluated independently by a reviewer with no prior context. Its verdict was to delete, and its reasoning is worth recording because it also names the one fact that would reverse it.

The catalog is genuinely not Codex-coupled: `OperationDefinition` carries no protocol type, and `orchestrator-client.ts` is a plain loopback HTTP client. But it is not the app's API client either — `web/lib/api/` is, behind the Next proxy routes — and it is not the app's programmatic surface, which is the `zv` CLI. It is a second, hand-maintained description of the same routes, and every importer of it today is reachable only from the assistant.

The expensive knowledge inside it is assistant-shaped rather than API-shaped: the Spanish and English alias table, the search scoring heuristic, the destructive-term penalty, dynamic input discovery, and the agent-defense invariants. A different assistant would re-derive those anyway. The mechanical part — risk class and preview — reduces to HTTP method plus path, which `surface-coverage.test.ts` already proves.

The strongest argument the other way, which is real: the gateway predates the assistant. It was added on 2026-07-13 for the retired external MCP server and was repurposed without a rewrite when that server went away on 2026-07-20. It has already survived one change of consumer.

The fact that would flip the decision is a *named* next consumer — a decided-on replacement assistant, or a plan to expose Studio operations to an external caller. Not a vague possibility. Absent one, `surface-coverage.test.ts` turns from a safety net into a tax: it forces every future HTTP route change to drag the catalog along, at roughly a hundred lines per feature, to keep a catalog honest that nothing reads.

**Therefore the removal commit must be recorded here once it lands, so a future rebuild starts from `git show <sha> -- desktop/src/mcp desktop/src/studio-operations` rather than from scratch.** Add the SHA to this section as the last step of the work.

## Data And Compatibility

There is no persisted contract to migrate. Existing `codex-agent-result.json` files and agent artifacts under `data/` stay on disk as history; nothing reads them and nothing breaks because they exist.

This is the significant difference from the stream killfeed and captions removal, where a persisted edit plan had to keep loading. Here no stored document carries a field that disappears.

## Documentation

`CLAUDE.md` states that FragForge Agent is the only assistant surface shipped in Studio, and that Studio adds a separate approval of the exact costly or destructive operation preview. Both stop being true and must go, along with any instruction to reach for the integrated typed operation gateway.

`CLAUDE.md` lines 50 and 141, and `desktop/GUIDE.md` lines 62 and 83, promise a gateway that will no longer exist; those are the known hits, and a sweep must find the rest.

`PRODUCT.md` and `.codex/GUIDE.md` need the same pass. The landing page does not currently advertise the agent, so it needs no change unless the sweep finds one.

## Testing

- Delete the tests dedicated to every removed module.
- Trim any test that asserts the presence of an assistant route, operation, or IPC channel, rather than weakening the assertion. `surface-coverage.test.ts` is deleted outright, since it exists to hold the MCP surface against the HTTP surface.
- Add nothing new. This removal introduces no behavior that needs pinning; the compatibility risk that justified new tests last time does not exist here.

## Verification

1. `& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --no-format --build --security`
2. `pnpm --dir web run lint`, `typecheck`, `test:unit`, `build`
3. `pnpm --dir desktop run lint`, `typecheck`, `test:unit`, `build`
4. `pnpm --dir landing run build`
5. A real look at Studio: the shell must have no empty column and no gap where the rail was, the command strip must not show a dead toggle, and the render view must not offer candidate generation.

## Risks

The web app shell layout is built around three columns. Removing one is a layout change, not just a deletion, and it is the part most likely to look wrong rather than fail a test. It gets the visual check, not just a passing build.

`desktop/src/main.ts` wires the assistant, the MCP gateway and the IPC channels together. Cutting them out touches the same file three times, so the work is ordered to land those cuts in one pass rather than three.
