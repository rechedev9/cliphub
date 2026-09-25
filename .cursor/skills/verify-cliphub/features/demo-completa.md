# Demo completa

Full Demo 16:9 records every round from one POV through HLAE and CS2, then composes the recap. Cloud Linux cannot run that capture. Do not report Pass from this VM.

## Sub-features

- `full-door` opens the constructor from `Crear vídeo largo`, `/clips/nueva?formato=full`, or `/clips/<id>/nuevo?formato=full`.
- `full-plan` loads the editorial plan and enables `Crear Full Demo` when the plan is valid.
- `full-capture` records through HLAE and a running `cs2.exe` on King's Windows Studio.
- `full-wait` is the 16:9 wait overlay. Live overlay inspect is `zv verify prove --feature full-demo-16x9-wait --job-id <uuid>`.

## How to get to it (user POV)

- From the empty hub, choose `Crear vídeo largo`. That opens the upload door with `formato=full`, not capture.
- From a parsed partida, choose `Preparar vídeo largo`. `Vídeo largo 16:9` is the format-bar label on the constructor, not the hub CTA.
- Open `/clips/<job>/nuevo?formato=full` after a POV exists.

## Driving it with control-cliphub

Preconditions:

- Linux and any host where `zv.host.capture_recertification` is not `studio_live`: stop. Record gap `hlae_cs2_windows_studio`. Do not click `Crear Full Demo`.
- Windows Studio with live `ports.json`, `jobs.db`, `/healthz`, `C:\HLAE-*\HLAE.exe`, and running `cs2.exe`: continue with `zv`, not with a fake Pass from web alone.

- **Name the gap.** Run `./bin/zv verify doctor --format json`. On Cloud Linux the report is `ok=false`, `closed=true`, and `gaps` contains `hlae_cs2_windows_studio`. That is the proof this host cannot recertify capture.
- **Constructor door only.** On Linux you may open `Crear vídeo largo` as a UI door. Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs goto --path /clips/nueva?formato=full`. The tab title is `Cargar demo`. The heading is `Crea un vídeo largo`. The format bar is a compact `aria-pressed` pair under `Tipo de vídeo` (`Short 9:16` / `Vídeo largo 16:9`); the pressed full button is `Vídeo largo 16:9`. Descriptions sit beside the pair, not inside the button. The file chooser `Elegir demos de CS2` is enabled after hydration. This is not Full Demo Pass.
- **Produce URL without a job.** `goto --path /clips/<uuid>/nuevo?formato=full` waits until `Cargando la partida` is hidden. Without an orchestrator the page settles on `Servicio local sin conexión`. Do not treat that empty state as the Full Demo form. The live produce heading after a parsed POV is `Full POV Chill`. Groups are `HUD de la partida`, `Sonido`, `Entre rondas`, `Overlays`, and the `Intro, sponsor y outro` upload panel (MP4 cards Intro · Sponsor · Outro; the sponsor plays after round 2). `Sonido` note is `Audio de partida equilibrado automáticamente, sin música de fondo.` and exposes gain sliders `Juego` and `Voces` (percent). The old `Sponsor` group (toggle, placement, window, narration) is retired; the sponsor is the middle card of the upload panel. Do not look for the retired Aspecto / Extra / Avanzado accordions, and do not treat a four-group HUD/Sonido/Overlays/Sponsor list as current.
- **Windows inspect.** On King's Studio, run `./bin/zv verify prove --feature demo-completa --job-id <uuid> --format json`. The command GETs `/api/jobs/{id}?view=status`. `user_path` must not become `pass` from that GET. Screenshot of Electron is still a named remaining gap (`studio_overlay_walk`).
- **Forbidden.** Do not POST generate. Do not claim hosted CI green is HLAE proof. Do not treat `C:\HLAE\HLAE.exe` as valid. Do not treat an installed CS2 path as a running `cs2.exe`.

## Gotchas

- `Crear vídeo largo` on the empty hub is an upload door. Agents keep misreading it as capture. It is not.
- A parsed partida offers the hub link `Preparar vídeo largo`. `Vídeo largo 16:9` is only the constructor format-bar button. `/clips/nueva` does not add a roster destination switch (`allowDestinationSwitch={false}`; e2e count is 0).
- The primary full-demo roster CTA is `Continuar al vídeo largo`. The Short roster CTA is `Continuar al Short`.
- After a parsed POV the form has five groups: `HUD de la partida` (not the brief chip `HUD`), `Sonido` (toggle `Incluir voces del equipo` plus gain sliders `Juego` / `Voces`, added with the 8dd0aa93 layout/audio pass), `Entre rondas` (transitions; added with the #187 simplify), `Overlays`, and `Intro, sponsor y outro`.
- The enqueue toast still says `Sigue el progreso en Demos y vídeos`. The produce footer ready hint uses the same leftover rail name, but on Full Demo it is `sr-only`. Name the live string; do not edit it from this skill.
- `/clips/<id>/nuevo` first paints `Cargando la partida`. A snapshot taken too early is only the skeleton.
- `zv verify prove --feature full-demo-16x9-wait` without `--job-id` fail-closes with `studio_job_id_required` even when Studio is up.
- `web/e2e/full-demo.spec.ts` stubs `/full-demo/plan`. Those tests are the constructor contract. They are not recertification.
- Desktop e2e can boot Electron on a machine that has the assembled resources. Booting the shell is not HLAE capture.
