---
status: accepted
---

# A content product in the monorepo, beside Studio and ClipMind

ClipHub has two verticals: clipping and YouTube content (full demo POVs, Shorts, Gemini Neon thumbnails), and AI analytics with MCP (ClipMind). The content work runs today outside the repo, in Claude scheduled tasks, skills and loose JSON state on the owner's PC. We bring it into this public monorepo as its own TypeScript product, for the owner only, and keep the repo one workspace. This ADR covers the monorepo structure and the content vertical; ClipMind analytics is out of its scope.

## Decision

- **Layout.** The top-level folders stay (`web`, `desktop`, `landing`, `mcp-apps`, `tools`, `services`, `cmd`, `internal`), and `content/` joins them at the root. One root pnpm workspace with a catalog holds the shared versions. Node 26 and the latest versions of everything, TypeScript 7 where the tool supports it.
- **Content product.** TypeScript, owner-only for now, never in the installer. It is its own stdio MCP server, `cliphub-content`, registered in a root `.mcp.json`, and it talks to Studio only through the `zv-mcp` MCP. Long jobs use MCP Tasks. Skills come over MCP, with `.claude/skills` as the fallback. A thin CLI over the same core serves tests and debugging.
- **Tools over prompts.** Every step with one right answer is a TS tool; the LLM supplies the creative inputs and the tool validates them. A step moves from a skill to a tool once the tool passes the golden tests of its spec, with no shadow period. Skills shrink to when and why to call a tool.
- **Thumbnails.** `docs/content/thumbnails.md` is the source of truth, and the owner approves it before any renderer code.
- **State.** Local SQLite (`node:sqlite`) with a daily copy to the VPS. No loose JSON state and no private repo. Per-PC settings live in `content/config.local.json` (gitignored) beside a committed `config.example.json`. Tokens stay where they are today.
- **Platforms.** YouTube only, X later. TikTok and Instagram are out: the new design has no queue-cut, shorts-queue, published-log, crossposted_queued or vertical-repost.
- **Metrics.** `collect_metrics` runs every 8 h and stores the raw JSON of the YouTube Data and Analytics APIs in SQLite; `export_metrics` runs on demand. The weekly analysis task retires afterwards.
- **HLTV ingest.** The community SQLite on the VPS (`services/community`, table `pro_match_maps`, keyed by HLTV match id and map) is the single registry. Downloads stay on the PC, because Cloudflare needs the home IP and CS2 capture needs Windows. The harvester asks the VPS before it downloads. The content task's fast path imports the local file at once and uploads it to the VPS in parallel; the 24 h sweep skips what already exists. `daily.sh` moves into the repo.
- **Runner.** Claude scheduled tasks stay the runner. Their prompts shrink in the last PR, together with the owner.
- **Agent docs.** The root `AGENTS.md` is the one index (`CLAUDE.md` stays `@AGENTS.md`), and `.claude/rules` and `.claude/skills` stay where they are.

## Planned PRs, in order

1. Agent docs: `AGENTS.md` as the monorepo index, the doc audit and this ADR.
2. Root pnpm workspace with a version catalog.
3. Node 26 and dependency upgrades, TypeScript 7 where supported.
4. Single HLTV ingest against `pro_match_maps`, with `daily.sh` in the repo.
5. `content/` scaffold: the `cliphub-content` MCP server, its CLI and its SQLite.
6. Thumbnails: the spec first, then the tool.
7. `collect_metrics` and `export_metrics`.
8. Titles, the planner and the YouTube upload.
9. Shorter scheduled-task prompts, and the retirement of `Roles\cliphub`.

## Considered options

- **A private repo for the content work.** Rejected: two repos let the Studio MCP contract and the content tools drift, and the code holds no secrets once tokens stay outside it.
- **Keep the content steps as skills and prompts.** Rejected: a step with one right answer is cheaper and safer as a tested tool, and a prompt cannot be golden-tested.
- **Ship the content product in the Studio installer.** Rejected for now: it serves one channel, and the installer stays the product we sell.
- **Download HLTV demos on the VPS.** Rejected: Cloudflare challenges the VPS IP, and capture needs a Windows PC with CS2 anyway.

## Consequences

- The repo gains a second TypeScript runtime product, so its packages join the root workspace and the `check` gates as they land.
- Until the content PRs land, the content flow keeps running from the scheduled tasks and `Roles\cliphub`; `AGENTS.md` describes only what exists in code.
- The harvester and the content task depend on the community API being reachable; a VPS outage delays ingest but loses no download, since files stay on the PC.
