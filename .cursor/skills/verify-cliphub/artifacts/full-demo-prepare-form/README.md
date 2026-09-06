# full-demo-prepare-form prove run

Cloud Linux walk of the Full POV / prepare-form user path for #164 on 2026-09-06. This VM can open the constructor door. It cannot paint Aspecto / Sonido / Extra / Avanzado: that screen needs a parsed partida plus a live full-demo plan, and this host has no jobs.db, no orchestrator, and no HLAE/CS2.

Commands, in order:

```sh
go build -o bin/zv ./cmd/zv
node .cursor/skills/verify-cliphub/control-cliphub.mjs launch --evidence .cursor/skills/verify-cliphub/artifacts/full-demo-prepare-form
node .cursor/skills/verify-cliphub/control-cliphub.mjs doctor --json
./bin/zv verify doctor --dry-run --format json
./bin/zv verify prove --feature demo-completa --dry-run --format json
./bin/zv verify prove --feature demo-completa --format json
./bin/zv verify prove --feature full-demo-16x9-wait --format json
node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature inicio --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /clips/nueva?formato=full --out .cursor/skills/verify-cliphub/artifacts/full-demo-prepare-form/full-door.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --path /clips/nueva?formato=full --out .cursor/skills/verify-cliphub/artifacts/full-demo-prepare-form/full-door.png
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /clips/11111111-1111-4111-8111-111111111111/nuevo?formato=full --out .cursor/skills/verify-cliphub/artifacts/full-demo-prepare-form/prepare-missing-job.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --path /clips/11111111-1111-4111-8111-111111111111/nuevo?formato=full --out .cursor/skills/verify-cliphub/artifacts/full-demo-prepare-form/prepare-missing-job.png
node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature demo-completa --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs cleanup --json
```

`control-cliphub goto --path /clips/nueva?formato=full` exits 1 here because Playwright `page.title()` is `Cargar demo` (layout template is `%s · ClipHub`). The document still identifies ClipHub. Snapshot/screenshot of that path succeeded.

`control-cliphub` snapshot of `/clips/<uuid>/nuevo?formato=full` waits for hub `Cargando partidas`, not produce `Cargando la partida`, so `prepare-missing-job.png` is the loading skeleton. A same-origin Playwright wait on the launched instance (not a route stub) then recorded the settled empty state in `prepare-settled.*` and the painted door after `Crear vídeo largo` in `full-door-after-click.*`.

`drive --feature demo-completa` refused: Cloud Linux is closed (`hlae_cs2_windows_studio`). Do not fake Pass.

After cleanup the run file is gone, `127.0.0.1:4173` is down, and these files remain.

- `launch.json` is the isolated Next origin `http://127.0.0.1:4173` and pid.
- `doctor.json` has `web.ok=true`, `named_gap=hlae_cs2_windows_studio`, `capture_recertification=unavailable`, `zv.ok=false`.
- `drive.json` is `inicio`: empty hub, `Crear Short`, return via the rail.
- `hub.aria.txt` / `hub.png` show `¿Qué quieres crear?` and the `Crear vídeo largo` card (`/clips/nueva?formato=full`, Horizontal · 16:9). Footer names Windows Studio + CS2 + HLAE.
- `full-door.aria.txt` / `full-door.png` are the constructor door: heading `Crea un vídeo largo`, `Vídeo largo 16:9` pressed, step `3 Preparar vídeo` not yet reached. `Elegir demos de CS2` is disabled (no orchestrator).
- `full-door-after-click.*` is the same door after clicking `Crear vídeo largo` from `/clips`. `Aspecto` count is 0.
- `prepare-settled.aria.txt` / `prepare-settled.png` / `walk.json`: produce URL without a job settles on `Servicio local sin conexión`. Counts: Aspecto 0, Sonido 0, Extra 0, Avanzado 0, freeze tick 0, `Crear Full Demo` 0. `Crear Full Demo` was not clicked.
- `zv-prove-demo-completa.json` and `zv-prove-full-demo-16x9-wait.json` fail-close with gap `hlae_cs2_windows_studio` and `user_path=gap`.
- `cleanup.json` records `evidence_exists=true`.

This is not a Pass on Full Demo 16:9 and not a Pass on live 9:16. Gap `hlae_cs2_windows_studio` stays closed. Aspecto / Sonido / Extra / Avanzado stay unproven on this VM until a parsed job + plan exist on King's Windows Studio.
