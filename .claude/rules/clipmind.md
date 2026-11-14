---
paths:
  - "cmd/clipmind-server/**"
  - "internal/clipmind/**"
  - "web/lib/clipmind/**"
  - "web/lib/edition.ts"
  - "web/proxy.ts"
  - "services/**"
  - "deploy/**"
  - "landing/lib/site.ts"
---

# ClipMind and the VPS

Studio is local. ClipMind is the web edition of the same analysis. Both build from one commit and ship together.

## Two editions of one UI

- `NEXT_PUBLIC_CLIPHUB_EDITION=web` is read at build time, so the web edition is its own `web` build. Without it the build is the desktop edition.
- `web/lib/edition.ts` lists the desktop-only pages and API prefixes and the web-only account pages. `TestEveryLibraryRouteHasAnEdition` (`internal/httpapi`) fails when a new library, map or team route is tagged neither analysis nor desktop-only.
- In the web edition `web/proxy.ts` authenticates with the session cookie (`web/lib/clipmind/session.ts`) instead of the loopback guard and capability cookie, and the route handlers call the ClipMind server at `CLIPMIND_URL`.
- `cmd/clipmind-server` (`internal/clipmind`) serves `httpapi.AnalysisRoutes()` with one SQLite library per workspace. Its environment and operator commands are at the top of `cmd/clipmind-server/main.go`.
- The browser parses demos with the Go scanner compiled to WebAssembly (`cmd/zv-scan-wasm`), so a demo never leaves the user's PC.

## Sync with Studio

Studio pulls and pushes the `sync_changes` log of a linked workspace and exchanges match files through `/api/sync/artifacts`. Each side sends a sync contract, the hash of the synced columns plus the schema version, and a mismatch answers 409 `sync_client_outdated` or `sync_server_outdated`. A store migration or a change to a synced table therefore breaks sync until Studio and ClipMind run the same commit. Ship them together.

## Servers on the VPS

One VPS, SSH alias `openclaw`, behind its edge Caddy. None of these ship in the installer. Changing anything on it needs the owner's approval first.

| Service | Code | On the VPS |
|---|---|---|
| ClipMind, `cliphubstrats.rechedev.cloud` | `web/` (web edition) and `cmd/clipmind-server` | Compose project `cliphub-strats` in `/opt/cliphub-strats`: `api` (distroless, `api/clipmind-server`) and `web` (Node, `web/app/`), data in `data/` |
| Community API, `cliphub.rechedev.cloud/v1/*`: pro reference, rankings, scraped pro matches | `services/community` | `cliphub-community.service`; `cliphub-community-index.service` indexes new pro demos |
| Telemetry collector | `services/telemetry` | `cliphub-telemetry.service` (`deploy/telemetry/`) |
| Landing, `cliphub.rechedev.cloud` | `landing/` as a static export | Caddy container in `/opt/cliphub-landing`, files in `site/` |

The Compose files and Dockerfiles live only on the VPS. This repo has the code they run.

## Deploy ClipMind

From the release commit, after the Studio release is published:

1. `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o <out>/api/clipmind-server ./cmd/clipmind-server`.
2. `NEXT_PUBLIC_CLIPHUB_EDITION=web pnpm --dir web run build`, then assemble `<out>/web/app` as `desktop/scripts/assemble.mjs` does: `.next/standalone`, plus `.next/static` into `.next/static`, `public/` and `content/`.
3. Check the build locally: `node server.js` answers `/patrones` with a redirect to `/iniciar-sesion` and `/shorts` with one to `/descargar`.
4. On the VPS: back up `api`, `web` and `data` of `/opt/cliphub-strats`, replace `api/clipmind-server` and `web/app/`, then `docker compose up -d --build`.
5. Check `https://cliphubstrats.rechedev.cloud` and `docker logs cliphub-strats-api-1`.
