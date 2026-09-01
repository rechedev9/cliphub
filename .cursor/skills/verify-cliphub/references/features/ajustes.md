# Ajustes

Desktop settings: installed build, telemetry, and the Steam account the user plays on.

## Sub-features

- `settings-rail` — Sidebar `09 Ajustes` → `/settings`.
- `settings-info` — ClipHub Studio version / build / Electron / Chromium.
- `settings-telemetry` — Opt-in diagnostics. Never includes demos, videos, SteamID, or credentials.
- `settings-steam` — SteamID + authentication code + Web API key. No password stored.
- `settings-bridge` — Version and telemetry need the Electron desktop bridge.

## How to get to it (user POV)

- Click **Ajustes**.
- Header **CONFIGURACIÓN**.

## What done looks like

Three panels: APLICACIÓN (ClipHub Studio), telemetry, Steam. Steam save persists through `/api/steam/account`. Version rows are visible inside the packaged app. A Chrome tab against `drive.open_url` shows the desktop-only note for version/telemetry and can still load Steam over HTTP.

## Driving it with zv verify

Preconditions:

- Cheap: map present.
- Version/telemetry: packaged Studio (Electron bridge). Steam: orchestrator.

- **Cheap contract.** `./bin/zv verify prove --feature ajustes --format json`. `drive.route` is `/settings`.
- **Live API.** When Studio is up, prove GETs `/api/steam/account`. Empty account is success. Version/telemetry still need Electron. `user_path` becomes inspected, never pass.
- **Open.** Click **Ajustes**. Heading CONFIGURACIÓN.
- **Desktop info.** Inside Electron: Versión / Build / Electron / Chromium. In a Chrome tab: La versión instalada solo está disponible dentro de la app de escritorio.
- **Steam save.** Fill SteamID, auth code, API key. Reload: values persist. Password fields are absent.
- **Steam clear.** Clear account. Fields empty after reload.
- **Telemetry.** Activado / Desactivado round-trips through the desktop bridge. Failure copy: No se pudo guardar la preferencia.

## Gotchas

- Never persist `ZV_STEAM_USERNAME` / `ZV_STEAM_PASSWORD` / `ZV_STEAM_GUARD`. Password is prompted on download, held in process memory, or read from env. Never log it.
- The account to connect is the one the user plays on. A secondary bot account enumerates nothing.
- Steam GC is an explicit user action. Never open it at startup. One CS2 session per account.
- Ajustes never writes a Steam password.
