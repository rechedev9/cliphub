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
- In the web edition `web/proxy.ts` authenticates with the session cookie (`web/lib/clipmind/session.ts`) instead of the loopback host check and capability cookie. The same `localAPIRequestError` guard still rejects cross-site requests there. The route handlers the route handlers call the ClipMind server at `CLIPMIND_URL`.
- `cmd/clipmind-server` (`internal/clipmind`) serves `httpapi.AnalysisRoutes()` with one SQLite library per workspace. Its environment and operator commands are at the top of `cmd/clipmind-server/main.go`.
- The browser parses demos with the Go scanner compiled to WebAssembly (`cmd/zv-scan-wasm`), so a demo never leaves the user's PC.

## Sync with Studio

Studio pulls and pushes the `sync_changes` log of a linked workspace and exchanges match files through `/api/sync/artifacts`. Each side sends a sync contract, a hash of the synced tables' columns. A different hash answers 409, and the schema version only picks the code: `sync_client_outdated` or `sync_server_outdated` (`syncContractOK` in `internal/httpapi/sync_handlers.go`). A change to a synced table's columns therefore stops sync until Studio and ClipMind run the same commit. Ship them together.

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

1. Build the Linux server with `CGO_ENABLED=0`, `GOOS=linux` and `GOARCH=amd64` set: `go build -trimpath -ldflags "-s -w" -o <out>/api/clipmind-server ./cmd/clipmind-server`.
2. Build the web edition with `NEXT_PUBLIC_CLIPHUB_EDITION=web` set: `pnpm --dir web run build`. Assemble `<out>/web/app` as `desktop/scripts/assemble.mjs` does: `.next/standalone`, then `.next/static` into `.next/static` and `public/`.
3. Check the build locally: `node server.js` answers `/patrones` with a redirect to `/iniciar-sesion` and `/shorts` with one to `/descargar`.
4. On the VPS, in `/opt/cliphub-strats`: `docker compose stop api`, so `data/` holds no open SQLite, then back up `api`, `web` and `data` to a dated tarball in `/opt/cliphub-strats-backups/`. The new server migrates every workspace on start, so only this backup undoes it.
5. Replace `api/clipmind-server` and `web/app/` (keep the old one as `web/app.old`), then `docker compose up -d --build`.
6. Check `https://cliphubstrats.rechedev.cloud` and `docker logs cliphub-strats-api-1`.

Rollback: `docker compose stop`, restore `api`, `web/app` and `data` from the tarball, then `docker compose up -d --build`.

## Deploy the landing

`landing/lib/site.ts` already names the new installer after the release commit. The site is a static export, but `landing/next.config.mjs` keeps no `output`, so the export needs two edits that are never committed: `output: 'export'` in the config and `export const dynamic = "force-static"` in `app/opengraph-image.tsx`. Run `pnpm --dir landing run build`, revert both (`git status` must be clean again), and check that `landing/out/index.html` names the new `Setup.X.Y.Z.exe`. On the VPS, back up `/opt/cliphub-landing/site`, replace it with `out/`, and run `docker compose restart`: the container mounts the folder, so a renamed folder keeps serving the old files until it restarts.
