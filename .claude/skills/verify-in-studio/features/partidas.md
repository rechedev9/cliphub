# Partidas (match library)

The library of imported demos: the list at `/partidas` and the match page at `/partidas/<match_id>`. Both editions show it. Import differs: Studio imports in the Electron window (an agent uses `import_demo`), ClipMind imports in the browser at `/importar` ([ClipMind](clipmind.md)).

Last proven: 2026-11-19 on Studio 5.25.0 (`83ce9fff`) with a FACEIT `.dem.zst`, by the journey below.

## Sub-features

- Import: "Importar demos" on the page and "DEMO" at the foot of the rail (Studio window only).
- Tabs above the list: "Privadas" (`/partidas`), "Públicas" (`/partidas/pro`, pro matches from the community server), "Playlists" (`/replay/playlists`), "Series y veto" (`/partidas/series`).
- List controls: the search box "Buscar por mapa, equipo, archivo o etiqueta", "Filtros", "Por día" or "Tabla", and the sort "Fecha de la partida".
- A row: the map card (a link), the score of each team, the source chip ("FACEIT"), tags, a "2D" link and "Crear Short".
- The match page: tabs "Resumen", "Rondas", "Replay", "Métricas", "Vídeos", and the header buttons "Crear Short", "Demo completa" and "Todas las partidas". The Replay tab is [Replay 2D](replay.md).

## How to get to it (user POV)

1. Rail "Partidas". An empty library shows "Aún no hay partidas".
2. Import a demo. The row appears under a day heading such as "sáb 14 nov 2026", and the count reads "1 partida".
3. Click the map name on the row. The match page opens on "Resumen".

## Driving it with the browser and zv-mcp

1. Before the import, open `/partidas` in the browser at `http://127.0.0.1:<web>` and take a screenshot of "Aún no hay partidas".
2. Import through the branch's MCP (`Invoke-StudioMcp` from [the skill](../SKILL.md#drive)), with a demo from outside the repo:

```powershell
$demo = Invoke-StudioMcp import_demo @{ path = '<absolute path to a .dem or .dem.zst>'; tags = @('verify') }
do { $s = Invoke-StudioMcp wait_for_demo @{ id = $demo.demo_id; wait_seconds = 50 } } while ($s.state -in 'queued', 'parsing')
$s | ConvertTo-Json -Compress | Set-Content "$e\wait_for_demo.json"
Invoke-StudioMcp get_match @{ id = $s.match_id } | ConvertTo-Json -Depth 6 | Set-Content "$e\get_match.json"
```

3. Reload `/partidas` and wait for a team name. The row's link is `a[href="/partidas/<match_id>"]` and its text is the map's display name ("Dust2").
4. Click it. The page's `h1` is the map name, and the line under it reads `<team> 7 : 13 <team> · 14 nov · 30 min · 20 rondas · FACEIT · Sin serie`. The tabs are `role=tab`.

Proof: "1 partida" and the row after the import; the match page shows the same round count as `round_count` in `get_match`; and the profile has `data\library\<demo_id>\demo.dem` and `positions.zvpos`.

Failure path: import a file of random bytes named `corrupt.dem`. `import_demo` fails with `not_a_demo: the file is not a CS2 demo`, `studio.log` has `httpapi: refuse library demo "corrupt.dem" code=not_a_demo`, and the list does not change.

## Gotchas

- The browser cannot import: the import POST needs the window's capability cookie.
- A role and name target "Dust2" matches two links: the map card and "Abrir el replay 2D de Dust2". Target the `href`.
- A FACEIT demo can list a coach (`COACH|...`), so `get_match` can return 11 players.
- The browser console shows `GET /api/teams/logo` 404 for teams without a logo. That is not a failure.
- A repeated file is detected by its fingerprint and not imported twice (`duplicate`). Use another file or a new profile to import again.
- A 157 MB `.dem.zst` was `ready` 4 s after the import on 2026-11-19.
