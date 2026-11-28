# ClipHub

This monorepo is the single index of ClipHub. The UI is in Spanish. Rules live in `.claude/rules/`
and load on their own when a session touches the paths in their frontmatter; skills (recipes and
checklists) live in `.claude/skills/`. `CLAUDE.md` imports this file. Keep this file an index:
details belong in a rule.

## Products and verticals

ClipHub has two verticals: **clipping and YouTube content** (Shorts, Full Demo POVs, stream clips,
thumbnails) and **CS2 analysis with AI over MCP**. Three products serve them:

- **ClipHub Studio**, the Windows desktop app we sell, covers both verticals. It runs locally:
  demos, capture, renders and the user's library stay on their PC. It only reads shared data (pro
  reference, rankings, scraped pro matches) from the community API on the VPS, and syncs a library
  only when the user links a ClipMind workspace. Assistants drive it through `zv-mcp`.
- **ClipMind (ClipHub Strats)**, the web edition of the analysis at `cliphubstrats.rechedev.cloud`:
  accounts, team workspaces and a library that syncs with Studio. PC-bound features stay in Studio.
- **The content product**, the owner's own YouTube pipeline: owner-only, never registered for
  installer users. ClipHub has one MCP server, `cliphub-studio` (`cmd/zv-mcp`); the channel tools join it
  as owner-only `channel_` tools ([ADR 0004](docs/adr/0004-one-mcp-server.md)). Until that port
  lands, the channel core is the TypeScript package in `content/`, run through its CLI
  ([docs/content/README.md](docs/content/README.md)); the product's design is
  [ADR 0003](docs/adr/0003-content-vertical-and-monorepo.md).

Studio and ClipMind ship together from the same commit: every Studio release also deploys
ClipMind, or their sync contracts drift. Design and verify each change in every edition that shows
it. [ClipMind and the VPS](.claude/rules/clipmind.md).

## Layout

| Path | What it is |
|---|---|
| `web/` | Next.js UI of both editions. `web/app/api/*` proxies to the orchestrator (Studio) or to `cmd/clipmind-server` (ClipMind). `web/lib/edition-routes.ts` decides which pages and APIs each edition has |
| `desktop/` | Electron shell of Studio and the installer build |
| `cmd/`, `internal/` | Go: the orchestrator, workers, parser, recorder, editor, store and the CLIs |
| `mcp-apps/` | React views that `zv-mcp` serves as MCP Apps (built into `cmd/zv-mcp/apps/`) |
| `content/` | The content product's TypeScript core, its CLI and its SQLite state, until ADR 0004 ports it into `zv-mcp` ([docs/content/README.md](docs/content/README.md)) |
| `landing/` | The public site `cliphub.rechedev.cloud`, a static export |
| `services/community` | Community API on the VPS: pro reference, rankings, pro lineups, pro matches (`pro_match_maps`) |
| `services/telemetry` | Telemetry collector on the VPS. [Remote debugging](docs/remote-debug-tracing.md) |
| `tools/hltv-harvester` | Operator Electron tool on the owner's PC that downloads HLTV demo archives |
| `tools/chperf`, `tools/oxlint-no-comments` | A C performance probe, and the oxlint plugin behind `cliphub/no-comments` |
| `plugins/teamspeak` | TeamSpeak plugin that records team voice for Studio (`scripts/build-teamspeak-plugin.ps1`) |
| `data/`, `deploy/`, `scripts/` | Bundled game data and music catalog, VPS copies of the deploy files, build and operator scripts |

## How the pieces talk

- `desktop/src/main.ts` picks stable loopback ports (`desktop/src/stable-ports.ts`), spawns
  `zv-orchestrator.exe` and the web standalone server (`ORCHESTRATOR_URL`, `PORT`), then loads the
  UI. `web/proxy.ts` and `web/lib/api/local-request-guard.ts` reject cross-origin and DNS-rebound
  requests. [Web inside Electron](.claude/rules/web-electron.md).
- `cmd/zv-orchestrator` serves the local HTTP API (`internal/httpapi`) and the job queue. Workers in
  `internal/workers` parse demos in process (`internal/parser`) and run `zv-recorder` (CS2 capture
  through HLAE) and `zv-editor` (FFmpeg render) as subprocesses. State lives in SQLite through
  `internal/store`. The installer ships these three binaries plus `zv-mcp.exe`.
- `cmd/zv-mcp` is the stdio MCP server for assistants, an adapter over the orchestrator API
  ([docs/studio-mcp.md](docs/studio-mcp.md)). `cmd/zv` is the CLI over the same pipeline; the other
  `cmd/` tools are for development.
- ClipMind: `web/` built with `NEXT_PUBLIC_CLIPHUB_EDITION=web` talks to `cmd/clipmind-server`
  (`internal/clipmind`), which serves the same `httpapi.AnalysisRoutes()` with one SQLite library
  per workspace. Studio syncs a linked workspace through `/api/sync`.
- Pro matches: `tools/hltv-harvester` downloads HLTV archives into `%USERPROFILE%\cliphub-hltv`,
  after it asks the community API which matches the catalog already has;
  `cliphub-community prepare-hltv` scans them into analyses on the same PC; the community API on the
  VPS serves the pro match catalog that Studio reads. The harvester's `ingest` commands chain these
  steps: [tools/hltv-harvester/README.md](tools/hltv-harvester/README.md).
- Content: there is no second MCP server. The channel tools are owner-only tools of `zv-mcp`,
  registered only when it starts with `CLIPHUB_CONTENT_CONFIG`; their SQLite state is a file of its
  own outside the repo, never `jobs.db`. Until the port, `content/` runs through its CLI and reaches
  Studio only by spawning `zv-mcp`. [Content product](.claude/rules/content.md).
- On the VPS (SSH alias `openclaw`), never in the installer: ClipMind, the community API, telemetry
  and the landing.

## Build and check

- Toolchain: the Go version of `go.mod`, the Node major of `.node-version` and the pnpm of
  `packageManager` in the root `package.json`. pnpm only warns ("Unsupported engine") when `node`
  is older and then runs the scripts on it, so check `node -v` before you trust a local result.
- Go: `go vet ./...`, `go test ./...`, and `go run ./cmd/zv check` (validates the workflow
  catalog, usage text and command build targets). One test:
  `go test ./internal/<pkg> -run TestName -count=1`.
- Fast local Go loop: `go test -short ./...` skips the slow FFmpeg renders and `TestNoDeadCode`.
  Pre-PR runs use the full suite. `internal/editor` alone nears the 10 min default on Windows, so
  pass `-timeout 30m` to a full run.
- One root pnpm workspace (`pnpm-workspace.yaml`: `web`, `desktop`, `landing`, `mcp-apps`,
  `tools/hltv-harvester`, `content`) with one `pnpm-lock.yaml` and a hoisted `node_modules`: install once with
  `pnpm install`, never inside a package. Shared dependency versions live in the `catalog:` of
  `pnpm-workspace.yaml`; overrides, `allowBuilds` and patches live there too.
- `pnpm run check` is the local gate: every package's `typecheck`, `lint` and `test:unit`, then
  `go run ./cmd/zv check`. It does not run the Go tests or the builds. The release workflow runs
  its own list of checks, which leaves out `content` and `tools/hltv-harvester`: this gate is the
  only one those two have. The `Makefile` targets (`make check`, `make test`) are aliases for the
  commands above; a Windows PC without `make` runs the commands.
- One package: `pnpm --dir <pkg> run typecheck`, `lint`, `test:unit`; `run build` for `web` and
  `landing`. One test: `pnpm --dir <pkg> exec node --test <path>.test.ts`. `landing`'s `test:unit`
  runs `next build` first, because its tests read the prerendered page.
- `web`'s `build` flattens Next's monorepo standalone output (`web/scripts/flatten-standalone.mjs`)
  so `.next/standalone/server.js` stays where the installer and the ClipMind image expect it.
- A few Full Demo editor tests depend on the FFmpeg version; Studio ships FFmpeg 8.1.2.
  [Render and the editor](.claude/rules/render-editor.md).

## Golden rules

Each rule names what enforces it. "Review only" means nothing fails when an agent skips it: those are the next to turn into a check. Follow the link before you work around one.

| Rule | Enforced by |
|---|---|
| Source files stay at or under 1000 lines. Files already past it are frozen: they shrink, never grow, and no max is raised. [Split a file](.claude/skills/split-file/SKILL.md) | `TestGoFileLength` (`cmd/zv`), oxlint `max-lines` |
| Code has no comments and no `eslint-*` or `oxlint-*` directives; only `//go:build`, `//go:embed`, a generated-code header, `/// <reference` and a shebang are allowed. Files that had comments before the rule are frozen in a `comment-baseline.json` that only shrinks: never raise a count or add a file. A lint exception goes in the package's `.oxlintrc.json` `overrides`. After you delete comments, lower the baseline with `go test ./cmd/zv -run TestNoComments -shrink-comment-baseline` or `node ../tools/oxlint-no-comments/shrink-baseline.mjs` in the package | `TestNoComments` and `TestCommentBaselinesOnlyShrink` (`cmd/zv`); oxlint `cliphub/no-comments` (`tools/oxlint-no-comments`) in every pnpm package |
| No production function that only tests call; a helper that only `_windows.go` code calls lives in a `_windows.go` file | `TestNoDeadCode` (`cmd/zv`, pinned `deadcode` and `staticcheck U1000` for Windows and Linux). `-short` skips it, so a `-short` run proves nothing about dead code |
| UI code calls an API only through `apiFetch` (`web/lib/api/edition-fetch.ts`), and a shared component gates a Studio-only call on `currentEdition()`. [ClipMind and the VPS](.claude/rules/clipmind.md) | oxlint `no-restricted-globals` (bare `fetch`, `XMLHttpRequest`) in web; `apiFetch` refuses a route the edition lacks (`web/lib/api/edition-fetch.test.ts`). The gate itself is review only |
| A UI feature lands with its `cmd/zv-mcp` tool, `serverInstructions` and `docs/studio-mcp.md` row. [MCP parity](.claude/rules/mcp-parity.md) | `TestEveryRouteHasAnMCPTool` (`internal/httpapi`) for a new route; a feature on an existing route is review only |
| A store method lands in SQLite and memory with a shared contract test. [SQLite store](.claude/rules/store-sqlite.md) | Contract tests in `internal/store` |
| No new e2e suites, verification scripts, evidence folders or reports; the one test to extend is the `internal/pipeline` golden test. The `cmd/zv/*_e2e_test.go` CLI tests and the smoke and capture-lab scripts under `scripts/` are older than this rule: do not add siblings to them. [Render and the editor](.claude/rules/render-editor.md) | Review only |
| No CI on pushes or pull requests; the full Go suite runs locally before a PR and a release. [Release](.claude/rules/release.md) | Review only |
| `/post-work-review` (Skill tool) before every commit, `/code-review high --fix` before a PR. [Post-work review](.claude/skills/post-work-review/SKILL.md) | `.githooks/pre-commit` refuses an agent's commit whose staged tree the review did not record (`core.hooksPath` set by the `SessionStart` hook in `.claude/settings.json`) |
| A user-visible change is checked by hand in a throwaway Studio profile, never the real `%APPDATA%\cliphub-studio` or the session's `cliphub-studio` MCP; the profile is deleted after. [Verify in Studio](.claude/skills/verify-in-studio/SKILL.md) | Review only |
| The repository is public: `<user>` and `%APPDATA%`, never tokens, keys, support codes or `agent-session.json` contents | Review only |

## Index

### Rules

- [HLAE pin and capture](.claude/rules/capture-hlae.md): CS2 updates, the HLAE pin, capture pitfalls.
- [Demo parser](.claude/rules/demo-parser.md): our CS2 demo parser `demo-parser-rsn`, how it reads odd data, and how to change, tag and prove it.
- [Render and the editor](.claude/rules/render-editor.md): FFmpeg 8.1.2, the `internal/pipeline` golden test, render-attempt ownership, startup reconcile, the Full Demo artifact cache, plan hashes, HUD and overlays.
- [SQLite store](.claude/rules/store-sqlite.md): migrations and the paired SQLite and memory repositories.
- [MCP parity](.claude/rules/mcp-parity.md): what a UI change updates under `cmd/zv-mcp` and in `docs/studio-mcp.md`.
- [Web inside Electron](.claude/rules/web-electron.md): the desktop edition's Chromium, the local request guard, the capability cookie, `landing/`, the WebMCP preview, React attribute types.
- [ClipMind and the VPS](.claude/rules/clipmind.md): the web edition, Studio sync, the servers on the VPS, and how ClipMind and the landing are deployed.
- [Release](.claude/rules/release.md): the bar before a tag, and the one hosted workflow.
- [Content product](.claude/rules/content.md): the boundaries of `content/`, tools over prompts, its derived state, and how to run and check it.

### Skills

- [Feature with MCP](.claude/skills/feature-with-mcp/SKILL.md): a Studio screen, flow or control is added or changed.
- [Store method](.claude/skills/store-method/SKILL.md): `internal/store` gains or changes a method or a migration.
- [Split a file](.claude/skills/split-file/SKILL.md): a file nears 1000 lines, or `TestGoFileLength` or `max-lines` fails.
- [Update pipeline goldens](.claude/skills/update-golden/SKILL.md): `go test ./internal/pipeline` fails on a golden diff.
- [Verify in Studio](.claude/skills/verify-in-studio/SKILL.md): run Studio from source and check a change by hand.
- [Diff review](.claude/skills/diff-review/SKILL.md): review a branch, diff or PR. Read-only.
- [Post-work review](.claude/skills/post-work-review/SKILL.md): review and fix the staged change before each commit, then record it for `.githooks/pre-commit`.
- [Release check](.claude/skills/release-check/SKILL.md): full checks and a dry-run release workflow before a tag. It does not publish.
- [Shorts thumbnail](.claude/skills/shorts-thumbnail/SKILL.md): pixel-art Shorts thumbnail with Aseprite. The owner's content skills still call it; the thumbnails PR of ADR 0003 replaces it.

### Docs

- `docs/incidents.md`: domain rules and incident history (HLAE pin, Full Demo audio and overlays, telemetry, performance, release steps). The rules name its sections.
- [docs/studio-mcp.md](docs/studio-mcp.md): the Studio MCP server, its design and its tool table.
- [docs/remote-debug-tracing.md](docs/remote-debug-tracing.md): telemetry error events and the log channel, and how to query them.
- [docs/adr/](docs/adr/): architecture decisions. [ADR 0003](docs/adr/0003-content-vertical-and-monorepo.md) holds the monorepo and content-product plan; [ADR 0004](docs/adr/0004-one-mcp-server.md) makes `cliphub-studio` the one MCP server and orders the port.
- [docs/content/README.md](docs/content/README.md): the content product's process, state tables, tools and scheduled tasks. [docs/content/thumbnails.md](docs/content/thumbnails.md) is the thumbnail spec its renderer is written against.
- [tools/hltv-harvester/README.md](tools/hltv-harvester/README.md): the harvester's store, its modes and the `ingest` commands.
- `docs/product-vision.md` is a human doc on the product direction. Agents need not read it.

## Agent skills

What the engineering skills (grilling, specs, tickets, triage) read before they work:

- [Issue tracker](docs/agents/issue-tracker.md): issues and specs live in this repo's GitHub Issues.
- [Triage labels](docs/agents/triage-labels.md): the five default triage labels.
- [Domain docs](docs/agents/domain.md): single-context, with `GLOSSARY.md` at the root and ADRs in `docs/adr/`.

## Issues

Outside contributors only file issues, through their own AI agent. `CONTRIBUTING.md` tells that
agent what evidence to collect and `.github/ISSUE_TEMPLATE/bug-report.md` holds the sections.
Keep both in step with the PR body sections and with what `studio_status` returns
(`cmd/zv-mcp/tools.go`, instructions in `cmd/zv-mcp/main.go`).

## Maintenance

- When agents repeat a mistake, add a test or lint first. Write a rule only for what a check cannot catch.
- A correction for a rule already in the Golden rules table, marked "Review only", is a repeat: turn it into a check in the same change. Drop a row once its mistake can no longer be written.
- A rule states the invariant and names the test, linter or `docs/incidents.md` section behind it. It does not copy counts, lists or messages from the code.
- A rule changes in the same PR as the code it describes. [Diff review](.claude/skills/diff-review/SKILL.md) checks it, and [Release check](.claude/skills/release-check/SKILL.md) rereads the rules before each release.
- Each fact lives in one file; the others link to it.
- A spec of a shipped feature is deleted once it ships; its still-true invariants move into the rule for that area.
