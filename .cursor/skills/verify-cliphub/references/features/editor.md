# Editor

Multitrack montage of MP4s that already exist. The persisted editor plan is canonical; FFmpeg renders the final.

## Sub-features

- `editor-rail` — Sidebar `08 Editor` → `/editor`.
- `editor-empty` — **Todavía no hay montajes** with **Nuevo proyecto**.
- `editor-create` — Creates `Nuevo montaje` and routes to `/editor/{id}`.
- `editor-workspace` — Timeline, assets, overlays, preview.
- `editor-offline` — List/create failures surface as alerts, not a hung skeleton.

## How to get to it (user POV)

- Click **Editor**.
- Click **Nuevo proyecto**, or open an existing row.

## What done looks like

Header **Editor**. Empty state offers **Nuevo proyecto**. A created project opens `/editor/{id}`. The plan on disk is what render uses; do not bolt ad hoc FFmpeg around it.

## Driving it with zv verify

Preconditions:

- Cheap: map present.
- Live: orchestrator up. This path does not need HLAE/CS2.

- **Cheap contract.** `./bin/zv verify prove --feature editor --format json`. `drive.route` is `/editor`.
- **Live API.** When Studio is up, prove GETs `/api/editor/projects`. Empty list is success. `user_path` becomes inspected, never pass.
- **Empty.** Click **Editor**. Title Editor. Empty: Todavía no hay montajes.
- **Create.** Click **Nuevo proyecto**. Location becomes `/editor/{id}`.
- **Offline.** List failure: No se pudieron cargar los proyectos. Create failure: No se pudo crear el proyecto.
- **Persistence.** Reload `/editor/{id}` and the same project title is still there.

## Gotchas

- This editor montages already-rendered MP4s. It is not HLAE capture and not Biblioteca compose.
- Mutation capability errors are distinct from service-unavailable.
