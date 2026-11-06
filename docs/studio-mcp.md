# Studio MCP server: implementation plan

## Goal

Studio has two kinds of users, and both are first class. Some work in the Studio UI, where the
visual tasks live (reviewing plays, the editor, the analysis views). Others prefer to stay in an
assistant (Claude Desktop, ChatGPT Desktop) and do everything from there. The MCP server serves the
second group without taking anything from the first: both drive the same orchestrator, a job
started from the assistant shows up in the UI, and the UI stays complete on its own.

Let a user drive ClipHub Studio from an assistant in plain language:
"import my last demo and make a Short of the two best clutches". Claude acts on the Studio the
user has open, through the same orchestrator the UI uses, so every job Claude starts shows up in
the app as it runs.

The MCP server is a thin adapter over the orchestrator HTTP API. It adds no domain logic: what a
Short is, how plays are picked and how renders run stay in the orchestrator.

Why MCP and not only a CLI: MCP gives Claude live job progress, in-chat approval before expensive
work, and a one-click path for customers who use Claude Desktop.

## History: the first MCP

Studio already shipped a local MCP server once. `768ffe32` (2026-07-13) added `desktop/src/mcp/`:
a TypeScript stdio server with a discovery handshake, generic `search`/`execute` tools,
preview-then-`confirmed=true` mutations and a Settings registration card (released in 2.1.0 and
2.2.0). `5422982d` (2026-07-20) retired it, the same day an in-app Codex agent landed
(`936f15c6`), and later releases removed the `discovery_secret` it kept in `ports.json`
(`desktop/src/stable-ports.ts`). The retire commit gives no reason; the reason, per the product
owner: in July 2026 MCP clients were immature and few users worked from an assistant. Both have
changed. `web/lib/webmcp.ts` is a still-live WebMCP preview.

What this attempt changes:
- Product-action tools (`generate_short`) instead of a generic `execute` over every endpoint.
- Go in `cmd/zv-mcp`, reusing `internal/tuiclient`, instead of 5,600 lines of TypeScript beside
  Electron.
- An explicit opt-in switch, so the session file exists only for users who connect Claude.
- Protocol features that did not exist or were not used then: elicitation and progress.

Decide in phase 1 whether the WebMCP preview stays.

## Product behavior

- Settings has an "Asistentes (Claude, ChatGPT)" card with the switch "Permitir que un asistente use
  Studio" (default off) and two copy buttons: the Claude Desktop config snippet and the
  `claude mcp add` command. Both point at a stable copy, `<userData>\mcp\zv-mcp.exe`, so a running
  `zv-mcp.exe` never blocks the auto-updater and a custom install folder never breaks the config.
  A `zv-mcp` the assistant kept running through an update keeps the old build in memory. It reads
  Full Demo plans and options as strictly as Studio stores them, so a field one side lacks asks
  for an assistant restart instead of passing as a stale plan.
- With the switch off, every tool answers with a clear message saying where to turn it on.
- Starting a Short or a Full Demo launches CS2 and records for minutes, so both ask for approval
  in the chat before they run. Rendering stream videos (`render_stream`, `render_stream_summary`)
  encodes for minutes and deleting (`delete_video`, `delete_stream`, `delete_demo`, `delete_captures`,
  `delete_storage_file`) or merging teams (`merge_teams`) cannot be undone, so they ask too.
  So do `retry_video` and `change_video_music`, which render a Short again.
- The user's own Claude subscription pays for the model. Studio adds no API key and no cost.

## Architecture

```
Claude Desktop / Claude Code
        |  stdio (MCP)
   zv-mcp.exe  (cmd/zv-mcp)
        |  HTTP + X-ClipHub-Token, loopback only
   zv-orchestrator  (started by Electron)
        ^
   Electron writes <userData>/agent-session.json after boot
```

- `cmd/zv-mcp`: Go, official SDK `github.com/modelcontextprotocol/go-sdk`, stdio only, no port.
  The spike pins the version and confirms which protocol features each client supports.
- HTTP goes through `internal/tuiclient` (base URL, token header, multipart, `APIError`, Full
  Demo helpers). Add the missing calls there. `tuiclient.New` already reads `ORCHESTRATOR_URL`
  and `ZV_MUTATION_TOKEN`, so the Go server can be built and tested against a locally run
  orchestrator before any Electron change.

### Session discovery

The orchestrator port is stable across boots (`ports.json`), but the mutation token is minted per
boot, and production requires it even for reads (`requireMutationToken` in
`internal/httpapi/routes.go`).

1. After `waitForDesktopServices` succeeds, Electron always writes `<userData>/agent-session.json`
   atomically: `{ "version": 1, "enabled": false }` when the switch is off, and
   `{ "version": 1, "enabled": true, "orchestratorUrl": "http://127.0.0.1:<port>", "token": "<hex>", "webOrigin": "http://127.0.0.1:<port>" }`
   when it is on. "No file" then means "Studio is closed", and `enabled: false` means "switch off".
2. Electron rewrites it at once when the switch changes (session file first, then
   `agent-access.json`), retries a rename that Windows refuses while `zv-mcp` reads the file, and
   falls back to deleting it when turning access off fails, so no token stays on disk. It deletes
   the file on quit, on boot failure and at the start of a new boot attempt (a new boot mints a new
   token). `webOrigin` serves public UI assets such as the HUD previews (`/hud/*` needs no cookie).
3. `zv-mcp` finds userData at `%APPDATA%\cliphub-studio`, overridden by
   `CLIPHUB_STUDIO_USER_DATA`. This is a new variable: `CLIPHUB_E2E_USER_DATA` is consumed and
   deleted by `isolated-userdata.cjs`, so the MCP cannot inherit it.
4. Staleness: probe `/healthz` (outside auth) and reread the file on any 401. No pid check: pid
   liveness on Windows needs Win32 calls and pid reuse makes it unreliable.

Security: the web child already holds the same token in its environment (`boot-security.ts`), and
any process of the same Windows user can read the profile's database and media, so a file in
userData keeps the boundary at "the logged-in user". The switch keeps the token off disk for users
who never connect Claude. The token never appears in tool output, logs or telemetry.

The settings switch needs a new store file, a new action in the typed settings IPC
(`studio-settings-ipc.ts`), preload wiring and a web Settings component. The retired
`web/components/settings/mcp-settings.tsx` (in `5422982d^`) is a starting point.

### Long jobs and progress

There is no event stream; the UI polls.
- Start tools return right away with the job id and first status.
- `wait_for_job` blocks up to 50 s by default, polls `GET /api/jobs/{id}?view=status` every 2 s
  and sends MCP progress notifications. It returns the final status with `failure_code`, or
  "still running" so Claude calls it again.
- After capture a Short job stays `recorded`; the render outcome lives in
  `GET /api/jobs/{id}/renders/{variant}`, and `render/render-progress.json` is never deleted. So for
  `recorded`, `wait_for_job` decides from the render variant state (queued or rendering is running,
  failed is a failure, ready is done), never from the progress file alone. `generate_short` and
  `render_full_demo` return the variant to pass in.
- Library demos have their own states (`queued`, `parsing`, `ready`, `failed`): `wait_for_demo`.

The orchestrator rate limiter is a pass-through in Studio (`WithRateLimit(0, 0)`), so polling is
safe. Only uploads are limited (2 concurrent).

### Approval

- Elicitation before `generate_short` and `render_full_demo`, showing what will run: player, map,
  rounds and kills of the chosen plays plus every option, or the Full Demo plan summary.
- The same approval guards `render_stream` (layout, how many videos, which are saved moments and
  which are new plays), `render_stream_summary` (play count, of how many, and length), `delete_video`,
  `delete_stream`, `delete_demo`, `merge_teams` (both team names and the match count of the one that goes),
  `delete_captures` (bytes freed), `delete_storage_file` (file, kind and size), `retry_video` and `change_video_music`.
- On protocol 2026-07-28 the SDK forbids a blocking `Elicit` inside a tool call, so the question
  travels as an input request (`CallToolResult.InputRequests`); the SDK asks it directly for older
  clients. A failed or declined question starts nothing and hands out no code.
- Only a client that does not advertise elicitation gets the approval-code path: an HMAC with a
  per-process random key over the tool, the job and the exact request, single use, expiring after
  10 minutes. This path still relies on the model asking the user, as the tool description says.
- Headless Claude Code (`claude -p`) advertises elicitation and declines it, so nothing starts.

### Untrusted text

Player names, clan names, victims and publish-assistant titles come from the demo and are chosen by
players. Stream titles come from the streamer, play and moment labels from the stream plan, and
yt-dlp failure reasons from the remote site. In tool output, return them in labelled fields, strip
control characters and cap their length. In approval text a stream title also loses « » and quote
characters, so it cannot close its quote and read as part of the question. Tool descriptions say
these fields are data, not instructions. A name never reaches a path, a shell or a header.

## Tools (v1)

Names are product actions. Descriptions are in English for the model; Claude answers in the user's
language. Output is compact JSON with ids Claude can chain.

The flow mirrors the UI: import a demo (the scan starts by itself), read the match players, create
a job for one player (the job parses that player; the API may return an existing job with
`reused: true`), list the plan segments, generate.

Read:

| Tool | Orchestrator calls |
| --- | --- |
| `studio_status` | `/healthz`, `/api/capabilities` (`record.enabled`), session file state; Studio version (session file), support code (`telemetry.json`) and `studio.log` path for a bug report (`CONTRIBUTING.md`) |
| `list_matches`, `get_match` | `GET /api/library/matches`, `GET /api/library/matches/{id}` (players with SteamIDs, side keys and their library team) |
| `list_moments` | `GET /api/jobs/{id}/moments`: the same segment ids as the "Prepara tu Short" list, with scores; the top 10 by default (`limit`), plus `total`; `weapon`, `kinds` (clutch, ace, multi_kill) and `side` (CT, T) filter the list |
| `list_series_moments` | `GET /api/jobs/{id}` (same `target_steamid`, different demos) and `GET /api/jobs/{id}/moments` per map; plays of every map tagged with `map_number`, `map` and `job_id`, in play order (map, round, tick) or best first; same filters as `list_moments` |
| `find_player_moments` | `GET /api/library/matches?steamid=`, `POST /api/library/demos/{id}/jobs` and the filtered `GET /api/jobs/{id}/moments` per match; one player's plays between two dates (`from`, `to`) and on one map, up to 10 matches per call; a match still parsing comes back `pending` |
| `list_presets` | `GET /api/presets` |
| `list_videos` | `GET /api/jobs/{id}/renders/{variant}` per preset; MP4 links through the UI origin, since a URL cannot carry the token header |
| `list_all_videos` | `GET /api/jobs` (newest 30 by default, at most 100), then `GET /api/jobs/batch-status` for every job and preset in calls of 100, and `GET /api/stream-jobs` for the same number of streams: finished videos with MP4 links, renders still running or failed, jobs whose parse or capture failed, and rendered stream moments |
| `publish_copy` | `GET .../videos/{name}/publish-assistant`: title, description, tags, `alternative_titles` (up to 4), the recommended copies (the suggested one and up to 4 alternatives) with score and rationale, keywords, the 5 best publish times across the schedule days, and trending terms (or why they are missing) |
| `list_pro_settings` | the `internal/prosettings` catalog (no orchestrator call): id, name, team, resolution and viewmodel FOV for `plan_full_demo` `pro_settings` |
| `list_teams` | `GET /api/library/teams` |
| `get_team` | `GET /api/library/teams/{id}`: aliases and the roster by date |
| `list_players` | `GET /api/library/players` (`q` nick, `team_id`, `limit` 50 by default and at most 200): a nick to a SteamID64 |
| `get_player` | `GET /api/library/players/{steamid}`: nicks and teams by date, linked FACEIT account |
| `scout_team` | `GET /api/library/teams/{id}/scout` (`map`, `side`, `from`, `to`, repeatable `opponent_id` and `match_id`), summarized: maps, buys, sites, round shapes, openings, executes, AWP, players |
| `player_metrics` | `GET /api/library/players/{steamid}/metrics`; rates with the `get_baseline` definitions |
| `get_baseline` | `GET /api/library/baseline` (`scope` library or pro, `map`, `min_rounds`) |
| `find_rounds` | `GET /api/library/matches?team_id=` or `?steamid=`, then `GET /api/library/matches/{id}/tactical` per match (up to 20); round refs `match_id/round` filtered by side, buy and result |
| `team_bombsites` | `GET /api/library/teams/{id}/bombsites` (`map`, `side`, `from`, `to`, `opponent_id`, `match_id`); per map and site, the CT hold and retake rates and the T attack, plant and post-plant rates |
| `team_playbook` | `GET /api/library/teams/{id}/playbook` (`map` and `side` required, `from`, `to`, `opponent_id`, `match_id`); category, site and zone shares with win rates, the tempo median, and up to 5 round refs per row |
| `team_prep` | `GET /api/library/teams/{id}/prep` (`map` required, both sides, `from`, `to`, `opponent_id`, `match_id`); buy matchups, tempo per buy, and each CT player's callouts per time window, with up to 5 round refs per row |
| `team_grenades` | `GET /api/library/teams/{id}/grenades` (repeatable `kind`, `map`, `side`, `from`, `to`, `opponent_id`, `match_id`); per kind the count, sides, median second and top throwers, then compact rows with the round ref and rounded world positions, 30 by default and at most 100 |
| `player_positions` | `GET /api/library/players/{steamid}/positions` (`map`, `from`, `to`); per map and side the zones, callouts, CT role and AWP use |
| `leaderboard` | `GET /api/library/leaderboard` (`metric` required, `scope`, `map`, `min_rounds`, `limit` 20 by default and at most 100) |
| `list_streams` | `GET /api/stream-jobs` (`limit` 20 by default and at most 100) |
| `list_stream_variants` | `GET /api/stream-variants`; aspect ratio and whether the layout shows the facecam |
| `list_stream_videos` | `GET /api/stream-jobs/{id}` (`rendered_outputs`), `GET /api/stream-jobs/{id}/summary`, and `GET .../renders/{variant}` per rendered layout for its `delivery` files other than MP4s ("Archivos adicionales": cover, metadata, plan, manifest); links through the UI origin |
| `get_stream_edit_plan` | `GET /api/stream-jobs/{id}` and `GET .../edit-plan`: moments with their `edit`, banners, music, kill feed, effects, layout, crops and `face_crop_reviewed`, and `updated_at` |
| `wait_for_stream` | polls `GET /api/stream-jobs/{id}` until it leaves `acquiring` |
| `wait_for_stream_render` | polls `GET /api/stream-jobs/{id}/renders/{variant}`, or `GET .../summary` for variant `summary`; while `rendering`, `render_progress` gives the percent and the video being encoded (`progress`, such as `video 2 of 3`) |
| `storage_usage` | `GET /api/storage`: total bytes and the Full Demo captures a finished render covers (`freeable`), largest first |
| `list_storage_files` | `GET /api/storage/files` (the first call starts the PC demo scan): PC demos, Studio demo copies and videos, newest first, 50 per list by default and at most 500, with totals; a path with control or format characters is left out |
| `steam_matches` | `GET /api/steam/account`: recent matches and whether the history and a Steam session are set; secrets are never returned |
| `job_players` | `GET /api/jobs/{id}/roster` of a job that `import_share_code` left `scanned` |
| `check_integrity` | `GET /api/jobs/{id}/anticheat`; a 409 (never screened) starts it with `POST`; players most suspicious first with their top 3 metrics and the report's limitations |
| `map_3d_status` | `GET /api/maps/{map}/scene/summary`, which reads the map's render scene from the CS2 install on first use; triangles, textures and whether it has baked light, sun, fog, sky and post processing (exposure, tonemap and colour LUT), how many reflection probes and live lamps it has, plus how to open the real map in the Táctica 3D view, the 2D Mapa real floor plan and full screen |

Write:

| Tool | Orchestrator calls | Approval |
| --- | --- | --- |
| `import_demo` | `POST /api/library/demos` (multipart `tag` fields, then `demo`; the server applies only tags sent before the file, and a duplicate keeps its own tags) | no |
| `update_demo` | `PATCH /api/library/demos/{id}` with `source` (not `shared`) and/or the whole `tags` set | no |
| `reanalyze_demo` | `POST /api/library/demos/{id}/analyze`; a demo already queued or parsing is left alone, a shared match answers 409 | no |
| `create_job` | `POST /api/library/demos/{id}/jobs` with `target_steamid`; with `reparse` on a reused job, also `POST /api/jobs/{id}/parse` | no |
| `create_series_jobs` | per map: `GET /api/library/demos/{id}` and `GET /api/library/matches/{id}` to find the player by SteamID in every map, then `POST /api/library/demos/{id}/jobs`; nothing is created if one map lacks the player | no |
| `generate_short` | `POST /api/jobs/{id}/generate` with `preset` (default from `list_presets`) and `segment_ids` from the plan; one job (one map) per call, so a series gets one Short per map. `music` goes as `{key, volume, game_volume}` when `music_volume` or `game_volume` is set (percents in the tool, gains on the wire, full music volume left unset like the UI); `intro_text`/`outro_text` and the affiliate banner (`keydrop_family`, `keydrop_style`, `keydrop_code`, a 0 to 4 s window by default) go in `edit` | yes |
| `render_full_demo` | `GET /full-demo/plan` defaults, `POST /full-demo/plan`, then `POST /generate` with preset `gameplay-pov-60` and the approval (`tuiclient.GenerateFullDemo`) | yes |
| `wait_for_job` | polls a job; takes the render `variant` | no |
| `wait_for_demo` | polls a library demo until `ready` or `failed` | no |
| `plan_full_demo` | defaults plus the user's HUD and voices, `POST /full-demo/plan`; returns duration and `plan_hash`. `pro_settings` resolves a catalog pro into the capture (or `none`); `hud_portrait` uploads a PNG or JPG to `POST /api/full-demo/overlay-images` (focus HUD only); `intro_bumper`, `sponsor_bumper` and `outro_bumper` upload an MP4 to `POST /api/editor/assets` with the UI's local-file provenance. Like every option, they start from the defaults, not from a plan saved in the UI | no (saves the plan the UI also shows) |
| `import_stream` | `POST /api/stream-jobs`: JSON `source_url` (Twitch, Kick or YouTube, checked with `vodfetch.ValidateSource` first), `title`, `source_kind`; or multipart `video` for a local `.mp4` | no |
| `find_stream_plays` | `GET /api/stream-jobs/{id}/plays`, `POST` when idle or with `rescan`; plays ranked by score, 20 by default | no |
| `edit_stream_plan` | `GET` and `PUT /api/stream-jobs/{id}/edit-plan?expected_updated_at=` (raw fields; each part given replaces that part, the rest is kept; Studio answers 409 when another save landed after `expected_updated_at`); new moments get free `clip-NNN` ids; a music key is checked against `GET /api/songs` | no (saves the plan the UI also shows) |
| `render_stream_summary` | `POST /api/stream-jobs/{id}/summary` with `play_ids` when given: one 16:9 video of those plays (every play when empty) | yes |
| `render_stream` | `GET` and `PUT /api/stream-jobs/{id}/edit-plan` (raw fields, so banners, music and effects survive; `play_ids` are added to `clips` as "Añadir" does, skipping plays that overlap a saved moment; an `uploaded` stream's plan is always saved, which makes it `ready`; the save carries `expected_updated_at`), then `POST .../renders/{variant}` with `expected_edit_plan_updated_at`; a facecam layout with an unchecked crop is refused before the approval | yes |
| `delete_video` | `DELETE /api/jobs/{id}/renders/{variant}/videos/{name}` | yes |
| `retry_video` | `GET /api/jobs/{id}?view=status` and `GET .../renders/{variant}` (a `failed` render with the effective `edit`, `music` and `segment_ids` of its edit document), then `POST .../renders/{variant}` with them. Studio keeps those choices only for the last finished version (a failed render points at it), so a Short that never finished cannot be retried this way and a failed replacement renders that version again. A failed capture, a failure that needs a new capture (`recording_not_reusable:` and the others of `requiresRecapture`) or a Full Demo is refused with a pointer to `generate_short` or `render_full_demo` | yes |
| `change_video_music` | the same reads on a `ready` Short, or on a `failed` one that has a finished version (this retries a failed music change), then `POST .../renders/{variant}` with the new `music` (`{key, volume, game_volume}`, or none) and the stored `edit` and `segment_ids`; the same mix as now is refused, as in the UI | yes |
| `delete_stream` | `DELETE /api/stream-jobs/{id}` | yes |
| `delete_captures` | `GET /api/storage` (the job must be `freeable`), then `DELETE /api/jobs/{id}/captures` | yes |
| `rescan_storage_files` | `POST /api/storage/files/scan`, then `GET /api/storage/files` | no |
| `delete_storage_file` | `GET /api/storage/files` (the server only accepts a path its last list returned; a `studio_demo` goes with `delete_demo`), then `POST /api/storage/files/delete` | yes |
| `reveal_storage_file` | `GET /api/storage/files`, then `POST /api/storage/files/reveal` (File Explorer on this PC) | no |
| `sync_steam_matches` | `POST /api/steam/matches/sync`; `history_not_configured` and `need_known_code` point the user to Ajustes | no |
| `import_share_code` | `POST /api/steam/import` with `code` only: Studio downloads with the Steam session the user started in the UI, else `steam_credentials_required` asks the user to sign in there. Makes a Crear job | no |
| `pick_player` | `POST /api/jobs/{id}/parse` with `target_steamid` | no |
| `delete_demo` | `GET /api/library/demos/{id}` for the summary, then `DELETE /api/library/demos/{id}` (demo, match, analysis and corrections; videos and jobs stay) | yes |
| `update_team` | `PATCH /api/library/teams/{id}` (`name`, `aliases` replaces the set, so the list starts from `get_team`: a dropped clan name makes a new team on the next import) | no |
| `assign_match_team` | `PUT /api/library/matches/{id}/teams/{key}` (`team_id`, `team_name` found or created, or cleared; `remember_alias`); `get_match` lists the side keys | no |
| `merge_teams` | `GET /api/library/teams/{id}` for both teams, then `POST /api/library/teams/{id}/merge` with `into_team_id` | yes |

Short look and Full Demo HUD:

| Tool | What it does |
| --- | --- |
| `list_short_options` | format, kill effect, transition, music, extras and the affiliate banner families and styles, each with a one-line Spanish description and the default |
| `get_song_audio` | one track as audio content, app-only (`_meta.ui.visibility: ["app"]`) |
| `list_hud_themes` | the 11 custom HUDs plus "Original de CS2", with 640x360 JPEG thumbnails of the previews as image content (about 4k tokens for all 11) |
| `choose_short_options` | MCP App `ui://cliphub/short-options`: picker with play buttons; text fallback without UI |
| `choose_hud` | MCP App `ui://cliphub/hud-gallery`: preview grid that calls `plan_full_demo` |

`import_demo` accepts only an absolute local-drive path to a `.dem` or `.dem.zst` file (no UNC
paths), refuses directories and files above the server's 700 MiB per-demo limit, and leaves real
validation to the orchestrator. Every id argument is validated (UUIDs, `seg-` ids, strict variant
and video names) before it reaches a URL, and a panic in a handler is recovered so the stdio server
survives.

`studio_status` reports whether capture is enabled, so Claude says so before it starts a Short
instead of failing halfway.

Not covered yet (see the parity rule in AGENTS.md): the editor, collections, the stratbook,
scouting reports, library series, FACEIT follows, practice launch, community ranking, `open_studio`, resources and prompts. Steam sign-in, the authentication code
and the Web API key stay in the UI on purpose: an assistant never handles Steam credentials. There is no tool to cancel a queued
render: the orchestrator has no cancel route, and Quitar on a queued video in Mis vídeos only drops
the UI's local record of it.

## Spike results (2026-11-01)

- SDK: `github.com/modelcontextprotocol/go-sdk` v1.8.0. Typed tools give `structuredContent` and a
  JSON text block for free.
- Claude Code (`claude -p` with `--mcp-config`) ran the whole flow against a throwaway
  orchestrator in about 30 s and 9 turns: `import_demo`, `wait_for_demo`, `get_match`,
  `create_job`, `wait_for_job`, `list_moments`, `generate_short` returning `needs_approval`. A
  random-bytes `.dem` came back as `not_a_demo` with a clear message.
- Headless Claude Code does not advertise elicitation, so the approval-code fallback is the path
  it takes. It worked: the model stopped and asked instead of approving on its own.
- A panic in one tool kills the stdio server and the client only sees "Connection closed". Tool
  handlers must never panic; the spike hit this once (`toolError(nil)`).
- A bare orchestrator reports capture enabled even without CS2 on the path checked; do not treat
  `capture_enabled` as proof that a capture will succeed.
- Session discovery is in: Electron writes `agent-session.json` after the health checks when
  `<userData>/agent-access.json` holds `{"enabled": true}` (the Settings switch will write that
  file), and removes it at boot start, on boot failure and on quit. Studio built from the branch
  with a throwaway profile published it, and Claude Code ran the flow against that Studio with no
  URL or token in its config. Re-importing returned `duplicate` and re-creating the job returned
  `reused`, so repeated assistant calls do not duplicate work.
- With no session file, every tool answers "ClipHub Studio is not running. Ask the user to open
  ClipHub Studio and try again".
- Claude Desktop: registered in `claude_desktop_config.json`; checking it needs a Claude Desktop
  restart by the user.
- ChatGPT Desktop is not installed on the development PC, so its stdio support is still unverified.

## Implementation phases

1. **Spike.** Confirm request and response bodies against `internal/httpapi`. Pin the SDK.
   Register a hello-world `zv-mcp` in Claude Code, Claude Desktop and ChatGPT Desktop and check
   tool timeouts, progress and elicitation support in each. Reports disagree on whether ChatGPT
   Desktop runs local stdio servers; if it only takes remote servers, ChatGPT support moves to a
   later phase and v1 stays stdio. Whatever a client lacks, the hash fallback must cover.
2. **Read tools**, against a locally run orchestrator with `ORCHESTRATOR_URL` and
   `ZV_MUTATION_TOKEN`. Map orchestrator errors to clear tool errors (Studio closed, switch off,
   job not found, capture disabled).
3. **Write tools, `wait_for_job`, approval and untrusted-text handling.**
4. **Electron**: session file, settings switch, Settings card with the config snippet and the
   `claude mcp add` command. Go: `internal/studiosession` reads and validates the file.
5. **Packaging**: add `zv-mcp` to `$commands` in `scripts/build.ps1`, copy it in `assemble.mjs`
   (which today copies exactly three exes), and update the release asset list and integrity check
   if it ships outside the installer.
6. **Later**: a `.mcpb` bundle. It embeds its own `zv-mcp.exe`, so it ships as a separate release
   asset with `release-integrity.mjs` and the asset list updated together, not inside the
   installer.

## Tests

- Unit tests where there is real logic: session file validation, error mapping, argument and path
  validation, progress mapping, name sanitizing.
- `TestNoDeadCode` skips under `-short`, and the release runs `-short`, so dead `tuiclient`
  helpers are caught only locally: run `go test ./cmd/zv -run TestNoDeadCode` before the PR.
- No e2e suite. Manual check per `AGENTS.md`: build Studio from the branch with a throwaway
  profile, set `CLIPHUB_STUDIO_USER_DATA` to it, register `zv-mcp` in Claude Code, ask Claude to
  import a demo and reach the "Prepara tu Short" list. Then import a corrupt `.dem` and check that
  Claude reports a clear error and `studio.log` shows it.

## Open questions

1. Does the WebMCP preview (`web/lib/webmcp.ts`) stay?
2. Does ChatGPT Desktop run local stdio servers (answered by the spike)?

## v1 results (2026-11-01)

- Built by a workflow of Sonnet agents in two rounds (implementation, then the fixes from an
  adversarial review), then checked by hand.
- Checks: `go vet` and `go test` for `cmd/zv-mcp` and `internal/tuiclient`, `TestNoDeadCode` for both
  GOOS targets, and typecheck, lint and unit tests for desktop and web all pass.
- Real Studio from the branch with a throwaway profile: Claude Code, configured only with the stable
  `<userData>\mcp\zv-mcp.exe`, imported a demo, picked the top fragger, created the job, listed the
  plays, read the Short options (2 formats, 6 kill effects, 6 transitions, 15 tracks, 7 extras),
  listed 12 HUDs with 11 previews, planned a Full Demo with the `ember` HUD (23 rounds, 1328 s) and
  was refused a UNC path and a corrupt demo. The headless client declined the Short and the render,
  so nothing started. `studio.log` showed no errors and no refused requests, and the match showed up
  in Partidas.
- The Settings switch, clicked in the real window: off rewrote the session to
  `{"version":1,"enabled":false}` and `zv-mcp` answered that access is off; on republished the URL,
  token and `webOrigin` without a restart. A new `zv-mcp.exe` build replaced the stable copy on the
  next boot.
- Not verified yet: MCP Apps rendering, audio playback and the elicitation form in a real Claude
  Desktop or ChatGPT Desktop (ChatGPT Desktop is not installed here), and a real CS2 capture.
- `zv-mcp.exe` is about 27 MB because the Full Demo and Short tools import `recapplan`,
  `renderplan` and `customhud` to validate requests the same way the server does.

## QA pass (2026-11-01)

A scripted MCP client drove `zv-mcp` over stdio against real Studio from the branch (throwaway
profile, two real demos), and a small MCP Apps host emulator ran both views in the built-in browser
under a strict CSP (`img-src data:; media-src data:; connect-src 'none'`).

- About 70 tool cases: bad and unknown ids, relative, UNC, `\\?\` and missing paths, folders,
  empty, garbage and corrupt demos, duplicates (same file in another case), bad SteamIDs, a player
  not in the demo (the job fails with `target_not_found`, as in the UI), out of range limits and
  levels, unknown HUDs, effects, tracks and presets, duplicate and unknown segments, wrong arg types
  and extra args. Every one got a clear tool error or a sane clamp; none crashed the server.
- Approval: decline and cancel start nothing on every protocol; a forged or reused code, or a code
  for other plays or options, is refused; an elicitation client never gets a code path.
- Session file: missing, empty, malformed, version 2, non-loopback or `localhost` URL, https, a
  path in the URL, short or wrong token, dead port, bad `webOrigin` and a 5 MB file each give a
  specific message. A killed Studio reads as closed; a restarted Studio heals the same `zv-mcp`
  process in about 4 s. A normal quit deletes the session file.
- Views: options picker, track previews (switching tracks stops the previous one), the approval
  step and Cancel, the HUD gallery with 11 thumbnails, planning from the gallery, the chat fallback
  when the host does not let the view call tools, and a host that refuses `ui/message`.

Fixed during the pass:

- A client asking for a protocol newer than the server negotiates (for example `2026-07-28`, which
  the stdio handshake answers with `2025-11-25`) got `input_required` results it cannot read, so the
  approval never showed. `pinNegotiatedVersion` stores the negotiated version; the SDK reads the
  requested one.
- Approval text is Spanish and drops the job id and plan hash, since clients show it to the user
  verbatim in the elicitation form.
- A rejected token said "call the tool again", an endless loop when the token stays wrong; it now
  says to retry once and then ask the user to restart Studio.
- Studio could not refresh the stable `zv-mcp.exe` while an assistant ran it (EPERM). It now moves
  the running copy aside, puts the new one in place and deletes old copies on a later boot.
- The picker said "Generado" after only sending the request to the chat; it now says "Enviado" or
  "En marcha", and the views say "el asistente" instead of "Claude".
- Full Demo notices were cut at 64 characters; waits now report 100% progress when they finish.
- `cmd/zv-mcp` was missing from the Makefile and the `zv` pass-throughs, which failed
  `TestZVBinaryCurrentRepoWorkflowChecksEndToEnd`.

Real capture, approved through elicitation: two plays of job `be912ccd`, glitch, zoom-whip and
`pase-directo`. CS2 recorded both plays and Studio rendered `viral-60-clean` in about a minute and a
half: 1080x1920 at 60 fps, 11.8 s, music in the mix, a cover image. `list_videos` returned the MP4
link (served with range requests) and `publish_copy` the title, description and tags. `studio.log`
had no errors besides the deliberate bad SteamID.

That run found one more bug, fixed: right after `generate_short`, `wait_for_job` answered "parsed,
not running", because a worker had not taken the job yet. A capture this process started now counts
as "capture queued" for up to two minutes, until a worker owns the job.

Known noise: `list_videos` without a render asks every preset, and each missing one logs a 404 as an
error diagnostic in `studio.log`.

Still not verified: the views and the elicitation form inside a real Claude Desktop Chat tab or
ChatGPT Desktop. Without tool calls from the view, the picker has no track previews and the gallery
has no thumbnails.
