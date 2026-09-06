# full-demo-pov-fixed-freeze prove run

Cloud Linux walk of the Full Demo POV / fixed-freeze user path for #165 on 2026-09-06. This VM can doctor Studio web and open the constructor door. It cannot paint Aspecto / Sonido / Extra / Avanzado, **Volver a preparar**, or the fixed-freeze subtract: those need a parsed partida plus a live full-demo plan, or a failed Full Demo output. This host has no jobs.db, no orchestrator, and no HLAE/CS2.

**Named gap:** `hlae_cs2_windows_studio`

This is not a Pass on Full Demo 16:9 and not a Pass on live 9:16. King's local CS2/HLAE 19/19 is King's personal Windows Studio path, not cloud VM proof. Do not invent HLAE Pass.

Commands, in order:

```sh
go build -o bin/zv ./cmd/zv
node .cursor/skills/verify-cliphub/control-cliphub.mjs launch --evidence .cursor/skills/verify-cliphub/artifacts/full-demo-pov-fixed-freeze
node .cursor/skills/verify-cliphub/control-cliphub.mjs doctor --json
./bin/zv verify doctor --dry-run --format json
./bin/zv verify prove --feature demo-completa --dry-run --format json
./bin/zv verify prove --feature demo-completa --format json
./bin/zv verify prove --feature full-demo-16x9-wait --format json
node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature inicio --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /clips --out .cursor/skills/verify-cliphub/artifacts/full-demo-pov-fixed-freeze/hub.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --path /clips --out .cursor/skills/verify-cliphub/artifacts/full-demo-pov-fixed-freeze/hub.png
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /clips/nueva?formato=full --out .cursor/skills/verify-cliphub/artifacts/full-demo-pov-fixed-freeze/full-door.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --path /clips/nueva?formato=full --out .cursor/skills/verify-cliphub/artifacts/full-demo-pov-fixed-freeze/full-door.png
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /clips/11111111-1111-4111-8111-111111111111/nuevo?formato=full --out .cursor/skills/verify-cliphub/artifacts/full-demo-pov-fixed-freeze/prepare-missing-job.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --path /clips/11111111-1111-4111-8111-111111111111/nuevo?formato=full --out .cursor/skills/verify-cliphub/artifacts/full-demo-pov-fixed-freeze/prepare-missing-job.png
node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature demo-completa --json
# same-origin Playwright on the launched instance (no route stubs): click Crear vídeo largo, wait produce settle
node --experimental-strip-types --test web/lib/full-demo-plan.test.ts web/lib/api/failure-reason.test.ts web/lib/full-demo.test.ts
go test ./internal/recapplan/ -count=1 -timeout 2m -run 'Freeze|POV|Legacy|Respawn|Stale|Approve'
node .cursor/skills/verify-cliphub/control-cliphub.mjs cleanup --json
```

`control-cliphub snapshot` of `/clips/<uuid>/nuevo?formato=full` waits for hub `Cargando partidas`, not produce `Cargando la partida`, so `prepare-missing-job.png` is the loading skeleton. A same-origin Playwright wait on the launched instance (not a route stub) then recorded the settled empty state in `prepare-settled.*` and the painted door after `Crear vídeo largo` in `full-door-after-click.*`.

`drive --feature demo-completa` refused: Cloud Linux is closed (`hlae_cs2_windows_studio`). `Crear Full Demo` was not clicked. Do not fake Pass.

`zv verify prove --feature demo-completa --dry-run` is catalog inspect only (`ok=true`, `user_path=gap`, no HTTP). Live prove fail-closes with gap `hlae_cs2_windows_studio`.

After cleanup the run file is gone, `127.0.0.1:4173` is down, and these files remain.

- `launch.json` is the isolated Next origin `http://127.0.0.1:4173` and pid.
- `doctor.json` has `web.ok=true`, `named_gap=hlae_cs2_windows_studio`, `capture_recertification=unavailable`, `zv.ok=false`.
- `drive.json` is `inicio`: empty hub, `Crear Short`, return via the rail.
- `hub.aria.txt` / `hub.png` show `¿Qué quieres crear?` and the `Crear vídeo largo` card (`/clips/nueva?formato=full`, Horizontal · 16:9). Footer names Windows Studio + CS2 + HLAE. Counts: `Volver a preparar` 0, freeze spin 0, keep-voice switch 0, `Crear Full Demo` 0.
- `full-door.aria.txt` / `full-door.png` and `full-door-after-click.*` are the constructor door: heading `Crea un vídeo largo`, `Vídeo largo 16:9` pressed, step `3 Preparar vídeo` not yet reached. Aspecto / freeze-fijo copy / `Volver a preparar` / `Crear Full Demo` are 0.
- `prepare-settled.aria.txt` / `prepare-settled.png` / `walk.json`: produce URL without a job settles on `Servicio local sin conexión`. Counts: Aspecto 0, Sonido 0, Extra 0, Avanzado 0, freeze spin 0, freeze-fijo copy 0, keep-voice switch 0, tick inicial 0, `Crear Full Demo` 0, `Volver a preparar` 0. `Crear Full Demo` was not clicked.
- `zv-prove-demo-completa.json` and `zv-prove-full-demo-16x9-wait.json` fail-close with gap `hlae_cs2_windows_studio` and `user_path=gap`.
- `unit-adjacent.json` / `unit-web-freeze-volver.tap` / `unit-go-recapplan.txt`: web 52/52 (fixed freeze migrate, replan gate, POV acquisition → prepare not retry) and recapplan Freeze|POV|Legacy|Respawn|Stale|Approve ok. Unit-adjacent only. Not HLAE Pass.
- `cleanup.json` records `evidence_exists=true`.

Limit: prepare UI / **Volver a preparar** / fixed-freeze subtract cannot be fully driven without Windows Studio plus a parsed job or a failed Full Demo output. Gap `hlae_cs2_windows_studio` stays closed.
