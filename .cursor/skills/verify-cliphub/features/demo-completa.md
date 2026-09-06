# Demo completa

Full Demo 16:9 records every round from one POV through HLAE and CS2, then composes the recap. Cloud Linux cannot run that capture. Do not report Pass from this VM.

## Sub-features

- `full-door` opens the constructor from `Crear vídeo largo` or `/clips/<id>/nuevo?formato=full`.
- `full-plan` loads the editorial plan and enables `Crear Full Demo` when the plan is valid.
- `full-capture` records through HLAE and a running `cs2.exe` on King's Windows Studio.
- `full-wait` is the 16:9 wait overlay. Live overlay inspect is `zv verify prove --feature full-demo-16x9-wait --job-id <uuid>`.

## How to get to it (user POV)

- From the empty hub, choose `Crear vídeo largo`. That opens the upload door with `formato=full`, not capture.
- From a parsed partida, choose Vídeo largo 16:9.
- Open `/clips/<job>/nuevo?formato=full` after a POV exists.

## Driving it with control-cliphub

Preconditions:

- Linux and any host where `zv.host.capture_recertification` is not `studio_live`: stop. Record gap `hlae_cs2_windows_studio`. Do not click `Crear Full Demo`.
- Windows Studio with live `ports.json`, `jobs.db`, `/healthz`, `C:\HLAE-*\HLAE.exe`, and running `cs2.exe`: continue with `zv`, not with a fake Pass from web alone.

- **Name the gap.** Run `./bin/zv verify doctor --format json`. On Cloud Linux the report is `ok=false`, `closed=true`, and `gaps` contains `hlae_cs2_windows_studio`. That is the proof this host cannot recertify capture.
- **Constructor door only.** On Linux you may open `Crear vídeo largo` as a UI door. Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs goto --path /clips/nueva?formato=full`. The page is `Cargar demo`. This is not Full Demo Pass.
- **Windows inspect.** On King's Studio, run `./bin/zv verify prove --feature demo-completa --job-id <uuid> --format json`. The command GETs `/api/jobs/{id}?view=status`. `user_path` must not become `pass` from that GET. Screenshot of Electron is still a named remaining gap (`studio_overlay_walk`).
- **Forbidden.** Do not POST generate. Do not claim hosted CI green is HLAE proof. Do not treat `C:\HLAE\HLAE.exe` as valid. Do not treat an installed CS2 path as a running `cs2.exe`.

## Gotchas

- `Crear vídeo largo` on the empty hub is an upload door. Agents keep misreading it as capture. It is not.
- `zv verify prove --feature full-demo-16x9-wait` without `--job-id` fail-closes with `studio_job_id_required` even when Studio is up.
- `web/e2e/full-demo.spec.ts` stubs `/full-demo/plan`. Those tests are the constructor contract. They are not recertification.
- Desktop e2e can boot Electron on a machine that has the assembled resources. Booting the shell is not HLAE capture.
