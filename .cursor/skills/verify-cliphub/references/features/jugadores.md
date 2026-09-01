# Jugadores

FACEIT-indexed player list for triage. Stats are not the clip source.

## Sub-features

- `players-rail` — Sidebar `06 Jugadores` → `/players`.
- `players-door` — Inicio plate **Busca un jugador**.
- `players-follow` — Followed players and match rooms.
- `players-unconfigured` — FACEIT key missing is unconfigured, not offline.
- `players-offline` — Orchestrator down is service-unavailable.

## How to get to it (user POV)

- Click **Jugadores**.
- From Inicio, choose **Busca un jugador**.
- Browse indexed players. Open a room URL if present.

## What done looks like

Players and matches render from the local FACEIT index. Profile links out. Download stays user-authorized. No Download API. No key in the page or logs.

## Driving it with zv verify

Preconditions:

- Cheap: map present.
- Live FACEIT: `FACEIT_API_KEY` in the environment, never in git.

- **Cheap contract.** `./bin/zv verify prove --feature jugadores --format json`.
- **Live API.** When Studio is up, prove GETs `/api/faceit/followed`. `200` or `503 {code:"faceit_not_configured"}` is success. `user_path` becomes inspected, never pass.
- **Open.** Click **Jugadores** or Inicio **Busca un jugador**.
- **Unconfigured.** Missing key → unconfigured empty state, not a spinner forever.
- **Offline.** `503 {code:"service_unavailable"}` → offline copy, distinct from unconfigured.
- **Search.** Lookup a nick. Followed list persists across reload.

## Gotchas

- FACEIT Data API is approved. FACEIT Download API is not.
- Rate stats must be normalized per round when match lengths differ.
- Use external stats to shortlist demos; use parsed demo evidence to select moments.
