# ClipHub content

The content vertical turns pro CS2 matches into videos for the ClipHub YouTube channel: a full-map POV
(long video) per map, Shorts of its best plays, and thumbnails with Gemini Neon backgrounds. It is for
the owner only and is not part of the Studio installer.

The code lives in `content/` (package `cliphub-content`). It runs as its own stdio MCP server,
`cliphub-content`, registered in the root `.mcp.json`, and reaches Studio only through Studio's own MCP
(`zv-mcp`). A thin CLI (`pnpm --dir content run cli <tool> [json]`) runs the same tools in process for
tests and debugging.

## The rule

Every step with one right answer is a TypeScript tool. The model supplies the creative inputs (which
play, the hook words, the blurb, the Gemini art) and the tool validates them before it records anything.
A step moves from a skill to a tool once the tool passes the golden tests of its spec; there is no
shadow period. Skills then shrink to when and why to call a tool.

## The process

1. **Watch**: find finished maps of the followed event on HLTV, and FACEIT matches of followed pros, and
   pick the player of each map (the MVP for an event map).
2. **Plan**: rank the day's matches and give each map a slot (POV and Short, or Short only) within the
   YouTube cadence.
3. **Get the demo**: the HLTV harvester downloads event demos on this PC (Cloudflare needs the home
   IP); FACEIT demos come from the match room in the browser. The community registry on the VPS
   (`services/community`, table `pro_match_maps`, keyed by HLTV match id and map) says what is already
   there, so the harvester asks it before it downloads.
4. **Record**: Studio imports the demo, records the POV with the Focus HUD and renders the Short
   (CS2 capture needs Windows).
5. **Package**: titles, descriptions, tags, chapters and thumbnails.
6. **Publish**: upload to YouTube in the cadence, link each Short to its POV as the related video and
   check that YouTube shows the custom thumbnail.
7. **Clean up**: once a map is done, delete its Studio videos and captures, its thumbnails and its demo.
   The map's record, descriptions and receipts stay.
8. **Measure**: channel and per-video metrics feed the weekly review.

YouTube is the only platform; X may come later. TikTok and Instagram are out: no vertical cut, no
shorts queue and no cross-posting state.

## State

One SQLite file (`node:sqlite`) per PC, at the path in `content/config.local.json` (default
`%LOCALAPPDATA%\cliphub-content\state.db`). A daily copy goes to the VPS later; there are no loose JSON
state files and no private repo.

| Table | Holds |
|---|---|
| `maps` | one row per map POV: source (`event`, `faceit` or `clip`), player, match facts, stats, demo, Studio job, the POV upload, a skip and the clean-up time |
| `shorts` | each Short of a map: segments, play, files, copy, YouTube id, related-video link and thumbnail check, or why it was not published |
| `uploads` | YouTube upload receipts |
| `metric_snapshots` | raw API JSON with the time it was fetched |
| `migrations` | the applied schema versions |

A map's stage (`open`, `ready`, `done`, `cleaned`, `skipped`) and each Short's status are derived from
these fields in `content/src/state/model.ts`, never stored. `record_map_event` is the only way to change
them and refuses an illegal step, such as cleaning a map that is not done. Legacy fields without a
column are kept in a `legacy_json` column, so an import loses nothing.

Config: `content/config.example.json` is the template; copy it to `content/config.local.json`
(gitignored). `CLIPHUB_CONTENT_*` variables override it (`CONFIG`, `STATE_PATH`, `LEGACY_MAPS_DIR`,
`LEGACY_METRICS_DIR`, `STUDIO_MCP_COMMAND`, `RENDER_BIN`). Tokens stay where they are today; the config never
holds one.

## MCP tools today

| Tool | Does |
|---|---|
| `content_status` | config found, state path and schema version, row counts, whether `zv-mcp` answers `studio_status` |
| `list_maps`, `get_map` | maps with their derived stage and statuses |
| `record_map_event` | one step of a map: POV or Short published, Short linked, thumbnail checked, skips, clean |
| `import_legacy_state` | reads the old `map.json` folders, their `*.youtube.json` receipts, `channel-snapshots.jsonl` and `videos-latest.json`; a dry run unless `write` is true, and it only reads those folders |
| `render_gpu_info` | probes the discrete GPU adapter (Vulkan/DX12) and FFmpeg NVENC encoders via `tools/render` (`cliphub-render`) |
| `validate_essay_spec` | validates a `ProjectSpec` or multi-scene `EssaySpec` JSON against `tools/render` and verifies all media paths |
| `snapshot_essay` | renders fast GPU PNG snapshots at requested timestamps for visual QA before a full render |
| `start_essay_render`, `get_essay_render_status` | launches a non-blocking GPU render (`--require-gpu` on by default) via `tools/render` into a file that does not exist yet, and polls completion + structured FPS/timing stats |

Long jobs (renders, uploads) will run as MCP Tasks. The TypeScript SDK (v2) has no server runtime for
Tasks yet (`@modelcontextprotocol/ext-tasks` covers the client side only), so those tools are not built
until it does.

## Scheduled tasks

Claude scheduled tasks stay the runner. Their prompts shrink last, once the tools below exist.

| Task | When | What it does today | Tools that replace steps |
|---|---|---|---|
| `cliphub-pro-pov-2-2` (production) | every 30 min | Holds a lock so only one run produces. Watches HLTV for finished event maps and creates their records; picks FACEIT matches of followed pros by hook (ADR, kills against deaths, kill count) and rotation; downloads demos with the harvester; records the POV and Short in Studio when auto approval is on and CS2 is not in use; writes copy and Gemini thumbnails; uploads in the cadence; links Shorts and checks thumbnails in YouTube Studio; cleans up done maps | state and transitions (this PR); a lock tool; `plan_today` (next actions per map and slot); cadence check (rolling 24 h window, spacing, Madrid hours); title, description, tag and chapter builders with validators; thumbnail renderer built from `docs/content/thumbnails.md` once the owner approves that spec; YouTube upload, thumbnail and verify; clean-up plan and run; FACEIT pick and rotation; demo fetch through the harvester and the VPS registry |
| `cliphub-plan-dia-major` | daily before the first match of the event | Reads the event's matches for the day on HLTV, ranks them (team rankings, regions, famous players) and writes the day's plan with a slot per match | ranking and slot assignment as a tool; the model only reads the HLTV pages |
| `cliphub-estado-15min` | every 15 min | Read only: lists what was published since the last report, what is being prepared and the queue length, and whether Studio or a token blocks | `content_status` plus a published-since query over the state |
| `cliphub-metricas-8h` | every 8 h | Reads channel totals and recent uploads from the YouTube API, appends a channel snapshot and writes per-video views, then reports growth | `collect_metrics` (raw Data and Analytics API JSON into `metric_snapshots`) and `export_metrics` |
| `cliphub-analisis-semanal` | weekly | Exports the YouTube Studio table, compares the week with the last one by group (FACEIT or event, Short or POV) and proposes at most one change | retires later, once `export_metrics` covers its numbers |
| `cliphub-lineups-pro-diario` | daily | Runs the HLTV harvester for recent top-team demos, prepares the pro analyses, uploads new ones to the VPS and reindexes | the daily script moves into the repo; the harvester asks the VPS registry before each download; the content task's fast path imports a demo at once and uploads it in parallel, and the 24 h sweep skips what exists |
| `cliphub-pro-pov`, `cliphub-pro-pov-2` | superseded | Earlier versions of the production task | none; retire them |
| `cliphub-vertical-repost` | retired by design | Cross-posted Shorts to TikTok and Instagram | none: those platforms are out |
| `cliphub-x-daily-pulse` | paused | Drafts X posts about CS2 news, never posts | a future X step |

The legacy role folder (`Roles\cliphub`) retires once its state is imported and the tasks point at
these tools.
