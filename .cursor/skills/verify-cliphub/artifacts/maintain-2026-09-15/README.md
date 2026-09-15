# maintain-2026-09-15 prove run

Cloud Linux weekday maintain of `.cursor/skills/verify-cliphub/` on 2026-09-15 against current main (after #187). Re-driven after Bugbot Medium findings on `8d837d99` so `/clips/nueva` and `/streams` wait for settled UI, not first-paint headings. The instance served `http://127.0.0.1:4173`. Capture is closed.

Commands, in order:

```sh
go build -o bin/zv ./cmd/zv
node .cursor/skills/verify-cliphub/control-cliphub.mjs launch --evidence .cursor/skills/verify-cliphub/artifacts/maintain-2026-09-15
node .cursor/skills/verify-cliphub/control-cliphub.mjs doctor --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature inicio --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs goto --path /clips/nueva --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs goto --path /clips/nueva?formato=full --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs click --path /clips --role link --name "Recortar un stream" --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs click --path /clips --role link --name "Crear vídeo largo" --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs click --path /clips --role link --name "Crear Short" --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /clips --out .cursor/skills/verify-cliphub/artifacts/maintain-2026-09-15/hub.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --path /clips --out .cursor/skills/verify-cliphub/artifacts/maintain-2026-09-15/hub.png
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /clips/nueva --out .cursor/skills/verify-cliphub/artifacts/maintain-2026-09-15/upload.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --path /clips/nueva --out .cursor/skills/verify-cliphub/artifacts/maintain-2026-09-15/upload.png
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /clips/nueva?formato=full --out .cursor/skills/verify-cliphub/artifacts/maintain-2026-09-15/full-door.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --path /clips/nueva?formato=full --out .cursor/skills/verify-cliphub/artifacts/maintain-2026-09-15/full-door.png
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /streams --out .cursor/skills/verify-cliphub/artifacts/maintain-2026-09-15/streams.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --path /streams --out .cursor/skills/verify-cliphub/artifacts/maintain-2026-09-15/streams.png
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /players --out .cursor/skills/verify-cliphub/artifacts/maintain-2026-09-15/players.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /clips/11111111-1111-4111-8111-111111111111/nuevo?formato=full --out .cursor/skills/verify-cliphub/artifacts/maintain-2026-09-15/prepare-missing-job.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature demo-completa --json
./bin/zv verify prove --feature demo-completa --format json
./bin/zv verify prove --feature full-demo-16x9-wait --format json
node .cursor/skills/verify-cliphub/control-cliphub.mjs cleanup --json
```

`drive --feature demo-completa` refused. Cloud Linux is closed (`hlae_cs2_windows_studio`). Do not fake Pass.

After cleanup the run file is gone and these files remain.

- `launch.json` is the isolated Next origin `http://127.0.0.1:4173` and pid.
- `doctor.json` has `web.ok=true`, gap `hlae_cs2_windows_studio`, `capture_recertification=unavailable`, `zv.ok=false`.
- `drive.json` is `inicio`: empty hub, `Crear Short` → `/clips/nueva?formato=short`, return via the rail.
- `hub.aria.txt` / `hub.png` show `¿Qué quieres crear?`, the three creation cards, and first-run guide `De la demo a tu vídeo`.
- `goto-nueva.json` identity is `wordmark` with tab title `Cargar demo`.
- `click-stream.json` → `/streams`. `click-full.json` → `/clips/nueva?formato=full`. `click-short.json` → `/clips/nueva?formato=short`.
- `upload.aria.txt` / `upload.png` heading `Crea un Short`. `Elegir demos de CS2` is **enabled** after hydration. A disabled chooser is first paint, not the offline product state. The input is not gated on the orchestrator.
- `full-door.aria.txt` heading `Crea un vídeo largo`. Format-bar `Vídeo largo 16:9` is pressed. Chooser enabled after hydration.
- `streams.aria.txt` / `streams.png` settled list: offline alert `El servicio de Clips de stream está offline` with `Reintentar`. No `Cargando streams`. URL field `Enlace del vídeo de Twitch, YouTube o Kick`. The page heading is not the ready signal.
- `players.aria.txt` / `players.png` settle on `Servicio local sin conexión` (no `Cargando jugadores`). Header form is `Nick o URL de FACEIT` / `Seguir jugador`, both disabled. Rail name is `Jugadores FACEIT`. Tab title is `ClipHub`.
- `prepare-missing-job.aria.txt` / `prepare-missing-job.png` settle on `Servicio local sin conexión` after `Cargando la partida` hides. `Crear Full Demo` was not clicked.
- `zv-prove-demo-completa.json` and `zv-prove-full-demo-16x9-wait.json` fail-close with gap `hlae_cs2_windows_studio` and `user_path=gap`.
- `cleanup.json` records `evidence_exists=true`.

This is not a Pass on Full Demo 16:9 and not a Pass on live 9:16. Gap `hlae_cs2_windows_studio` stays closed.
