# Remove The Embedded Codex Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove ClipHub Studio's embedded assistant, the typed operation gateway that served it, and the headless Codex worker that proposed publish text, leaving Studio a pure GUI.

**Architecture:** Three independent removals plus a documentation pass. The Go side goes first because the desktop surface-coverage test asserts the operation catalog against `internal/httpapi/routes.go`; with the routes gone first, that test's deletion is obviously correct rather than a silent weakening. Unlike the previous removal in this repository, each task leaves the module building, so each task commits on its own.

**Tech Stack:** Go 1.26.5, Next.js 15 / React 19 (`web/`), Electron + TypeScript (`desktop/`), pnpm 11.9.0, Node 24.

Source spec: `docs/superpowers/specs/2026-07-26-remove-embedded-codex-agent-design.md`. Read its "The Gateway Decision, And How To Undo It" section before starting — it records why the gateway is being deleted rather than kept, and Task 5 has to append the removal SHA to it.

## Global Constraints

- Work directly on `main`. Never create a branch or worktree, and never open a pull request. The pre-commit hook rejects commits made outside `main`.
- Commit with `committer "message" path1 [path2 ...]` (on PATH). Never `git add` plus `git commit`, and never `.` as a path.
- **Never pass a multi-line commit message through a PowerShell here-string via the Bash tool.** A previous agent did and corrupted the message with stray `@` characters. Use a Bash heredoc.
- Delete files and directories with `trash <path> [...]` (on PATH). Never `rm`, `Remove-Item`, or `del`.
- Never bypass `.githooks/pre-commit` with `--no-verify` or `core.hooksPath`. It is the only gate this repository has.
- Never add "Generated with Claude Code" or `Co-Authored-By` lines to commits.
- Bare `bash` is a broken WSL shim on this machine. Invoke shell gates as `& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh ...`.
- Run package commands as `pnpm --dir web|desktop|landing <script>`. There is no root workspace.
- Do not add dependencies and do not run `go mod tidy`.
- Do not edit anything under `data/`, `web/.next/`, `desktop/build-resources/`, or `desktop/dist-installer/`.
- **Do not touch the development-side Codex harness:** `scripts/codex-*.sh`, `.codex/prompts/`, `.codex/skills/`, `.codex/config.toml`. That is the tool used to work on this repository, not something shipped with the app. `.codex/GUIDE.md` and `.codex/session-context.md` may be edited in Task 4 only where they describe the shipped in-app agent.
- **Do not touch the publish-pack caption artifact:** `renderplan.RenderVariantArtifactCaption`, `artifacts.RenderVariantCaptionKey`, `GET /api/jobs/{id}/renders/{variant}/captions/{name}`, `renderplan/publish_board.go` and `renderplan/upload_targets.go` all stay. The removed agent produced candidates; the artifact is written by the publish flow.
- **Do not touch `ZV_MUTATION_TOKEN`.** It is not the assistant's — the web proxy uses it to reach the orchestrator.
- Write boring, idiomatic Go and follow the existing TypeScript patterns. Add useful context when returning errors.
- Markdown is written one full sentence per line. Preserve each file's existing line endings; some files here are CRLF.

---

## File Structure

Deleted whole:

- `internal/workers/agent_worker.go` and `agent_worker_test.go`
- `internal/renderplan/agent.go` and `agent_test.go`
- `desktop/src/assistant/` (app-server client and transport, controller, history, native approval, and their tests)
- `desktop/src/assistant-ipc.ts`, `desktop/src/assistant-command.ts` and their tests
- `desktop/src/mcp/` (operations, orchestrator client, discovery, operation-gateway test, surface coverage)
- `desktop/src/studio-operations/`
- `web/components/assistant/` (panel, provider, rail)
- `web/lib/assistant.ts` and its tests
- `web/components/shell/assistant-rail-state.ts`

Trimmed but kept: `internal/tasks/tasks.go`, `internal/artifacts/keys.go`, `internal/httpapi/{routes,handlers,workbench_htmx}.go`, `cmd/zv-orchestrator/{config,main}.go`, `desktop/src/{main,preload}.ts`, `web/app/(app)/layout.tsx`, `web/components/shell/{command-strip,shell-cookies}.tsx`, `web/app/(app)/settings/page.tsx`, and the documentation set.

---

## Task 1: The Headless Publish-Text Agent

Remove the Codex worker that proposed title, caption and hashtag candidates, and the orchestrator configuration that enabled it.

**Files:**
- Delete: `internal/workers/agent_worker.go`, `internal/workers/agent_worker_test.go`, `internal/renderplan/agent.go`, `internal/renderplan/agent_test.go`
- Modify: `internal/tasks/tasks.go`, `tasks_test.go`, `internal/artifacts/keys.go`, `keys_test.go`, `internal/httpapi/routes.go`, `handlers.go`, `handlers_test.go`, `workbench_htmx.go`, `cmd/zv-orchestrator/config.go`, `config_test.go`, `main.go`, `inline_queue_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: no `tasks.TypeCodexAgent`, `tasks.CodexAgentPayload`, `tasks.NewCodexAgentTask`, `renderplan.AgentKindCaptionCandidates`, `renderplan.AgentContext`, `renderplan.AgentResult`, `renderplan.AgentArtifacts`, `renderplan.NewAgentArtifacts`, `renderplan.NewAgentContext`, `renderplan.NewAgentResult`, `renderplan.AgentSchemaVersion`, `artifacts.RenderVariantAgentResultKey`, `workers.AgentWorker`, or `workers.AgentWorkerConfig`.
  Task 2 relies on the three HTTP routes below being gone.

- [ ] **Step 1: Delete the two packages' agent files**

```powershell
trash internal/workers/agent_worker.go internal/workers/agent_worker_test.go internal/renderplan/agent.go internal/renderplan/agent_test.go
```

- [ ] **Step 2: Remove the routes and their handlers**

In `internal/httpapi/routes.go`, delete these three registrations:

```go
r.Post("/ui/jobs/{id}/agent/captions", h.WorkbenchStartCaptionAgent)
r.Post("/api/jobs/{id}/renders/{variant}/agent/captions", h.StartCaptionAgent)
r.Get("/api/jobs/{id}/renders/{variant}/agent/captions", h.GetCaptionAgent)
```

Keep `r.Get("/api/jobs/{id}/renders/{variant}/captions/{name}", h.GetRenderCaption)` — that serves the publish-pack caption artifact and is not the agent.

In `internal/httpapi/handlers.go`, delete `Handlers.StartCaptionAgent` and `Handlers.GetCaptionAgent`, and in `workbench_htmx.go` delete `WorkbenchStartCaptionAgent` along with any workbench markup that offered candidate generation.

Keep `Handlers.GetRenderCaption` and the `renderplan.RenderVariantArtifactCaption` branch at `handlers.go:1489` and `:1572`.

- [ ] **Step 3: Remove the task type and artifact key**

In `internal/tasks/tasks.go`, delete `TypeCodexAgent`, `CodexAgentPayload`, `NewCodexAgentTask`, and any header or accessor that only they used.

In `internal/artifacts/keys.go`, delete `RenderVariantAgentResultKey` and any unexported helper left with no caller. Keep `RenderVariantCaptionKey`.

- [ ] **Step 4: Remove the orchestrator configuration**

In `cmd/zv-orchestrator/config.go`, delete `CodexPath`, `CodexModel`, `AgentTimeout`, the `ZV_CODEX_PATH`, `ZV_CODEX_MODEL` and `ZV_AGENT_TIMEOUT` reads, and the `agentWorkerEnabled` predicate.

In `cmd/zv-orchestrator/main.go`, delete the `workers.NewAgentWorker` construction, the `taskHandlers[tasks.TypeCodexAgent]` registration and its `log.Printf("worker: codex agent enabled")`.

Leave `MutationToken` and its validation completely alone. Leave `DiscoverySecret` alone for now — Task 2 owns it, because it belongs to the gateway rather than to this worker.

- [ ] **Step 5: Trim the tests**

Delete every test case in `tasks_test.go`, `keys_test.go`, `handlers_test.go`, `config_test.go` and `inline_queue_test.go` that exercises a removed task type, route, key or configuration value. Trim fixtures rather than hollowing assertions: a test that still runs must still assert something real.

- [ ] **Step 6: Run the Go gate**

Run: `& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --no-format --build`

Expected: PASS. The module builds, `go vet` is clean, `zv check` passes, and every Go test passes.

- [ ] **Step 7: Commit**

```powershell
committer "Remove the Codex publish-text agent" internal cmd
```

---

## Task 2: The Desktop Assistant And Operation Gateway

Remove the embedded assistant, the typed operation gateway, and every IPC channel that connected them to the renderer.

**Files:**
- Delete: `desktop/src/assistant/` (whole directory), `desktop/src/assistant-ipc.ts`, `desktop/src/assistant-command.ts`, `desktop/src/mcp/` (whole directory), `desktop/src/studio-operations/` (whole directory), and every matching `*.test.ts`
- Modify: `desktop/src/main.ts`, `desktop/src/preload.ts`, `desktop/src/process-session.test.ts` if it references a removed module
- Modify: `cmd/zv-orchestrator/config.go`, `config_test.go`, `main.go`

**Interfaces:**
- Consumes: the routes removed in Task 1 — the surface-coverage test asserted against them, which is why it is deleted here rather than updated.
- Produces: a preload surface with no `CLIPHUBAssistant` bridge, and a main process with no assistant controller, no operation gateway and no orchestrator client.
  Task 3 relies on `window.CLIPHUBAssistant` no longer existing.

Read `desktop/GUIDE.md` before starting.

- [ ] **Step 1: Delete the modules**

```powershell
trash desktop/src/assistant desktop/src/assistant-ipc.ts desktop/src/assistant-command.ts desktop/src/mcp desktop/src/studio-operations
```

Then check for stragglers: `desktop/src/assistant-ipc.test.ts` and `desktop/src/assistant-command.test.ts` may sit outside those directories. Delete any that exist.

- [ ] **Step 2: Unwire the main process**

In `desktop/src/main.ts`, delete the imports at lines 46-61 (`ASSISTANT_CHANNEL`, `ASSISTANT_EVENT_CHANNEL`, `AssistantEvent`, `AssistantIPCResponse`, `assistantCommandFailure`, `dispatchAssistantRequest`, `AssistantController`, `AssistantHistoryStore`, the native-approval imports, `OperationGateway`, `OrchestratorClient`).

Delete the module-level state at lines 151-158: `assistantController`, `assistantNativeApproval`, `assistantHistoryFile`, `assistantWorkspace`, and the comment about Codex getting a dedicated empty cwd.

Delete `sendAssistantEvent` and `getAssistantController` entirely, the assistant IPC handler registration, and the `showAssistantNativeApproval` dialog.

The assistant workspace directory under `app.getPath('userData')` is created by `getAssistantController`. With that gone nothing creates it; do not add cleanup code for existing installs, and say so in your report.

- [ ] **Step 3: Narrow the preload surface**

In `desktop/src/preload.ts`, delete the `ASSISTANT_CHANNEL` and `ASSISTANT_EVENT_CHANNEL` constants and the whole `contextBridge.exposeInMainWorld('CLIPHUBAssistant', { ... })` block with its `status`, `wake`, `send`, `cancel`, `approve` and `reject` methods.

Leave every other exposed bridge intact.

- [ ] **Step 4: Remove the discovery credential**

In `cmd/zv-orchestrator/config.go`, delete `DiscoverySecret`, `discoverySecretEnvironmentVariable`, `validDiscoverySecret` and `clearDiscoverySecretEnvironment`, plus the call site in `main.go`. That credential existed for the gateway's ports-file discovery handshake, which is now gone.

Before deleting `clearDiscoverySecretEnvironment`, confirm no other credential relies on it. `clearEnvironmentVariable` is shared and must stay.

Leave `MutationToken` and `validSessionCapability` untouched.

- [ ] **Step 5: Run the gates**

```powershell
& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --no-format --build
pnpm --dir desktop run lint
pnpm --dir desktop run typecheck
pnpm --dir desktop run test:unit
pnpm --dir desktop run build
```

Expected: all pass.

- [ ] **Step 6: Commit**

```powershell
committer "Remove the embedded assistant and its operation gateway" desktop cmd/zv-orchestrator
```

---

## Task 3: The Web Assistant Rail

Remove the assistant from the app shell and give its width back to the content column.

**Files:**
- Delete: `web/components/assistant/` (whole directory), `web/lib/assistant.ts` and its tests, `web/components/shell/assistant-rail-state.ts` and its tests
- Modify: `web/app/(app)/layout.tsx`, `web/components/shell/command-strip.tsx`, `web/components/shell/shell-cookies.ts`, `web/app/(app)/settings/page.tsx`, and any render view offering candidate generation

**Interfaces:**
- Consumes: the preload surface from Task 2 — nothing may still call `window.CLIPHUBAssistant`.
- Produces: a two-column shell.

Read `web/CLAUDE.md` before starting, and `web/design.md` before touching layout.

- [ ] **Step 1: Delete the assistant modules**

```powershell
trash web/components/assistant web/lib/assistant.ts web/components/shell/assistant-rail-state.ts
```

Delete their test files too; find them with a glob rather than assuming the names.

- [ ] **Step 2: Reduce the shell to two columns**

In `web/app/(app)/layout.tsx`, delete the `AssistantRail` and `assistantRailStateFromCookie` imports, the `ASSISTANT_RAIL_COOKIE_NAME` import, the `assistantState` cookie read, and the `<AssistantRail />` element.

The `<main>` element carries a long comment explaining that `@container/content` exists because the real box was "the viewport minus a 240px sidebar minus the assistant". Rewrite that comment to match the new geometry rather than leaving it describing a column that no longer exists. Keep `@container/content` itself and keep `mr-auto` — the optical-spine reasoning still holds with two columns.

Re-check `max-w-[1440px]` against `web/design.md`: with the rail gone the content box is wider at every viewport, and the design doc names 1440px as a validation target. If the doc implies a different bound now, say so in your report rather than changing it silently.

In `web/components/shell/shell-cookies.ts`, delete `ASSISTANT_RAIL_COOKIE_NAME`. Do not write migration code to clear the stale cookie from existing installs; an unread cookie is harmless.

- [ ] **Step 3: Remove the command-strip toggle**

In `web/components/shell/command-strip.tsx`, delete the `toggleAssistant` import and the control that calls it, and update the file's header comment, which describes the strip as spanning "the content and assistant" columns.

- [ ] **Step 4: Remove the remaining agent-facing copy and controls**

In `web/app/(app)/settings/page.tsx`, the page description reads "Consulta la versión instalada de ClipHub Studio. El agente integrado usa tu sesión personal de Codex." Drop the second sentence.

Search `web/` for any control that starts caption, title or hashtag candidate generation, or that renders their results, and remove it. It called routes deleted in Task 1, so it is dead either way.

If you find an assistant reference in a `web/` file this plan does not list, remove it and name the file in your report.

- [ ] **Step 5: Run the web gates**

```powershell
pnpm --dir web run lint
pnpm --dir web run typecheck
pnpm --dir web run test:unit
pnpm --dir web run build
```

Expected: all four pass.

- [ ] **Step 6: Commit**

```powershell
committer "Remove the assistant rail from the app shell" web
```

---

## Task 4: Documentation

Stop documenting an assistant the product no longer ships.

**Files:**
- Modify: `CLAUDE.md`, `PRODUCT.md`, `desktop/GUIDE.md`, `.codex/GUIDE.md`, `.codex/session-context.md`

**Interfaces:**
- Consumes: the final shape from Tasks 1 through 3.
- Produces: documentation that matches the shipped app.

- [ ] **Step 1: Update `CLAUDE.md`**

Two known claims stop being true, at roughly lines 50 and 141: that ClipHub Agent is the only assistant surface shipped in Studio, and that Studio adds a separate approval of the exact costly or destructive operation preview. Delete both, along with the instruction to use the integrated typed operation gateway.

The sentence about not resurrecting the retired external MCP server can go with them — with no gateway at all, the distinction it drew no longer means anything. Say in your report if you disagree and left it.

Leave every other approval rule intact. The creative brief gate and the thumbnail gate are human approvals and are unaffected.

- [ ] **Step 2: Update the remaining documentation**

`desktop/GUIDE.md` lines 62 and 83 are the known hits; sweep the file for others, including any section on the embedded agent, the assistant workspace, or the operation gateway.

`PRODUCT.md`, `.codex/GUIDE.md` and `.codex/session-context.md` get the same pass. In the two `.codex` files, edit only what describes the **shipped in-app** agent. The development-side harness those files also document — the `codex-*.sh` wrappers, the prompts, the skills — stays exactly as it is.

- [ ] **Step 3: Verify the docs against the binary**

```powershell
.\scripts\build.ps1
.\bin\zv.exe capabilities --format json
```

Expected: `zv check` passes inside the build, and no capability or workflow advertises an agent surface.

- [ ] **Step 4: Commit**

```powershell
committer "Update documentation for a Studio with no embedded agent" CLAUDE.md PRODUCT.md desktop/GUIDE.md .codex
```

---

## Task 5: Verification And The Undo Record

Prove the removal, and leave a future maintainer the one pointer that makes it reversible.

**Files:** `docs/superpowers/specs/2026-07-26-remove-embedded-codex-agent-design.md` only.

- [ ] **Step 1: Confirm nothing survives**

```powershell
git grep -n -E -i "CLIPHUBAssistant|AssistantController|OperationGateway|studio-operations|src/mcp|TypeCodexAgent|AgentKindCaptionCandidates|ZV_CODEX_PATH|ZV_DISCOVERY_SECRET" -- ":!data" ":!docs/superpowers" ":!web/.next" ":!desktop/build-resources" ":!desktop/dist-installer" ":!.codex" ":!scripts"
```

Expected: no hits. A hit under `.codex/` or `scripts/` would be the development harness, which is why both are excluded — but read any such hit before dismissing it.

- [ ] **Step 2: Run every gate**

```powershell
& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --no-format --build --security
pnpm --dir web run lint
pnpm --dir web run typecheck
pnpm --dir web run test:unit
pnpm --dir web run build
pnpm --dir desktop run lint
pnpm --dir desktop run typecheck
pnpm --dir desktop run test:unit
pnpm --dir desktop run build
pnpm --dir landing run build
```

Expected: all pass.

- [ ] **Step 3: Look at Studio**

Start the local stack and open the app. Confirm by eye:

- The shell has two columns, with no empty gutter, no stray border and no gap where the rail was.
- The command strip has no dead toggle where the assistant control used to be.
- The settings page reads correctly without the Codex sentence.
- No render view offers to generate title, caption or hashtag candidates.

Fix any visual gap before finishing. This project holds a high bar on layout polish; a build that passes but looks cut open is not done.

- [ ] **Step 4: Record the undo pointer**

The spec's "The Gateway Decision, And How To Undo It" section ends by requiring the removal SHA, so a future rebuild starts from `git show` rather than from scratch.

Find the commit from Task 2 with `git log --oneline`, and append to that section the exact command a future maintainer would run, with the real SHA substituted:

```text
The gateway was removed in <sha>. To recover it: `git show <sha> -- desktop/src/mcp desktop/src/studio-operations`.
```

- [ ] **Step 5: Commit**

```powershell
committer "Record where the operation gateway can be recovered from" docs/superpowers/specs/2026-07-26-remove-embedded-codex-agent-design.md
```

---

## Self-Review Notes

Spec coverage was checked section by section. The three removal blocks map to Tasks 1, 2 and 3; the approval-surface change falls out of Task 2, since the gate lived in `studio-operations`; the configuration changes are split so each lands with the code that used it — the Codex worker's variables in Task 1, the gateway's discovery secret in Task 2. Documentation is Task 4 and verification is Task 5, which also discharges the spec's requirement to record the removal SHA.

The spec's protected surfaces — the development harness, the publish-pack caption artifact, and `ZV_MUTATION_TOKEN` — appear in the Global Constraints so every task inherits them, and each is named again in the task where it is most at risk of being cut by mistake.
