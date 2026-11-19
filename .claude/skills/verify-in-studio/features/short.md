# Short

A vertical video of one player's best plays: Studio captures them in CS2 through HLAE, then FFmpeg renders the Short. Studio only (`/shorts`, `/shorts/<job_id>`). The result lands in Mis vídeos (`/mis-videos`).

Not yet proven by this skill. The handles below come from the code at `83ce9fff` (2026-11-19). The first live run updates this file.

## Sub-features

- The home: h1 `#flow-home-title` "Tu mejor minuto, en vertical", and the demo list h2 `#demo-list-title` "Elige una demo".
- The stepper `ol[aria-label="Pasos"]` (Demo, Jugador, Jugadas, Grabar). The current step has `aria-current="step"`.
- The plays list (the "Prepara tu Short" list): section `aria-label="Jugadas"` (or "Muertes"), each play a `button[aria-pressed]`, and "Elegir las mejores", "Seleccionar todas", "Limpiar".
- Series of Shorts: `/series/<id>`.
- Mis vídeos: h1 "Mis vídeos", in-progress h2 "Grabando en este PC" or "Montando el vídeo en este PC", and the finished list h2 `#mis-videos-listos` "Listos" with one `article` per video.

## How to get to it (user POV)

1. Rail "Short", then "Crear Short" on a demo card. Or "Crear Short" on a [Partidas](partidas.md) row.
2. A job without a player shows "Elige de quién será el vídeo" and "Elegir jugador".
3. The page h1 reads "El Short de <player>". Pick plays, then the CTA "Crear Short". It reads "Preparando grabación…" while it starts.
4. A toast "Short en preparación" appears and the page moves to `/mis-videos`. The video shows under "Listos" when the render ends.

## Driving it with the browser and zv-mcp

1. Import and wait as in [Partidas](partidas.md). Then `create_job` for one player and `wait_for_job` until the job is parsed.
2. `list_moments` returns the same plays as the list, ranked as "Elegir las mejores" ranks them. It returns the top 10 and a `total`; pass a larger `limit` to compare every play with the list.
3. `generate_short` with the chosen segment ids returns a `variant`. It asks for approval first, so it needs a client that answers elicitation or the approval code path.
4. `wait_for_job` with that `variant`, in a loop (55 s at most per call), then `list_videos`.
5. In the browser: `/mis-videos` shows the card under "Listos", with "Reproducir <title>" and "MP4".

Proof: `wait_for_job` ends with no `failure_code`, the new MP4 is in `$p\data\jobs\<job>\renders\<variant>\revisions\<revision>\videos\` (`status.json` beside `revisions\` names the current revision), and the card is under "Listos".

## Gotchas

- Needs Steam, CS2 and HLAE on the machine, and CS2 records for minutes. A capture can sit as "capture queued" for up to 2 min after the start.
- Headless `claude -p` declines elicitation, so nothing starts.
- The rows carry no segment id in the DOM: match MCP plays to rows by round and side.
- A saved draft shows "Configuración recuperada…" with "Empezar de cero". Use a new profile for a clean flow.
- Starting from the browser fails: the start is a POST that needs the window's capability cookie.
