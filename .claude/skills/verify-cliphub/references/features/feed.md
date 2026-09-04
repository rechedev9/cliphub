# Feed

Community reel grid. Likes and publish time, not a local job list.

## Sub-features

- `feed-rail` — Sidebar `10 Feed` → `/feed`.
- `feed-sort` — **Recientes** vs **Top semana** (`aria-label="Ordenar feed"`).
- `feed-empty` — **Todavía no hay nada publicado**.
- `feed-offline` — Orchestrator down → **Servicio local sin responder** + **REINTENTAR**.
- `feed-error` — Non-offline load failure → **No se pudo cargar el feed**.

## How to get to it (user POV)

- Click **Feed**.
- Toggle Recientes / Top semana when the grid has items.

## What done looks like

Header **LA COMUNIDAD FORJA**. Grid shows community reels. Top semana ranks the last 7 days by likes, falling back to the full list when that window is empty. Offline is an alert with REINTENTAR, not a skeleton that never ends.

## Driving it with zv verify

Preconditions:

- Cheap: map present.
- Live grid: orchestrator reachable. Feed is a remote-ish list through the local API.

- **Cheap contract.** `./bin/zv verify prove --feature feed --format json`. `drive.route` is `/feed`.
- **Live API.** Desktop feed has no list API. When Studio is up, prove GETs `/api/capabilities` so an offline orchestrator is visible. `user_path` becomes inspected, never pass.
- **Open.** Click **Feed**. Heading LA COMUNIDAD FORJA.
- **Empty.** Zero items → Todavía no hay nada publicado. Sort toggles are hidden.
- **Sort.** With items, `aria-label="Ordenar feed"`: Recientes (Más recientes), Top semana (Top de la semana).
- **Offline.** `role="alert"` + REINTENTAR. A rejected `listFeed` must not leave the skeleton spinning.

## Gotchas

- Top semana falls back to the full list when nothing is in the last 7 days, so a short seed dataset never renders an empty grid.
- Feed is not Biblioteca. Local ready reels do not have to appear here.
