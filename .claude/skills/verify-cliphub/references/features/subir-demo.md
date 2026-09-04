# Subir demo

Local `.dem` intake. The file never leaves the PC.

## Sub-features

- `upload-rail` — Sidebar `02 Subir demo` → `/upload`.
- `upload-door` — Inicio plate **Sube una demo**.
- `upload-drop` — Dropzone / file picker.
- `upload-roster` — Roster parse after scan; zero players is a bad demo.
- `upload-sharecode` — Share-code import is a different door (Inicio / Steam), same CreateJob path.

## How to get to it (user POV)

- Click **Subir demo**.
- From Inicio, choose **Sube una demo**.

## What done looks like

A demo is accepted, roster appears, and a job exists in Partidas. Scan copy includes ANÁLISIS AUTOMÁTICO. Zero-player scan: Sin jugadores — ¿seguro que es una demo de CS2? No FACEIT Download API. No credential printed.

## Driving it with zv verify

Preconditions:

- Cheap: map present.
- Real parse: a local `.dem` (not in git) and a live orchestrator.

- **Cheap contract.** `./bin/zv verify prove --feature subir-demo --format json`.
- **Live API.** When Studio is up, prove GETs `/api/capabilities`. That is not an upload. `user_path` becomes inspected, never pass.
- **Open upload.** Click **Subir demo** or Inicio **Sube una demo**. Dropzone is visible.
- **Scan then pick.** Drop a `.dem`. Wait for roster. Pick a player. Job lands in Partidas.
- **Empty roster.** Treat as a bad demo, not a transient failure.
- **Offline.** Service-unavailable copy, not a corrupt-file claim.

## Gotchas

- Real `.dem` files are not in git. Do not commit them.
- FACEIT Download API is not approved. Room/Watch download is a user-authorized source.
- `matchId` / `outcomeId` stay strings across HTTP.
