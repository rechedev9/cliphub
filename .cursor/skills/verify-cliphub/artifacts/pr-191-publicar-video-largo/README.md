# pr-191-publicar-video-largo prove run

Cloud Linux babysit of PR #191 (`feat/long-video-publish-templates`) on 2026-09-16. The instance served `http://127.0.0.1:4173`. Capture is closed.

Commands, in order:

```sh
go build -o bin/zv ./cmd/zv
node .cursor/skills/verify-cliphub/control-cliphub.mjs launch --evidence .cursor/skills/verify-cliphub/artifacts/pr-191-publicar-video-largo
node .cursor/skills/verify-cliphub/control-cliphub.mjs doctor --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature publicar-video-largo --json
./bin/zv verify prove --feature publicar-video-largo --format json
./bin/zv verify prove --feature demo-completa --format json
./bin/zv verify prove --feature full-demo-16x9-wait --format json
node .cursor/skills/verify-cliphub/control-cliphub.mjs cleanup --json
```

`drive --feature publicar-video-largo` opened the empty hub (no `Publicar` row), then `/clips/<uuid>/publicar/<clipId>` settled on `Clip no encontrado`. `templates_reachable=false`. That is the honest Cloud Linux path: no finished long video, no live publish-assistant. Do not stub the assistant to fake `Plantillas para vídeo largo`.

`zv verify prove --feature demo-completa` and `full-demo-16x9-wait` fail-close with gap `hlae_cs2_windows_studio`. This PR does not claim Full Demo 16:9 capture or live 9:16 Pass.

After cleanup the run file is gone and these files remain.

- `launch.json` is the isolated Next origin `http://127.0.0.1:4173` and pid.
- `doctor.json` has `web.ok=true`, gap `hlae_cs2_windows_studio`, `capture_recertification=unavailable`, `zv.ok=false`.
- `drive.json` is `publicar-video-largo`: empty hub, zero Publicar links, missing-clip Publicar page.
- `hub.aria.txt` / `hub.png` show `¿Qué quieres crear?` and ClipHub identity. Offline banner `Servicio local offline`.
- `publicar.aria.txt` / `publicar.png` show heading `Clip no encontrado` and breadcrumb `Clips y vídeos publicar`.
- `zv-prove-publicar-video-largo.json` is cheap catalog inspect (`ok=true`, `user_path=unproven`). Not a template UI Pass.
- `zv-prove-demo-completa.json` and `zv-prove-full-demo-16x9-wait.json` fail-close with gap `hlae_cs2_windows_studio` and `user_path=gap`.
- `cleanup.json` records `evidence_exists=true`.

Presentation contract (fixture API, not this skill's live proof):

```sh
E2E_PORT=4174 pnpm --dir web exec playwright test e2e/long-video-publish.spec.ts
```

3 passed. `presentation/long-video-publish-390.png` and `presentation/long-video-publish-1440.png` show `Plantillas para vídeo largo` with selectable templates. The failed-video spec asserts the failed alert and no waiting copy. Those routes are stubbed. Not a live orchestrator walk.

This is not a Pass on Full Demo 16:9 and not a Pass on live 9:16. Gap `hlae_cs2_windows_studio` stays closed.
