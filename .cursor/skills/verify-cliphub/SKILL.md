---
name: verify-cliphub
description: Drive ClipHub Studio web the way a user does, doctor the live host, and name the Windows HLAE/CS2 gap. Use when proving a user-facing change, walking /clips or another mapped feature, or checking whether this machine can recertify capture.
---

# Verify ClipHub

Drive the real Studio web UI. Do not claim a Full Demo or 9:16 capture Pass from Cloud Linux. The capture host of record is King's Windows ClipHub Studio.

The lever is two CLIs. `zv verify` doctors Windows Studio, HLAE, and running `cs2.exe`. `control-cliphub.mjs` launches an isolated web instance, walks one mapped feature, and writes proof artifacts. Reuse both. Do not add a third doctor.

## Launch

Web is the surface this skill can start on Linux. Studio Electron plus HLAE/CS2 is a Windows path. Do not launch a second web server on a port this run did not allocate.

From the repo root:

```sh
node .cursor/skills/verify-cliphub/control-cliphub.mjs launch
```

Ready when `http://127.0.0.1:<port>/clips` returns HTTP 200 and `doctor --json` shows `web.ok=true`. Default port is `4173` so landing (`3100`) and a casual `next dev` (`3000`) stay untouched. Override with `--port` or `CLIPHUB_VERIFY_PORT`.

The run file is `.cursor/skills/verify-cliphub/.run/state.json`. It records the PID this launch started, the port, and the evidence directory. Drive only that instance. A second `launch` on the same port reuses that PID. A different `--port` preflights the new port and `next` install, then stops the prior instance before starting the new one, so a bind failure does not drop a live server that `cleanup` can no longer find. Passing `--evidence` on a reuse updates the directory later doctor and drive writes use.

If `web/node_modules` is missing, run `pnpm --dir web install --frozen-lockfile` first. Launch uses `pnpm --dir web run dev` with `--hostname 127.0.0.1`. Production `pnpm --dir web run build && pnpm --dir web run start` is the Playwright contract path. Use it when you need a standalone server, not for a routine agent walk.

Studio Electron (Windows, or a Linux shell without CS2) needs `pnpm --dir desktop run build`, `pnpm --dir desktop run assemble`, then `pnpm --dir desktop run test:e2e:ui`. That suite boots through `desktop/e2e/isolated-userdata.cjs` so it does not steal the user's single-instance lock. This skill does not start Electron. A green web walk is not an Electron boot.

Tear down with `cleanup`. Leave the evidence directory in place.

## Doctor

Run doctor first whenever the instance looks wrong.

```sh
node .cursor/skills/verify-cliphub/control-cliphub.mjs doctor --json
./bin/zv verify doctor --format json
```

Build `./bin/zv` with `go build -o bin/zv ./cmd/zv` when the binary is missing. `go run ./cmd/zv verify doctor --format json` is the same command.

`control-cliphub doctor` wraps `zv verify doctor` and then probes the launched web origin. Read these fields:

- `web.ok` is true when this run's Studio web answers `/clips` with HTTP 200 and the document title contains `ClipHub`.
- `zv.ok` is true only when Windows Studio is up, HLAE is detected, and `cs2.exe` is running. Cloud Linux is never `zv.ok`.
- `zv.gaps` must include `hlae_cs2_windows_studio` on Linux. That string is the closed capture gap. Do not rewrite it into a Pass.
- `zv.host.capture_recertification` is `unavailable` on this VM.

`--dry-run` on doctor skips the live `/healthz` GET inside `zv` and still prints the catalog plus the named gap. It does not write `jobs.db` and does not enqueue capture.

Refuse to drive when `web.ok` is false. A missing web origin is not a skip of the user path. A named HLAE gap is not a reason to skip `inicio`.

## Drive

Read `.cursor/skills/verify-cliphub/features/` before you pick a path. Drive one mapped feature through the CLI. Prefer ARIA roles, accessible names, and `data-slot` handles from `web/e2e`. Do not click by coordinates.

```sh
node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature inicio
```

`inicio` is the first-run Clips hub. The command opens `/clips`, waits until `Cargando partidas` is hidden, asserts the empty-hub region `¿Qué quieres crear?` or the populated heading `Tus demos y vídeos`, checks the numbered rail `Clips y vídeos` has `aria-current="page"`, follows `Crear Short` to `/clips/nueva?formato=short`, and returns through the rail. After the return it waits for the hub again before writing proof. `goto`, `snapshot`, and `screenshot` of `/clips` (including `?vista=clips`) also treat the clips-lens heading `Tus vídeos de demos` as ready.

Other drive verbs for a recipe in the feature map:

```sh
node .cursor/skills/verify-cliphub/control-cliphub.mjs goto --path /clips
node .cursor/skills/verify-cliphub/control-cliphub.mjs click --role link --name "Clips de stream"
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --out .cursor/skills/verify-cliphub/artifacts/inicio/hub.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --out .cursor/skills/verify-cliphub/artifacts/inicio/hub.png
```

Relative `--out` resolves from the repo root. `snapshot` and `screenshot` wait until `Cargando partidas` is hidden, then for the empty hub, `Tus demos y vídeos`, or the clips-lens heading `Tus vídeos de demos`.

Stable handles from the live UI and `web/e2e`:

- Rail current page: `[data-slot="sidebar"] a[aria-current="page"]`
- Hub empty region: `section[aria-label="¿Qué quieres crear?"]`
- Creation cards: links named `Crear Short`, `Crear vídeo largo`, `Recortar un stream`
- Upload file input: `input[type="file"]` on `/clips/nueva`
- Stream URL field: textbox `Enlace del vídeo de Twitch, YouTube o Kick`
- Players search: textbox `Buscar jugador seguido`

`zv verify prove --feature <id>` inspects the compiled catalog and, when Studio HTTP is up, GET a read-only probe. That inspect is not a UI walk and not a capture Pass. Use it as a cheap gate. Use `control-cliphub drive` for the user path.

Do not POST `/api/demos/*/generate` or enqueue HLAE capture from this skill. Full Demo 16:9 and live 9:16 wait stay closed on Cloud Linux.

## Evidence

Proof lives under `.cursor/skills/verify-cliphub/artifacts/<run-id>/`. Cleanup deletes the process and the run file. It never deletes this directory.

A proof for a web feature includes all of the following:

- The command that launched the instance and the URL it served
- `doctor.json` with `web.ok=true` and the named `hlae_cs2_windows_studio` gap on Linux
- The user action and the resulting state. A final screenshot alone is not enough.
- `hub.aria.txt` from `snapshot` and `hub.png` from `screenshot`, both showing ClipHub identity
- `drive.json` with the feature id, the routes visited, and the assertions that passed

Exercise the real `/clips` document. Do not set React state from the console. Do not treat `web/e2e` route stubs as a live orchestrator. Empty-hub copy is the honest first-run state when no API is up. That is a valid `inicio` proof.

Mocks stay inside Playwright presentation specs. This skill does not add Playwright to CI. Hosted gates remain `ci-frontend`, `ci-backend`, and `ci-infra`. `zv verify gates --dry-run --format json` lists those commands. Unsigned Studio installers stay on `desktop-release.yml`.

On Windows Studio, a capture proof also needs live `ports.json`, `jobs.db`, `/healthz` `{"service":"cliphub","status":"ok"}`, `C:\HLAE-*\HLAE.exe` (never `C:\HLAE\HLAE.exe`), and a running `cs2.exe`. `zv verify prove --feature demo-completa --job-id <uuid>` then GETs job status. That still is not a screenshot Pass. This CLI does not screenshot Electron.

## Cleanup

```sh
node .cursor/skills/verify-cliphub/control-cliphub.mjs cleanup --dry-run
node .cursor/skills/verify-cliphub/control-cliphub.mjs cleanup
```

Cleanup kills the PID recorded in `.run/state.json` and that process's descendants. It does not `pkill next` or `pkill electron`. `--dry-run` prints the PIDs and the evidence path, then exits 0.

After cleanup, confirm the evidence files still exist at the paths printed in `cleanup` JSON. If they are gone, the cleanup is wrong.

## Helpers

`control-cliphub.mjs` is executable. From the repo root:

```sh
node .cursor/skills/verify-cliphub/control-cliphub.mjs --help
node .cursor/skills/verify-cliphub/control-cliphub.mjs launch
node .cursor/skills/verify-cliphub/control-cliphub.mjs doctor --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature inicio
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --out .cursor/skills/verify-cliphub/artifacts/last/hub.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --out .cursor/skills/verify-cliphub/artifacts/last/hub.png
node .cursor/skills/verify-cliphub/control-cliphub.mjs cleanup
```

`zv verify` stays the Windows Studio lever:

```sh
go build -o bin/zv ./cmd/zv
./bin/zv verify doctor --format json
./bin/zv verify features --format json
./bin/zv verify http --url http://127.0.0.1:8080 --format json
./bin/zv verify prove --feature inicio --dry-run --format json
./bin/zv verify gates --dry-run --format json
```

Existing harnesses you should prefer before inventing another:

- `pnpm --dir web run test:e2e` is the presentation contract. It builds Studio web and stubs APIs. It is not HLAE proof.
- `pnpm --dir desktop run test:e2e:ui` boots real Electron with `isolated-userdata.cjs`.
- `go run ./cmd/zv check` is the backend skill and workflow contract.

Keep feature ids aligned with `internal/verify/catalog.go`. When a route in `web/lib/nav.ts` moves, update the feature file in the same change. `/maintain-verification-skill` is the maintenance loop.
