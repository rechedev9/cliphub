# Inicio

First-run door inside the Studio shell. Not a marketing page.

## Sub-features

- `inicio-rail` — Sidebar `00 Inicio` → `/onboarding`.
- `inicio-root` — `/` redirects to `/onboarding`.
- `inicio-doors` — Guide plates: Sube una demo, Corta un stream, Busca un jugador.
- `inicio-sharecode` — Paste a `CSGO-` share code. Decode works without Steam configured.
- `inicio-steam-recent` — Recent matches after Steam is saved in Ajustes.

## How to get to it (user POV)

- Open ClipHub Studio. Land on EMPIEZA AQUÍ.
- Click **Inicio** in the numbered rail.
- From empty Partidas, click **EMPIEZA AQUÍ**.

## What done looks like

Header **EMPIEZA AQUÍ** is visible. The three doors are there. Footer notes: El .dem no sale de tu PC, Sin login. Empty state does not invent extra upload/stream links. `/` never stays on a blank marketing root.

## Driving it with zv verify

Preconditions:

- Cheap: repo root, map present.
- UI walk: `drive.open_url` from `zv verify prove --feature inicio --format json`.

- **Cheap contract.** Run `./bin/zv verify prove --feature inicio --format json`. `ok` is true, `drive.route` is `/onboarding`.
- **Live API.** When Studio is up, prove GETs `/api/steam/account`. Empty account is success. `user_path` becomes inspected, never pass.
- **Open Inicio.** Click **Inicio**. Heading EMPIEZA AQUÍ. Doors: Sube una demo → `/upload`, Corta un stream → `/streams`, Busca un jugador → `/players`.
- **Share code without Steam.** Paste a well-shaped code. Result `status: "decoded"` is success, not an error.
- **Root redirect.** Open `/`. Location becomes `/onboarding`.
- **Dry-run.** `./bin/zv verify prove --feature inicio --dry-run --format json` issues no HTTP and does not touch jobs.db.

## Gotchas

- Steam GC is an explicit user action only. Never open it at startup.
- `matchId` / `outcomeId` stay strings across HTTP (they exceed 2^53).
- Unsigned installer. Actualizar reads GitHub `releases/latest`. Vercel is not the updater.
