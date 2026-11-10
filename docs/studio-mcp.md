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
- Starting a Short, a Full Demo or a round POV (`record_round_pov`) launches CS2 and records for minutes, so all ask for approval
  in the chat before they run. Rendering stream videos (`render_stream`, `render_stream_summary`)
  encodes for minutes and deleting (`delete_video`, `delete_stream`, `delete_demo`, `delete_captures`,
  `delete_storage_file`) or merging teams (`merge_teams`), and deleting a report or a play
  (`delete_report`, `delete_play`) cannot be undone, so they ask too.
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
- In a host with MCP Apps, the tools that start or follow a long job show the progress view
  `ui://cliphub/job-progress`: a bar with the Spanish stage ("Montando corte 17 de 18"), the
  percent, the time since the view opened and, once done, the video name from `list_videos` or
  `list_stream_videos`. The view calls `wait_for_job`, `wait_for_demo` or `wait_for_stream_render`
  itself with `wait_seconds` 5 until the job ends, so it keeps moving after the assistant's turn is
  over. A view scrolled out of the chat stops polling until it is back in sight. Hosts without UI
  get the same JSON as before.

The React views live in `mcp-apps/` (its own pnpm package, like `web/` and `desktop/`).
`pnpm --dir mcp-apps run build` bundles each view with esbuild into one self-contained HTML file in
`cmd/zv-mcp/apps/` (script and style inline, no network, as the default MCP Apps CSP requires), and
`zv-mcp` embeds that file with `go:embed`. The built file is committed, so the Go build needs no
Node: rebuild and commit it with every change under `mcp-apps/src`. Checks:
`pnpm --dir mcp-apps run typecheck`, `lint` and `test:unit`.

The orchestrator rate limiter is a pass-through in Studio (`WithRateLimit(0, 0)`), so polling is
safe. Only uploads are limited (2 concurrent).

### Approval

- Elicitation before `generate_short`, `render_full_demo` and `record_round_pov`, showing what will run: player, map,
  rounds and kills of the chosen plays plus every option, or the Full Demo plan summary.
- The same approval guards `render_stream` (layout, how many videos, which are saved moments and
  which are new plays), `render_stream_summary` (play count, of how many, and length), `delete_video`,
  `delete_stream`, `delete_demo` (the file names; one approval for every demo in `ids`), `merge_teams` (both team names and the match count of the one that goes),
  `delete_captures` (bytes freed), `delete_storage_file` (file, kind and size), `retry_video`, `change_video_music`,
  `delete_report`, `delete_play`, `delete_lineup`, `delete_call` and `delete_pattern` (the title), `delete_playlist` (name and round count), `delete_voice` (track count and length), `delete_voice_recording` (start, track count and length), `install_transcriber` (version and download size), `delete_library_series` (format and teams),
  `launch_practice` (map and spot count) and `launch_bot_replay` (round and side).
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
yt-dlp failure reasons from the remote site. Report and play titles and notes are the user's own
text, but an imported stratbook can carry someone else's. In tool output, return them in labelled
fields, strip control characters (notes keep their line breaks) and cap their length. In approval
text a stream, team, report or play name also loses « » and quote characters, so it cannot close its
quote and read as part of the question. Tool descriptions say
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
| `list_matches`, `get_match` | `GET /api/library/matches`, `GET /api/library/matches/{id}` (players with SteamIDs, side keys and their library team). `list_matches` reads every page of the matches, then the demos and series, as the Partidas list does: each match has its `kind` (Tipo de partida, derived: `scrim` for scrim demos, `oficial` for tournament (`hltv`) demos and for any match in a series with an event, `importada` for the rest, such as FACEIT pugs), source, tags, import date and, in a series, its format, event, veto and `map_role` (pick or decider). `kind`, `has_pov` (Con POV: a ready render with a video among the demo's jobs, the same rule as the POV chip), `sort` (`played` or `imported`), `team`, `opponent` and `event` are the controls above the list. Every match also carries `has_voice`, `voice_sync` (`auto`, `manual` or `unsynced`) and `voice_note`, read from `GET /api/library/voice/summary` (no tracks loaded); `has_voice` as a filter is Con voz, and `sync_voice` with `auto` true is the row's Volver a sincronizar voz |
| `list_moments` | `GET /api/jobs/{id}/moments`: the same segment ids as the "Prepara tu Short" list, with scores; the top 10 by default (`limit`), plus `total`; `weapon`, `kinds` (clutch, ace, multi_kill) and `side` (CT, T) filter the list; in a multi-POV job each play carries `player` and `steamid`, and `player` filters by SteamID64 or name |
| `list_series_moments` | `GET /api/jobs/{id}` (same `target_steamid`, different demos) and `GET /api/jobs/{id}/moments` per map; plays of every map tagged with `map_number`, `map` and `job_id`, in play order (map, round, tick) or best first; same filters as `list_moments` |
| `find_player_moments` | `GET /api/library/matches?steamid=`, `POST /api/library/demos/{id}/jobs` and the filtered `GET /api/jobs/{id}/moments` per match; one player's plays between two dates (`from`, `to`) and on one map, up to 10 matches per call; a match still parsing comes back `pending` |
| `list_presets` | `GET /api/presets` |
| `list_videos` | `GET /api/jobs/{id}/renders/{variant}` per preset; MP4 links through the UI origin, since a URL cannot carry the token header |
| `list_all_videos` | `GET /api/jobs` (newest 30 by default, at most 100), then `GET /api/jobs/batch-status` for every job and preset in calls of 100, and `GET /api/stream-jobs` for the same number of streams: finished videos with MP4 links, renders still running or failed, jobs whose parse or capture failed, and rendered stream moments |
| `publish_copy` | `GET .../videos/{name}/publish-assistant`: title, description, tags, `alternative_titles` (up to 4), the recommended copies (the suggested one and up to 4 alternatives) with score and rationale, keywords, the 5 best publish times across the schedule days, and trending terms (or why they are missing) |
| `list_pro_settings` | the `internal/prosettings` catalog (no orchestrator call): id, name, team, resolution and viewmodel FOV for `plan_full_demo` `pro_settings` |
| `search_studio` | `GET /api/library/search?q=` (`limit` per kind, 8 by default and at most 25): plays, lineups, calls, teams, players and matches holding every word, with accents dropped and `&` read as `y`; a map name (mirage, de_nuke...) keeps that map's plays, lineups and matches, a grenade word (humo, flash, molotov...) keeps those lineups |
| `list_teams` | `GET /api/library/teams`; each team says `own` (Mi equipo, one team at most) and its `hltv_rank` |
| `get_team` | `GET /api/library/teams/{id}`: aliases, the roster by date, `own` and `hltv_rank` |
| `team_lineups` | `GET /api/library/teams/{id}/lineups`: the Alineación selector's groups, who played the team's latest match (the UI's default lineup) and everyone else, with first and last dates and match counts; when that match names none of the team's players there is no latest group and the default counts every player |
| `list_players` | `GET /api/library/players` (`q` nick, `team_id`, `limit` 50 by default and at most 200): a nick to a SteamID64 |
| `list_reports`, `get_report` | `GET /api/library/reports` (`team_id`); `GET /api/library/reports/{id}`, `/scout` and `/metrics`: notes (Markdown; `list_reports` shows the first 300 characters and each round list without its refs), the named `round_lists` with each one's `studio_path` (`/replay?r=...&t=<title>`, the button in Informes), the report's own `studio_path` (`/informes/{id}`), the `scout_team` summary over the report's matches and side, each scouted player's metrics, and the `team_attack` read of up to 3 report maps unless the side is CT |
| `list_plays`, `get_play` | `GET /api/library/plays` (`team_id`, `map`, `side`, `kind`, `status`, `tag`, `include_archived` (archived plays are left out by default, as in the Jugadas tab), `limit` 50 by default and at most 200, `offset`; `truncated` when more match); `GET /api/library/plays/{id}`: grenades, positions by callout, status, tags, example refs and missing matches; `get_play` also reads `GET /api/library/patterns?play_id=` for the saved patterns linked to the play |
| `plan_table` | `GET /api/library/plays` (`map`, `side`, `team_id`, `limit` 1000; archived plays left out), grouped as the Tabla: a column per round type of the side in the UI's order, plays by title; `no_team` keeps only plays without a team |
| `list_lineups`, `get_lineup` | `GET /api/library/lineups` (`map`, `side`, `grenade`, `status`, `tag`, `include_archived`, `limit` 50 by default and at most 200, `offset`; `truncated` when more match); `GET /api/library/lineups/{id}`: origin, angles, callouts, technique (`technique_estimated` when read from 8 Hz samples), the `setpos` command and the round it came from |
| `list_pro_lineup_maps` | `GET /api/library/pro-lineups`: maps with pro lineups, their lineup and combo counts and the pro maps read (all time and recent); `fresh` is false when the community server did not answer and this is Studio's last copy |
| `find_pro_lineups` | `GET /api/library/pro-lineups/{map}`, filtered here by `kind`, `side`, `landing` and `from` (case-insensitive parts of the landing and throw callouts), `insta` and `video` (only lineups with a recorded clip), in the served order (most thrown recently first), 20 by default and at most 100, `truncated` when more match: id, slug, title, technique, throws, maps, round share and win, landed of attempts, median clock and top 3 teams |
| `get_pro_lineup` | `GET /api/library/pro-lineups/{map}`, one lineup by id or slug: the `setpos` command, origin, angles, landing, air time, trend, clock buckets, team, player, event and plant site usage, partner lineups (`with`), recent examples (with the thrower's `steamid64`) and the rank among lineups to the same callout; `clip_url` on the lineup and on each recorded example is the throw clip with the Focus inputs HUD on the community server (`/v1/pro-clips/...`), as the video on the lineup page; the radar geometry is left out |
| `pro_grenade_meta` | `GET /api/library/pro-lineups/{map}`, one `side` (required), `kind`, `period` `recent` (the index's recent window) or `all`, 20 by default and at most 100: the side's lineups ranked by throws per 100 side rounds of the period (`rounds_t` or `rounds_ct`), with round share, the top 3 recent teams and the percent of their side rounds, team count and the trend (last 30 days against the 60 before: `new` from no throws to at least 1 per 100 rounds, `rising` or `falling` when the rate moved by half and by 1 throw per 100 rounds, else `steady`) and `small_sample` under 5 throws in the period; `entering` lists the new and rising ids, as the Meta view of Lineups pro |
| `list_pro_combos` | `GET /api/library/pro-lineups/{map}` (`side`, 20 by default and at most 100): combos with their lineup ids and titles, recent rounds, team count, round win, clock and top 3 teams; `clip_url` is a recorded round of the combo, each throw from its thrower with the Focus inputs HUD, once one is recorded |
| `list_pro_matches` | `GET /api/library/pro-matches` (`days` 1 to 120, 7 by default, `team` an HLTV id or part of a name, `event`, `map`, `page` of 30): the HLTV matches the community server analysed (`GET /v1/pro-matches`), newest first, each match with its `event`, `hltv_url` (where its `.dem` files are downloaded), `series` (`format` bo1, bo2, bo3 or bo5 and every map of the series in play order with `pick_by` or `decider`, the unplayed decider too) and `veto` (the HLTV veto in order: `team`, `action` ban, pick or decider, `map`; empty when the server's sidecar has none), each map with its `entry`, `demos` (above 1 when a server restart split it), `library_match_id` once imported, `old_analysis` for an analysis older than the current scan and `radar_may_be_off` for one older than the map's radar calibration; plus the period's teams, events and maps, the followed teams and a `studio_path` that opens Partidas pro with the same filters (`/partidas/pro?dias=&equipo=&evento=&mapa=&pagina=`, the keys the page keeps in its address). A Studio without the server, or a server without the catalog, is an error that says so |
| `list_pro_follows` | `GET /api/library/pro-follows`: followed HLTV teams with when Studio last imported their new maps |
| `get_team_roles` | `GET /api/library/teams/{id}/roles?map=`: the saved sheet (T and CT, up to 5 roles each with name, player, callout and task) and the suggestion, T roles from `InferPlayerRole` and CT ones from where each player sets up in the prep (awp, anchor or rotator) |
| `list_pinned_players` | `GET /api/library/pinned-players`: the Ranking's Fijados in table order (`steamid64`, `name`) |
| `list_calls` | `GET /api/library/calls` (`side`, which also keeps the calls of both sides, `status`, `tag`, `include_archived`, `limit` 100 by default and at most 1000, `offset`): name, side, meaning, status and tags, by name |
| `list_callouts` | `GET /api/library/maps/{map}/callouts`: the places of the map's newest analysed match (calibrated from the overview first), each with its token, CS2's name and the team's name, plus places the team named that the geometry lacks |
| `detect_team_plays` | `GET /api/library/teams/{id}/plays` with the `scout_team` filter; each group's seed round is its `ref`; the 20 most repeated by default, at most 100, `truncated` when more. Each play carries a `studio_path` that opens every round of the group (`refs` in the report; the answer lists the first 5). |
| `list_library_series` | `GET /api/library/series` (`team_id`): format, event, veto notes, teams by slot, veto steps, match ids, `from_import` while a pro import's format, teams and veto stand unedited |
| `team_maps` | `GET /api/library/teams/{id}/maps` (`from`, `to`): record, round win rate per side, veto counts and preference per map over every match and series of the team; the side rounds read the newest 200 analyses and `sides_unread` counts the older matches left out of them; plus `GET /api/library/teams/{id}/map-counts` with the scope keys `opponent_id`, `lineup`, `max_matches`, `last_days`, `opponent_top`, `whose`: `scope_matches` per map, the "N partidas" of the map selector (0 greys the map out) |
| `simulate_veto` | `GET /api/library/teams/{id}/veto` (`opponent_id`, `format`, `first`, repeatable `pool` and `done`): steps with their basis, the maps played and each map's edge |
| `faceit_player` | `GET /api/faceit/players` (`nickname`, a nick or profile URL): id, level, ELO, country, SteamID64 |
| `faceit_matches` | `GET /api/faceit/players/{id}/matches` (`limit` 10 by default and at most 20): map, result, score, K/D/A, ADR, match room link |
| `list_faceit_follows` | `GET /api/faceit/followed`: followed and seeded players of Jugadores FACEIT |
| `get_player` | `GET /api/library/players/{steamid}`: nicks and teams by date, linked FACEIT account |
| `scout_team` | `GET /api/library/teams/{id}/scout` (`map`, `side`, `from`, `to`, repeatable `opponent_id`, `match_id` and `lineup`), summarized: maps, buys, sites, round shapes, openings, executes, AWP, players. `lineup` (SteamID64s, at most 50) keeps the rounds in which every player of the team is listed and drops matches with none, as the Alineación selector does; the team tools below take it too, and the team scope keys of `team_rounds` (`max_matches`, `last_days`, `opponent_top`, `whose`, `buy`, `opponent_buy`, `awp`, `opponent_awp`, `advantage`, `round_from`, `round_to`, and the Replay filters listed under `team_rounds`). Each buy, site, round shape (T and CT) and execute row carries a `studio_path` to its rounds: the report lists every round of each bucket in `refs`, and `rounds_linked` says when a path opens fewer rounds than the row counts. |
| `player_metrics` | `GET /api/library/players/{steamid}/metrics`; rates with the `get_baseline` definitions |
| `get_baseline` | `GET /api/library/baseline` (`scope` library or pro, `map`, `min_rounds`) |
| `find_rounds` | with `team_id`, `GET /api/library/teams/{id}/rounds` with the whole team scope (`lineup`, `max_matches`, `last_days`, `opponent_top`, `whose`, repeatable `buy` and `opponent_buy`, `awp`, `opponent_awp`, `advantage`, `round_from`, `round_to`, and the Replay filters listed under `team_rounds`), the rounds the team tools count; with `steamid`, `GET /api/library/matches?steamid=` then `GET /api/library/matches/{id}/tactical` per match. `play_id` keeps the rounds of a Libreta play (`GET /api/library/plays/{id}`, its examples) or of a detected play (its ref from `detect_team_plays`: every round of the group, with `play_rounds` counting them). Reads at most 50 matches newest first: `match_cap` and `more_matches` say when older ones were left out. Round refs `match_id/round` with both buys, the site and the round shape, filtered by `result`. `studio_path` opens the rounds it lists (at most 200) in the 2D Replay as one playlist (`/replay?r=...`), with the player in `jugador` for a `steamid`. |
| `team_bombsites` | `GET /api/library/teams/{id}/bombsites` (`map`, `side`, `from`, `to`, `opponent_id`, `match_id`, `lineup`); per map and site, the CT hold and retake rates and the T attack, plant and post-plant rates. Each CT and T row carries a `studio_path` to `/replay?r=...` with the site in `zona` (the report keeps every hit or attacked round in `refs`; `refs` in the answer still lists the first 5), and `rounds_linked` when it opens fewer rounds than the row counts. |
| `team_side_stats` | `GET /api/library/teams/{id}/side-stats` (the HTTP read also takes `group=none`, which skips the comparison group and returns only the team's figures and `players[]`, as Preparar's Jugadores block reads it; the tool does not expose it) (`map`, `side`, `from`, `to`, `opponent_id`, `match_id`, `lineup`, repeatable `compare_team`, and the team scope keys: repeatable `buy` and `opponent_buy`, `max_matches`, `opponent_top`, `whose`, `awp`, `opponent_awp`, `advantage`, `round_from`, `round_to`, and the Replay filters listed under `team_rounds`): the Estadísticas tab. `overall`, `ct` and `t` hold maps, rounds and pistols won, opening kills, 5v4 and 4v5 won, trades, utility damage and flash assists per round and rounds played, each with `group_avg`, the plain mean of the comparison teams (`compare_to`, or every other library team) over the same map, side, dates and buys, counting a team only with 20 rounds behind the figure (`group_teams` per figure); with `side` set, `overall` and `players` honour it while the `ct` and `t` blocks (and their group figures) ignore it, so the other side keeps real numbers; the group reads at most 20 matches per team (`group_matches`, `group_capped`) and unknown `compare_to` ids come back in `compare_skipped`; `players[]` carries the library role (awp, entry, support, rifler), with one `map` its `map_roles` (the role per side as the Ranking reads it: `role`, `map_role` and a `label` such as Lurk de B, Ancla A or Todoterreno), Rating de ClipHub (`rating_formula`), K/D, ADR, KPR and DPR; `match_list` the matches behind the figures; `awp_estimated` counts matches whose AWP filter was estimated; `studio_path` opens the tab with its filters (map, side, dates, rival, lineup, buys; other buy lists are refused) |
| `team_playbook` | `GET /api/library/teams/{id}/playbook` (`map` and `side` required, `from`, `to`, `opponent_id`, `match_id`, `lineup`, `with`, `without`); category, site, zone and tactic shares with win rates over the rounds the combination keeps, the tempo median, and up to 5 round refs per row. Tactic keys read `kind:value`: `category`, `site`, `buy`, `opponent_buy`, `opening` (callout), `opening_result` (`won`, `lost`), `awp` (callout at 12 s), `zone`, `utility` (`smoke@Callout`, `molotov@Callout`). Each category, site, zone and tactic row carries a `studio_path` that opens all its kept rounds (with the site or zone in `zona`); `refs` lists the first 5. |
| `prepare_team` | `GET /api/library/teams/{id}/scout` and `GET /api/library/teams/{id}/prep` with the same `map` (required), `from`, `to`, `opponent_id`, `match_id` and `lineup`: the team's Preparar tab, which joined Equipos > Scouting and Informes > Prep, plus `GET /api/library/teams/{id}/attack` (the `team_attack` read). `studio_path` opens it on the same lineup (`lineup=all` when none was given). `side` is the bar's side: the scouting and `prep.warnings` count it, and with it `prep.first_death` reads as in `team_prep`. Preparar's Jugadores block (a card per player) comes from `team_side_stats` with the same scope: map, side, rival, lineup and dates; with no map it shows every map. `prepare_team` does not return it, so an assistant calls `team_side_stats` alongside `prepare_team` for the players. Its scouting rows carry a `studio_path` like `scout_team`'s. |
| `team_attack` | `GET /api/library/teams/{id}/attack` (`map` required, T side only, `from`, `to`, `opponent_id`, `match_id`, `lineup`); sites (A, B, both before the plant, none) and Bombsite Tempo (rush, set strat, mid round, no site) crossed with buy matchups, preferred defaults (the A, mid and B lean by callout and floor at 25 s, from each player's last position) with usage and win %, executes per buy over the rounds with positions (route, rush and plant share, contact second or null) and each T player's top 3 spots and pace per tramo; warnings for a sample 21+ days old, few rounds, a roster change among the kept rounds, or a read capped at the newest matches. Each row gives its rounds and up to 5 example refs; `all_rounds`, `default_rounds`, `execute_rounds`, the top row of each list and each player's top spot carry a `studio_path` to `/replay?r=...` with `jugador`, `tramo` and `zona`, and `rounds_linked` when it was cut at 200 rounds |
| `show_zones` | `GET /api/library/maps/{map}/callouts?geometry=true`; draws the map outline with the asked zones (drawing in `_meta`), and the text names the zones the map lacks |
| `team_prep` | `GET /api/library/teams/{id}/prep` (`map` required, both sides, `from`, `to`, `opponent_id`, `match_id`, `lineup`); buy matchups, tempo per buy, and each CT player's library role, CT `map_role` (as the Ranking reads it) and callouts per tramo of the round clock (1:55 to 1:00, 1:00 to 0:40, 0:40 to the end: the Replay's quick tramos), with up to 5 round refs per row. `warnings` comes from `GET /api/library/teams/{id}/warnings` (the same scope, with `side` when given), the line on top of Preparar: `stale` (the team's newest analysed match, whatever the filters, over 14 calendar days old), `lineup` (N of the scope's M matches, read before the lineup filter, whose five hold a player outside the lineup or, without one, differ from the usual five) and `small_sample` (under 30 rounds in the scope). With `side` (T or CT), `t_from`, `t_to` (seconds after freeze end; 1:55 to 1:40 by default) and `movement` (`static` under 20 u/s, the default, `slow`, `fast` over 140 u/s) it also reads `GET /api/library/teams/{id}/first-death`: `first_death` gives each player's library role, `map_role` on that side, rounds with positions and 5 zones with most seconds per such round until the round's first death, as Posiciones hasta la primera muerte. Each matchup, CT spot (with `jugador`, `tramo` and `zona`) and `first_death` zone carries a `studio_path`, and `rounds_linked` when it opens fewer rounds than the row counts. The tempo rows list no rounds and have no path. |
| `team_rounds` | `GET /api/library/teams/{id}/rounds` with the team scope (`map`, `side`, `from`, `to`, `opponent_id`, `match_id`, `lineup`, `max_matches`, `opponent_top`, `whose`, repeatable `buy`, `opponent_buy`, `advantage`, `awp`, `opponent_awp`, `round_from`, `round_to`, repeatable `site`, `outcome`, `post_plant` (sent as `tag=postplant`), `phase`, `second_round`, repeatable `end_reason`, `weapon`, `opponent_weapon`, `equip_from`, `equip_to`, `opponent_equip_from`, `opponent_equip_to` and `tactic`, a Libreta play id); the rounds the team Replay's filter bar keeps per match, newest first, how many matches the opponent level could not rate (`opponent_unranked`, by each rival's rank today, or the catalog's rank on the day for a match imported from Partidas pro), `awp_estimated` (older analyses whose AWP came from kills and hits), `studio_path` (`/equipos/{id}?tab=replay`) and a `playlist_path` of the rounds. `last_days` becomes `from`. Every team tool takes the same scope keys. The team views read every match the team played (the list is paged, not capped) and keep the newest 50 the scope selects; the rest come back as `over_match_limit` skips, which the Replay shows as "N fuera por el tope de 50". One view opens at most 200 analyses, after the lineup and a single map or date range are applied in the library list; the rest come back as `over_read_limit` skips, which the Replay shows as "N sin leer por el tope de 200" |
| `team_grenades` | `GET /api/library/teams/{id}/grenades` (`map` required, repeatable `kind`, the team scope like `team_rounds`, and `opponents=1` for the utility of the team's rivals in the same rounds; the scope's buys and AWP still read the scouted team); `players` (SteamID64s) is applied here. `unranked` and `awp_estimated` as in `team_rounds`. `rounds` is the scope's rounds, the denominator of every count, and `matching_rounds` the rounds with a kept grenade (without one with `invert`). Per kind the count, sides, median second, top throwers and its top 5 landing callouts (`land_place`), each with a `studio_path` that marks the zone in the Replay; `clock` counts throws per 10 s of the round clock by player, with a last `Tras la plant` bin for throws after the round's `plant_seconds`; `spots` are the repeated throws (grouped by the orchestrator oldest match first: same kind, origin within 100 units, landing within 160; the key names the oldest throw, so newer imports keep it; callouts name a place only within 600 units) with rounds, wins, throwers and the top 3 facts of what the team did in the 20 s after (the round's `category@site`, the first duel's callout, callouts a player held 3 s). `spot` reads one in full: every fact, its rounds, a `setpos` of the throw nearest its centre and a `studio_path` that superposes its rounds (`s=1`, at most 200) from 3 s before the throw at the demo's tick rate, titled with the team's callout names. `invert` lists `inverted_rounds`. Then compact rows with the round ref, rounded world positions, `spot` and the `throw_tick` `save_lineup` reads, 30 by default and at most 100 |
| `player_positions` | `GET /api/library/players/{steamid}/positions` (`map`, `from`, `to`); per map and side the site zones, the top callouts with a `studio_path` each, the CT role (anchor or rotator), `map_role` (the side's role as the Ranking reads it, with its `label`) and the AWP share |
| `leaderboard` | `GET /api/library/leaderboard`, the Ranking page (`metric` required, including `rating` (the Rating de ClipHub: `tacticalplan.RatingFormula`) and `kd`; `scope`, `map`, `side`, `role` (`awp`, `entry`, `support`, `rifler` on any map, library scope only; `a_anchor` to `pack` need `map` and set the side), `map_role` (a callout held on the map and side, or `allround`; needs both, except `allround` on the library scope, which needs only `side` and then reads every map), `from`, `to`, `source`, `min_rounds`, `limit` 20 by default and at most 100). `map_roles` lists the positions players hold on the map and side. With neither `from` nor `to` the library scope reads the last 90 days, like the page (pass `from` 2000-01-01 for every date). Each row carries every metric, its `library_role`, its roles per side on the map, a `breakdown` (percent of rounds per role and per position, per side) and `refs` (match_id/round) to open in the Replay. `from`, `to` and `source` are for scope `library`; scope `pro` takes `side`, `role`, `map_role`, `days` (1 to 365, 90 by default), `top` (rival ranked 5, 15, 30 or 50 in HLTV), `team_top` (the player's own team, same levels) and `event` (named in the answer's `events`) and asks the community server (`GET /v1/pro-players`) for the pro players of its pro maps (HLTV is only the source of the team ranks): the rows carry a `team`, and their `refs` are the library's own rounds of that player on the map, side and role, since those pro maps are not in the library.; each row carries `pinned` from `GET /api/library/pinned-players` (omitted when Studio cannot list the pins) |
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
| `get_round` | `GET /api/library/matches/{id}/tactical` (the document without positions; the round endpoint carries megabytes of frames) and `GET .../metrics` for the round's win probability timeline; buys, patterns with reasons, bomb, timeouts (the CT or T Timeout bands of the Replay's timeline), player lines, kill feed, seconds from freeze end |
| `match_metrics` | `GET /api/library/matches/{id}/metrics` and `/tactical` for slot names; every player's metrics and, per round, the CT win chance at freeze end and the kill that moved it most |
| `match_tendencies` | `GET /api/library/matches/{id}/tactical/aggregate` (`team` required, `side`, repeatable `buy`, `outcome`, `round_from`, `round_to`); the tables of the match's Métricas tab. The rounds behind a figure come from `replay_rounds` with the same buy, site or shape |
| `integrity_dossier` | `GET /api/jobs/{id}/anticheat/dossier/{steamid}`: Markdown body, report channels and the reporting policy; a 409 says to wait for `check_integrity` |
| `check_integrity` | `GET /api/jobs/{id}/anticheat`; a 409 (never screened) starts it with `POST`; players most suspicious first with their top 3 metrics and the report's limitations |
| `replay_rounds` | `GET /api/library/matches/{id}/tactical`, filtered with `tacticalplan.Filter` exactly as the Replay's scope bar does; the kept rounds with winner, buys, site and shapes, the kept and total counts, and `studio_path`, which opens the Replay on that scope and form (`forma=repeticion`, `calor`, `recorridos` or `granadas`). `focus` (SteamID64s) becomes `subject=<slot>`: the players in the foreground, which Mapa de calor, Recorridos and Granadas count alone; it never drops a round. `awp` and `opponent_awp` (`with`, `without`) and repeatable `advantage` (`4v5`) filter as on the team Replay |
| `replay_overlay` | `GET /api/library/matches/{id}/tactical/overlay` with the scope and `focus` of `replay_rounds`; the rounds superposed from the end of freeze time, whose paths each draws, the total the scope keeps and the cap, and `studio_path` (`forma=recorridos`) |
| `match_heatmap` | `GET /api/library/matches/{id}/tactical/heatmap` with the scope and `focus` of `replay_rounds` plus `kind` (`position`, read at 50 seeded instants per round inside the window as cs2lens does, `death`, `kill`, `utility`, `timing`), `kill_at` (`killer` or `victim`), repeatable `grenade`, `t_from`, `t_to` and `t_from_plant` from `clock_from`, `clock_to` and `from_plant`; the hottest 64-unit cells (15 by default, at most 50) with their callout (`tacticalplan.NearestCallout` at the cell's median altitude), count, share of the side's total, rounds, median second and example round refs, the cells summed by callout, each side's totals and `studio_path` (`forma=calor`, `cuenta`, `baja`, `granada`, `tramo`) |
| `team_heatmap` | `GET /api/library/teams/{id}/heatmap` (`map`, `side`, `from`, `to`, `opponent_id`, `match_id`, the team scope of `team_rounds` with repeatable `buy` and `opponent_buy`, repeatable `player` SteamID64s (with `by_player`, one more request per player, at most 5, answered in `by_player`; `area` keeps only the cells whose centre lies in a zone and its floor), and the heatmap's own query of `match_heatmap`); the same answer over the team's matches on one map, the most played when `map` is empty, and `studio_path` to the team's Replay on that map with the full scope (`tab=replay`, `forma=calor`, `jugador`); a buy or advantage list the bar cannot hold is an error |
| `find_player_pattern` | `GET /api/library/teams/{id}/pattern` (`map`, `from`, `to`, `opponent_id`, `match_id`, `lineup`, the team scope of `team_rounds` (`max_matches`, `opponent_top`, `whose`, repeatable `buy` and `opponent_buy`, `awp`, `opponent_awp`, `advantage`, `round_from`, `round_to`, and the Replay filters listed under `team_rounds`), and `pattern`: the JSON of one `player` block with the side, players, `min_players` (that many in the zone at the same moment), `rival`, `area` as a world rectangle with an optional `level`, a `polygon` of world points with an optional `level`, or `callouts` (place tokens of `list_callouts`), the clock window and `invert`); the matched rounds by match with the round clock at which the player entered the zone (`+Ns tras el plant` past the plant), the team's rounds on the side, the win rate, the last 30 days, the trend (`up`, `down`, `flat`, empty under 10 rounds on either side of the cut) and months with gaps as zero, `missing_positions` (matches whose rounds a player block leaves out of scope), `unreadable_matches`, `grenades_without_throw`, `replay_path` (the playlist) and `studio_path` (`tab=replay`, `forma=patrones`, `patron`) |
| `find_pattern_rounds` | the same request with up to 6 blocks (`player`, `utility` with `grenade` and `from_throw`, `plant`), each with its own count; a round is kept when every block holds |
| `list_patterns` | `GET /api/library/patterns` (`map`, `team_id`, `category`, `play_id`, `stats=1`): the saved patterns of the map with `category`, `play_id`, `rounds`, `matched`, `won`, `win_rate`, `recent_matched` and `trend`, each with its own scope, over the team's matches with `team_id` (its patterns and the library-wide ones), or without it from each pattern's stored detection over the newest 200 matches of its map (rows in `library_pattern_matches`, one per pattern, match and team, refreshed after a demo's analysis and a pattern edit; a missing or stale row is detected on read) |
| `match_patterns` | `GET /api/library/matches/{id}/patterns` (`round`): the saved patterns of the match's map that hold in its rounds from the stored detection, one entry per pattern and side with its rounds, and `total`, every saved pattern that applies to the match (library-wide or of its two teams); a team's pattern counts only from that team's side, as the Replay's Patrones panel shows the current round |
| `pattern_rounds` | `GET /api/library/patterns/{id}` then `GET /api/library/patterns/{id}/rounds` (`team_id`); a saved pattern over a team, its own, or the newest 200 analysed matches of its map from both teams' side when it has none (`capped_at`), skipping teams without the named players |
| `round_lineups` | `GET /api/library/matches/{id}/tactical/rounds/{round}/lineups`; the round's grenades with thrower, landing callout and the catalog lineup each matches (Comparar granadas in the round panel) |
| `replay_playlist` | no request: checks the round refs and returns the `/replay?r=<match_id>:<round>` path that opens them as one passing playlist; nothing is stored (`create_playlist` keeps one) |
| `list_playlists`, `get_playlist` | `GET /api/library/playlists` (`q` over name, author and folder, `map`, `folder`, `no_folder`, `favorite`, `limit` 50 by default and at most 1000; `truncated` when more match), newest edit first; `GET /api/library/playlists/{id}`: the rounds in play order with their comment, map and `notes` (tick, text and stroke count of each note or drawing left on the round in the Replay), `missing` when the match is not in this library, and `studio_path` `/replay?p=<id>`, which opens the Replay with the playlist's panel |
| `voice_status` | `GET /api/library/matches/{id}/voice`: tracks (label, player, length), sync (`state` unsynced, auto or manual, `method`, `offset_ms`, `auto_offset_ms`, confidence, note and the Replay's label), transcript and transcriber states, and `studio_path` to round 1 |
| `get_round_comms`, `match_talk_time` | `GET /api/library/matches/{id}/voice/comms` (`round` for one round): talk time of each track by phase (freezetime, midround, post_plant, dead for a track tied to a player, after_round) and in total; `get_round_comms` adds the round's phrases (at most 300) with speaker, round clock, phase, tick and a `/replay?r=<match_id>:<round>:<tick>` path |
| `search_comms` | `GET /api/library/voice/search` (`q` as a phrase, case-insensitive, `team_id`, `match_id`, `from` and `to` dates, `limit` 50 by default and at most 200, `period` as `phase`: Studio places the phrases and keeps one round phase before the limit); each hit has its match, map, date, teams, round clock and Replay path |
| `player_setpos` | `GET /api/library/matches/{id}/tactical` for the slot and `GET /api/library/matches/{id}/tactical/rounds/{round}` for the samples; the `setpos x y z;setang pitch yaw 0` line (z at the feet, as in the lineups) of the sample nearest to `second` after the round went live, as S or a right-click on a player copies it |
| `export_replay_gif` | `GET /api/library/matches/{id}/tactical`, `GET .../tactical/rounds/{round}`, `GET /api/maps/{map}/radar` (a 404 leaves a plain plate) and `GET /api/library/annotations?match_id&round` unless `skip_drawings`; writes a new looping `.gif` (10 fps, lowered down to 1 for a long tramo or a big size so the frames stay in memory; 128 to 1024 px) of the tramo from `from_second` to `to_second` after the round went live or, with `from_plant`, after the plant. Exportar GIF records what the screen shows (superposed rounds, framing, the drawing in progress); this tool draws a fixed view of one round |
| `export_replay_video` | The reads of `export_replay_gif` for each round in `round` or `rounds` (played one after another with the same tramo), then `POST /api/library/matches/{id}/replay-video` (a `spec` part with `fps`, `width`, `height`, `speed`, `include_voice` and one segment per round with its `from_tick`, `to_tick` and frame count, then one JPEG `frame` part per frame; FFmpeg encodes H.264 as they arrive) and `GET /api/library/replay-videos/{video_id}`, which hands the MP4 over once; writes a new `.mp4` (24, 30 or 60 fps, 256 to 1024 px, speed 0.25 to 4, 120 s at most), drawn in full colour. `layers` picks players, utility, bomb, drawings and clock. With `include_voice` the match's team voice tracks (one FFmpeg input each) are cut to each round's tramo with the saved sync offset and mixed into AAC; `freeze_only` keeps only the freeze-time comms (`voice_to_tick` per segment). The Replay sends the offset it plays, an unsaved adjustment included, as `offset_ms`; `no_voice`, `voice_unsynced` and `voice_outside` say why it cannot. Exportar MP4 in the Replay sends what the screen shows through the same route |
| `real_map_status` | `GET /api/maps/{map}/scene/summary`, which reads the map's render scene from the CS2 install on first use; triangles, textures and whether it has baked light, sun, fog, sky and post processing (exposure, tonemap and colour LUT), plus how to open the Mapa real floor plan in the Replay and full screen |

Team scope rules (`internal/tacticalplan`, shared by the Replay, the playbook, the grenades and every team tool):

- Buy: each side's equipment value is sampled once a round, at freeze end plus a fixed delay; per player it is the side's value over the players it fielded. Eco is at most $1,000 a player, full buy at least $4,500 (CT) or $4,000 (T), force a half buy after a lost round with at most $400 a player in the bank, semi any other half buy, pistol the first round of each regulation half. The custom range (`equip_from`, `equip_to`) reads the same per-player value.
- `weapon` `scout`: a player of the side killed or hit an enemy with the SSG 08; `no_scout`: none did.
- `weapon` `hero_ak`: one player of the side killed or hit with an AK-47 while every teammate carried under $1,000 of equipment.
- `weapon` `hero_awp`: one player of the side held an AWP (from kills and hits before analysis version 11) while every teammate carried under $1,000.
- `second_round`: the round after a regulation half's pistol round. `end_reason`: `elimination`, `bomb_exploded`, `bomb_defused` or `time_expired`; pair it with `outcome` for a win reason.
- `tactic`: the example rounds of a Libreta play of the same team (another team's play is a 400); matches outside them drop out, and a deleted play keeps no round.

Write:

| Tool | Orchestrator calls | Approval |
| --- | --- | --- |
| `import_demo` | `POST /api/library/demos` (multipart `tag` fields, then `demo`; the server applies only tags sent before the file, and a duplicate keeps its own tags) | no |
| `update_demo` | `PATCH /api/library/demos/{id}` with `source` (not `shared`) and/or the whole `tags` set, or `add_tags` and `remove_tags` for single tags (the Etiqueta button of a Partidas row) | no |
| `reanalyze_demo` | `POST /api/library/demos/{id}/analyze`; a demo already queued or parsing is left alone, a shared match answers 409 | no |
| `create_job` | `POST /api/library/demos/{id}/jobs` with `target_steamid`; with `reparse` on a reused job, also `POST /api/jobs/{id}/parse` | no |
| `create_multi_pov_job` | `POST /api/library/demos/{id}/jobs` with `target_steamids` (2 to 5 teammates of one team, checked against the analysed match); the first is the anchor for recap, deaths and team comms. The job never reuses a single-player job and Full Demo refuses it | no |
| `create_series_jobs` | per map: `GET /api/library/demos/{id}` and `GET /api/library/matches/{id}` to find the player by SteamID in every map, then `POST /api/library/demos/{id}/jobs`; nothing is created if one map lacks the player. Each job then takes `generate_short` (a Short per map) or `plan_full_demo` and `render_full_demo` (a full demo per map), like the series page in Shorts and in Demo completa | no |
| `generate_short` | `POST /api/jobs/{id}/generate` with `preset` (default from `list_presets`) and `segment_ids` from the plan; one job (one map) per call, so a series gets one Short per map. A multi-POV job mixes players in one capture: the server refuses plays of two players less than half a second apart, a death reel and an empty pick. `music` goes as `{key, volume, game_volume}` when `music_volume` or `game_volume` is set (percents in the tool, gains on the wire, full music volume left unset like the UI); `intro_text`/`outro_text` and the affiliate banner (`keydrop_family`, `keydrop_style`, `keydrop_code`, a 0 to 4 s window by default) go in `edit` | yes |
| `render_full_demo` | `GET /full-demo/plan` defaults, `POST /full-demo/plan`, then `POST /generate` with preset `gameplay-pov-60` and the approval (`tuiclient.GenerateFullDemo`) | yes |
| `record_round_pov` | the Replay's "Grabar su POV de esta ronda" in one call: `GET /api/library/matches/{id}` (round and player checked), `POST /api/library/demos/{id}/jobs`, polls `?view=status` until `parsed` (`running` when `wait_seconds` ends first: call again), `GET`/`POST /full-demo/plan` with `rounds` of that one round and no effects between rounds, then `POST /generate` like `render_full_demo` (same approval, own code; variant `gameplay-pov-60`) | yes |
| `wait_for_job` | polls a job; takes the render `variant`; shows the progress view | no |
| `wait_for_demo` | polls a library demo until `ready` or `failed`; shows the progress view | no |
| `plan_full_demo` | defaults plus the user's HUD and voices, `POST /full-demo/plan`; returns duration and `plan_hash`. `pro_settings` resolves a catalog pro into the capture (or `none`); `hud_portrait` uploads a PNG or JPG to `POST /api/full-demo/overlay-images` (focus HUD only); `intro_bumper`, `sponsor_bumper` and `outro_bumper` upload an MP4 to `POST /api/editor/assets` with the UI's local-file provenance. Like every option, they start from the defaults, not from a plan saved in the UI | no (saves the plan the UI also shows) |
| `import_stream` | `POST /api/stream-jobs`: JSON `source_url` (Twitch, Kick or YouTube, checked with `vodfetch.ValidateSource` first), `title`, `source_kind`; or multipart `video` for a local `.mp4` | no |
| `find_stream_plays` | `GET /api/stream-jobs/{id}/plays`, `POST` when idle, with `rescan`, or when `voice` or `voice_sensitivity` differ from the finished scan's `voice` block (the POST body is `{voice, voice_sensitivity}`, the Studio's "Picos de voz" switch and slider). Plays ranked by score, 20 by default and never more than `limit` except `marked` ones, which are always listed. `voice_sensitivity` is bounded 1 to 5 in the tool schema and clamped before it is compared with the last scan. Each play has `kind` (`kill` or `voice_peak`), `voice_peak` (a kill play with a shout, +15 score), `marked` (a shout far over the streamer's own level for at least 1 s; at most 3 per hour of video) and a `note`. Kill plays keep ids `play-NNN`; voice-only plays are `voice-NNN`, at most 6 per hour of video (the loudest), and score below a one-kill play with a shout. The mic is the audio track titled "mic", else the second track, else the only (mixed) track; a track with no voice falls back to the first. The result has a `voice` summary (`off`, `done: N voice peaks at sensitivity S` with a note when it ran on the game mix, `no_audio`, `no_voice`, `failed`) | no |
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
| `delete_demo` | `GET /api/library/demos/{id}` for the summary of each demo (`id`, or up to 50 `ids` as Borrar on the partidas chosen in Partidas), then `DELETE /api/library/demos/{id}` for each, in order (demo, match, analysis and corrections, the match's team voice and the in-game voice read from the demo; videos and jobs stay) | yes |
| `update_team` | `PATCH /api/library/teams/{id}` (`name`, `aliases` replaces the set, so the list starts from `get_team`: a dropped clan name makes a new team on the next import); `own` and `hltv_rank` go to `PUT /api/library/teams/{id}/profile`, as Mi equipo and Ranking HLTV on the team page | no |
| `assign_match_team` | `PUT /api/library/matches/{id}/teams/{key}` (`team_id`, `team_name` found or created, or cleared; `remember_alias`); `get_match` lists the side keys | no |
| `create_report`, `update_report` | `POST /api/library/reports` (`title`, `team_id`, `match_ids`, `side`, `notes` in Markdown up to 20000 characters with `[text](/studio-path)` links, rendered in Informes and the exported report, `round_lists`: up to 30 named lists of up to 200 round refs, each `{title, rounds: [{match_id, round}]}`, every round of one of `match_ids`); `PATCH /api/library/reports/{id}` (the team cannot change; `notes` and `round_lists` are replaced whole, and `match_ids` alone drops the listed rounds of the matches taken out). A share package of a report is version 2 since it carries the lists and the long notes | no |
| `open_in_studio` | no orchestrator request: only app page paths (no `/api/`, `/_next/` or file paths). Writes `open-request.json` (`{"version":2,"id":<random>,"path":...}`, through a temp file per call) in the Studio profile; the desktop window takes it (a regular file of at most 64 KB), checks the path again, routes there in the app over IPC (a full load only when the window is not on Studio yet), comes to the front and writes `open-ack-<id>.json` (`ok` and a `reason`) once the page opened or failed. Without an ack in 8 s it withdraws its own request and returns `opened: false` with the full `http://127.0.0.1:<port>` address | no |
| `delete_report` | `GET` then `DELETE /api/library/reports/{id}` | yes |
| `save_round_as_play` | `GET /api/library/matches/{id}/tactical/rounds/{round}/play?team_key=` (the draft), then `POST /api/library/plays` with the title and notes | no |
| `save_detected_play` | `GET /api/library/teams/{id}/plays` again with the same filter, the group whose seed matches `play_ref`, then `POST /api/library/plays` (origin `detected`, positions second from the thresholds) | no |
| `create_play` | `POST /api/library/plays` with origin `manual` (title, map, side, kind, site, team, notes, status, tags), as Nueva jugada does | no |
| `save_playbook_combo` | `GET /api/library/teams/{id}/playbook` with the same filter, `with` and `without`, then `POST /api/library/plays` (origin `playbook`, the kept rounds' most common pattern and site, the combination in the notes, up to 50 kept rounds as examples) | no |
| `update_play` | `PATCH /api/library/plays/{id}` (`title`, `kind`, `site`, `notes`, `team_id` or null, `status`, `tags` as the whole set, `roles` as the whole list) | no |
| `set_team_roles` | `PUT /api/library/teams/{id}/roles` (`map`, `t`, `ct`): replaces the map's sheet; a merge moves it with the team | no |
| `set_pinned_players` | `PUT /api/library/pinned-players` (`players`, up to 100 with a 17 digit `steamid64` and a `name`): replaces the whole list the Ranking keeps on top of its table; a repeated player keeps its first place | no |
| `delete_play` | `GET` then `DELETE /api/library/plays/{id}` | yes |
| `export_plays` | `GET /api/library/plays/export` (`team_id`, `map`, `side`, `kind`, `include_archived`), written to a new `.json` file on a local drive; an existing file is never overwritten | no |
| `import_plays` | reads a local `.json` file (at most 8 MiB, valid JSON), `POST /api/library/plays/import`; imported, duplicate and rejected counts, with at most 50 plays and 50 rejections listed, and the counts of each other section a whole Libreta file holds (lineups, role sheets, calls, callout names; a sheet or name the Libreta already has is a duplicate) | no |
| `export_libreta` | `GET /api/library/libreta/export` (plays, lineups, role sheets by team name, calls and callout names, archived ones too), written to a new `.json` file on a local drive | no |
| `save_call`, `update_call` | `POST /api/library/calls`; `PATCH /api/library/calls/{id}` (`name`, `side` T, CT or both, `meaning`, `status`, `tags`) | no |
| `delete_call` | `GET` then `DELETE /api/library/calls/{id}` | yes |
| `create_playlist`, `update_playlist` | `POST /api/library/playlists` (`name`, `folder`, `author`, `favorite`, `rounds` as `{match_id, round, comment}` in play order; the server reads each round's map and demo hash); `PATCH /api/library/playlists/{id}` (the same fields; `rounds` replaces the list, which reorders, removes and comments rounds, and a round it already holds keeps its notes and drawings, and its comment when the round gives none; a star alone keeps the edit time; a round list edited after another change answers 409 `playlist_changed`) | no |
| `update_playlist_round` | `GET /api/library/playlists/{id}` when notes are given, then `PATCH /api/library/playlists/{id}/rounds/{match_id}/{round}` (`comment`, `notes` as the full list of `{id, tick, text, strokes}` where a kept id keeps that note's drawing, `position`), as the playlist panel in the Replay does | no |
| `add_playlist_rounds` | `POST /api/library/playlists/{id}/rounds` (`rounds`, `position` with 0 the start and the end without it); rounds the playlist holds keep their place and comment, as B does in the Replay | no |
| `delete_playlist` | `GET` then `DELETE /api/library/playlists/{id}`; its matches stay | yes |
| `export_playlists` | `GET /api/library/playlists/export` (repeated `id`, or the `list_playlists` filters), written to a new `.json` file on a local drive: format `cliphub-playlists` v1 with the playlists and a `matches` section naming each round's demo (hash, file name, teams, date), as Exportar al equipo does | no |
| `import_playlists` | reads a local `.json` file (at most 8 MiB, valid JSON), `POST /api/library/playlists/import`; imported, duplicate and rejected counts, at most 50 playlists listed, and `missing_matches`: the demos of rounds this library lacks. Those rounds are kept and resolve by demo hash once the demo is imported | no |
| `save_pattern` | `POST /api/library/patterns` (`name`, `map`, `team_id` or none for the whole library, `category`, `play_id` of a Libreta play, `pattern` with its side and blocks, `scope` with `matches`, `buy`, `opponent_buy`, where `half` is semi or force), as Guardar patrón does | no |
| `update_pattern` | `GET /api/library/patterns/{id}` then `PATCH /api/library/patterns/{id}` (`name`, `map`, `team_id` or `clear_team`, `category`, `play_id` or `clear_play`, `pattern` with the side and the full block list, `scope`), as Editar in Patrones guardados does; an edit of the map, blocks or scope queues the pattern's detection again | no |
| `delete_pattern` | `GET` then `DELETE /api/library/patterns/{id}` | yes |
| `import_voice` | checks a local audio file (at most 2 GiB), multipart `POST /api/library/matches/{id}/voice` with the `audio` part (upload timeout); Studio stores Ogg Opus tracks under the profile, one per channel past two, and the first import estimates the sync (in-game demo voice, then kills, then round 1 start) and logs a recording that does not fit the demo; every channel is split in one FFmpeg pass with `-protocol_whitelist file,pipe`; returns the voice and per-file results | no |
| `sync_voice` | `PATCH /api/library/matches/{id}/voice` with `offset_ms` (manual, as the Sincronizar bar's confirm) or `auto` true (estimate again) | no |
| `set_voice_track` | `PATCH /api/library/matches/{id}/voice/tracks/{track}` (`label`, `steamid64` of a match player or empty) | no |
| `install_transcriber`, `transcribe_voice` | `GET /api/library/voice/transcriber`, then after the approval `POST /api/library/voice/transcriber/install` (pinned whisper.cpp build and model, hash checked, in the background; Studio runs `whisper-cli --help` before it reports installed, and a build that cannot start is `failed` with `error_code` `runtime_missing` or `start_failed`, which name the Visual C++ runtime); `POST /api/library/matches/{id}/voice/transcribe` (`language`, `es` by default; 409 `transcriber_not_installed`), queued in the background (503 `queue_full` leaves the old state) | yes, the install |
| `demo_voice` | `GET /api/library/matches/{id}/voice/demo`; with `extract`, `POST` the same path, which reads both teams' in-game voice from the demo file in the background (`voicecomms`, one Ogg per speaking player from demo time 0, stored once per demo under the profile's `voice/demo`); `GET .../voice/demo/{steamid}/audio` is what the Replay's Voz de la demo plays | no |
| `delete_voice` | `GET` then `DELETE /api/library/matches/{id}/voice`: files, sync and transcript; the match stays | yes |
| `voice_plugin_status` | `GET /api/library/voice/plugin`, or with `setup_cs2` `POST /api/library/voice/plugin/setup` (writes `gamestate_integration_cliphub.cfg` into CS2's `cfg` with the current port and a per-profile token; 404 `cs2_not_found`): game state config `installed`, `stale` or `missing`, the plugin's inbox, whether CS2 reports a live match and the pending count | no |
| `list_voice_recordings` | `GET /api/library/voice/recordings`: the TeamSpeak plugin's recordings, newest first, `status` pending or attached, map, score, trigger (`auto` or `manual`), tracks with nickname, length and `failed` when Studio could not encode one (stored once, not retried), and the note on why one waits; only a complete recording with a final score attaches on its own; filtered by `status` in the tool | no |
| `attach_voice_recording` | `POST /api/library/voice/recordings/{id}/attach` with `match_id`: copies the tracks into the match's team voice (409 `voice_present` when it has one), ties tracks to players the teams remember, and syncs by CS2's round live times (`method` `game_clock`) or as an import does | no |
| `set_voice_speakers` | `PUT /api/library/matches/{id}/voice/speakers` with `speakers` `[{track, steamid64}]`; a plugin track's TeamSpeak identity is saved on the player's team | no |
| `delete_voice_recording` | `GET /api/library/voice/recordings` for the summary, then `DELETE /api/library/voice/recordings/{id}` (the inbox folder and the row; an attached match keeps its voice) | yes |
| `rename_callout` | `PUT /api/library/maps/{map}/callouts/{token}` with `name`, or `DELETE` for an empty name (back to CS2's name) | no |
| `save_lineup` | `POST /api/library/lineups` with `from` (`match_id`, `round`, `throw_tick` of a `team_grenades` row: Studio reads the throw, normalizes the pitch for `setang`, sets callouts and estimates movement and jump) or a pasted `setpos x y z;setang p y r` with `map`, `side` and `grenade`, as Guardar lineup and Nuevo lineup do | no |
| `save_pro_lineup` | `POST /api/library/pro-lineups/{map}/lineups/{id}/save` (a slug is turned into its id with `GET /api/library/pro-lineups/{map}` first): a Libreta lineup tagged `pro` with its technique estimated, as Guardar en la Libreta does on Lineups pro; a second save returns the copy already there; `delete_lineup` undoes it | no |
| `import_pro_match` | `POST /api/library/pro-matches/{match}/{entry}/import`: Studio reads the map from `GET /v1/pro-matches/{match}`, downloads the `.zvpro.zst` analysis of each of its demos (`GET /v1/pro-matches/{match}/{map}`, never the demo), joins the parts of a map split by a server restart (the later part keeps a replayed round) and stores it as a library match of source `pro`, read only until its `.dem` is imported, with the HLTV team names on its sides and the maps of one HLTV match in one library series, opened by its first imported map with the catalog's format, event and veto (team slots in HLTV order; a step of a team outside the library goes to the other slot when only one team is in it, else only the decider stays), marked `from_import` in `list_library_series`; while marked, each later import or reprepared map refreshes the format, teams and veto (a decider guessed from the map count gives way to the HLTV veto), and a person's edit of any of them clears the mark and is never overwritten; as the 2D button of a map does on Partidas pro; returns `match_id` and `studio_path`; a second import returns the same match, or replaces its analysis in place when the catalog's `sha256` or `analysis_version` is newer; an unreadable file is `pro_map_unreadable` | no |
| `follow_pro_team`, `unfollow_pro_team` | `POST /api/library/pro-follows` (`team_id`, `name`): Studio imports the team's last five maps, its maps of the last 14 days and every new one after each catalog refresh, as Seguir equipo does; `DELETE /api/library/pro-follows/{team}` stops it and keeps the imported maps | no |
| `update_lineup` | `PATCH /api/library/lineups/{id}` (`title`, `status`, `tags`, `notes`, `movement`, `jump`, `click`); map, side, grenade and the spot cannot change | no |
| `delete_lineup` | `GET` then `DELETE /api/library/lineups/{id}` | yes |
| `create_library_series`, `update_library_series` | `POST /api/library/series`, `PATCH /api/library/series/{id}` (`format`, `event`, `veto`, `team_ids`, `vetoes`) | no |
| `delete_library_series` | `GET` then `DELETE /api/library/series/{id}`; its matches stay | yes |
| `set_match_series` | `PATCH /api/library/matches/{id}` with `series_id` or null. The maps of one FACEIT match (`1-<match id>-<map>-1.dem`) join one series by themselves after their analysis: the series a map already has, else a new BO3 or BO5 with both teams | no |
| `follow_faceit_player`, `unfollow_faceit_player` | `POST /api/faceit/followed` (`nickname`); `DELETE /api/faceit/followed/{id}` (a seeded player is hidden) | no |
| `link_player_faceit` | `POST /api/library/players/{steamid}/faceit` | no |
| `launch_practice` | `POST /api/practice/launch` (`map`, at most 20 spots with `label`, `pos`, `ang` as pitch and yaw, `grenade`); a CT molotov becomes an incendiary, as on the grenades page. With `lineup_ids`, `GET /api/library/lineups/{id}` for each, all on `map`, as Practicar selección does in the Libreta. With `pro_lineup_ids` (ids or slugs), `GET /api/library/pro-lineups/{map}`: label the title, `pos` the origin, `ang` pitch and yaw, `grenade` the kind | yes |
| `launch_bot_replay` | `POST /api/library/matches/{id}/tactical/rounds/{round}/bot-replay` (`side` the user plays, `replace_steamid` a player of that side whose spawn and positions the user takes), as Practicar contra esta ronda does in the Replay's round panel. Studio writes `cliphub_practice.cfg`, which only defines aliases and binds because a fresh launch runs it before the map's server exists. In game the first KP_ENTER applies the server settings, kicks the bots and joins the side; the second adds one bot per player of the other side (`bot_stop 1`, `bot_dont_shoot 1`); later ones place them with `setpos_player` at moment 1 of up to 12 (start, grenades, first contact, plant). KP_PLUS steps, KP_MINUS changes spawn, KP_DEL shifts the first bot's number (1 to 8, default 2) when they do not move, KP_INS sets up again, and the keypad digits stand the user on the rivals' throws with the grenade. Plain CS2 cannot make bots walk recorded paths, look where the rival looked or throw grenades without a server plugin; the answer's `limits` says so | yes |
| `merge_teams` | `GET /api/library/teams/{id}` for both teams, then `POST /api/library/teams/{id}/merge` with `into_team_id` | yes |

Short look and Full Demo HUD:

| Tool | What it does |
| --- | --- |
| `list_short_options` | format, kill effect, transition, music, extras and the affiliate banner families and styles, each with a one-line Spanish description and the default |
| `get_song_audio` | one track as audio content, app-only (`_meta.ui.visibility: ["app"]`) |
| `list_hud_themes` | the 11 custom HUDs plus "Original de CS2", with 640x360 JPEG thumbnails of the previews as image content (about 4k tokens for all 11) |
| `choose_short_options` | MCP App `ui://cliphub/short-options`: picker with play buttons; text fallback without UI |
| `choose_hud` | MCP App `ui://cliphub/hud-gallery`: preview grid that calls `plan_full_demo` |
| `show_zones` | MCP App `ui://cliphub/zone-map`: the map outline with the zones of a finding marked (lower floor dashed) and its `studio_path`; text fallback without UI |
| `import_demo`, `wait_for_demo`, `wait_for_job`, `generate_short`, `render_full_demo`, `record_round_pov`, `retry_video`, `change_video_music`, `render_stream`, `render_stream_summary`, `wait_for_stream_render` | MCP App `ui://cliphub/job-progress` (React, `mcp-apps/`): live progress bar that follows the job by itself; nothing shows while a start waits for approval |

`import_demo` accepts only an absolute local-drive path to a `.dem` or `.dem.zst` file (no UNC
paths), refuses directories and files above the server's 700 MiB per-demo limit, and leaves real
validation to the orchestrator. Every id argument is validated (UUIDs, `seg-` ids, strict variant
and video names) before it reaches a URL, and a panic in a handler is recovered so the stdio server
survives.

`studio_status` reports whether capture is enabled, so Claude says so before it starts a Short
instead of failing halfway.

### Prompts

The server offers one prompt, `match_prep` (Preparar un partido), which shows in the client's menu and
starts the guided match prep. Arguments: `rival` (required, a team name or id), `maps` (such as
`de_mirage, de_nuke`; empty asks after the veto) and `from` (first day of the matches to read,
YYYY-MM-DD). It tells the assistant to find the team, read `prepare_team`, `team_grenades` and
`player_positions` per map, show the deciding zones with `show_zones`, repeat every data warning,
keep the `studio_path` of each finding (every row of the team tools has one) and use `find_rounds` for
the rest. The notes it asks for are Markdown with the veto first and, per map, these sections in
order: Tendencias en T por compra, Setups CT por ventana de tiempo, Utilidad recurrente, Jugadores
clave and Anti-estrategias (one counter per finding, as "if they do X, we do Y"), each finding a
`[text](studio_path)` link. It ends with `create_report` (or `update_report`), `get_report` and
`open_in_studio`.

Not covered yet (see the parity rule in AGENTS.md): the editor, report exports (the
HTML page and the `.cliphub` package), `open_studio`, the Libreta's Modo partida window (its data is `plan_table` and
`list_lineups`) and resources other than the app views. Steam sign-in, the authentication code
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
