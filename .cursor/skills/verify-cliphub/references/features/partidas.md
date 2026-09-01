# Demos

Match list for parsed and scanned demos, including series grouping.

## Sub-features

- `partidas-rail` — Sidebar `01 Demos` → `/matches`.
- `partidas-empty` — Empty state sends a first-run user to Inicio.
- `partidas-series` — Series cards → `/series/{seriesId}`.
- `partidas-detail` — Match row → `/matches/{jobId}`.
- `partidas-offline` — Orchestrator down is service-offline copy, not a bad demo.

## How to get to it (user POV)

- Click **Demos** in the rail.
- Finish an upload and return to the match list.
- From empty state, **EMPIEZA AQUÍ** goes to Inicio, not a hidden upload shortcut.

## What done looks like

The list shows local jobs. A series shares a client-minted `series_id`. Opening a match shows roster and next actions (Shorts, Vídeos largos) without inventing remote match data. Empty copy: **Aún no hay demos**.

## Driving it with zv verify

Preconditions:

- Cheap: map present.
- Live list: running orchestrator (`zv verify http --format json`).

- **Cheap contract.** `./bin/zv verify prove --feature partidas --format json`.
- **Live API.** When Studio is up, prove GETs `/api/demos/jobs` on Studio `web_url` (same-origin proxy, not the orchestrator token). Empty list is success. `user_path` becomes inspected, never pass.
- **Empty.** No jobs: title Aún no hay demos, action EMPIEZA AQUÍ → `/onboarding`.
- **Series.** Section `aria-label="Series"`. Row link `/series/{seriesId}`.
- **Offline.** `503 {code:"service_unavailable"}` → Servicio local sin conexión, not a parse error.
- **HTTP.** `./bin/zv verify http --format json` — absent is honest, not a fake Pass.

## Gotchas

- `scanned` in a series is settled, not progress.
- Orchestrator 404 after restart is unrecoverable for that job id; Retry cannot re-drive a gone job.
- Service offline is `503 {code:"service_unavailable"}`.
