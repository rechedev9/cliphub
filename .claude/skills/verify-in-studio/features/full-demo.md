# Demo completa (Full Demo)

The whole match, or a summary, from one player's point of view: CS2 capture through HLAE, then the FFmpeg render with overlays. Studio only (`/videos-largos`, `/videos-largos/<job_id>`, `/videos-largos/serie/<id>`). The result lands in Mis vídeos.

Not yet proven by this skill. The handles below come from the code at `83ce9fff` (2026-11-19). The first live run updates this file.

## Sub-features

- The home: h1 "La partida entera, con su vista", and the card CTA "Preparar demo completa".
- The kind switch: group `aria-label="Tipo de vídeo"` with "Partida entera" and "Resumen" (`aria-pressed`).
- The plan page: h1 "POV de <player> en <Mapa>", the plan line "<n> rondas · Ajustes guardados" (or "Se aplicarán al crear").
- The CTA "Crear demo completa", or "Poner demo completa en cola" while CS2 is busy.
- The stepper `ol[aria-label="Pasos"]` (Demo, Jugador, Ajustes, Grabar).

## How to get to it (user POV)

1. Rail "Demo completa", then "Preparar demo completa" on a card. Or "Demo completa" on a match page in [Partidas](partidas.md).
2. Choose the player and the settings. The plan line updates.
3. "Crear demo completa". A toast "Demo completa en cola" appears and the page moves to `/mis-videos`.

## Driving it with the browser and zv-mcp

1. Import and wait as in [Partidas](partidas.md). `create_job` for one player (a multi-POV job is refused).
2. `plan_full_demo` returns a `plan_hash`. It never starts CS2.
3. `render_full_demo` with that `plan_hash` asks for approval, then starts. Follow it with `wait_for_job` and the variant `gameplay-pov-60`, then `list_videos`.
4. In the browser: the plan page shows the planned rounds, and `/mis-videos` shows the card under "Listos" with the pill "Demo completa".

Proof: `$p\data\jobs\<job>\full-demo\current-plan.json` exists after the plan, the MP4 is in `renders\gameplay-pov-60\videos\`, and the card is under "Listos".

## Gotchas

- Needs Steam, CS2 and HLAE unless the capture is already cached (a render only). Recording a whole match takes a long time.
- A stale `plan_hash` is refused: plan again.
- A localStorage draft (`cliphub.full-demo.draft.v1:<job>`) can hide the MCP plan in the UI, with "Configuración recuperada de tu última visita."
- The render depends on FFmpeg 8.1.2, which Studio installs into `$p\tools` on first boot.
