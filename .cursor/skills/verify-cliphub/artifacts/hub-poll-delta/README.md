# hub-poll-delta prove run

Cloud Linux walk of Hub/Partidas after the idle-poll ETag short-circuit (#163). Commands, in order:

```sh
node .cursor/skills/verify-cliphub/control-cliphub.mjs launch --evidence .cursor/skills/verify-cliphub/artifacts/hub-poll-delta
node .cursor/skills/verify-cliphub/control-cliphub.mjs doctor --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature inicio --json
# wait past IDLE_POLL_MS (10s)
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /clips --out .cursor/skills/verify-cliphub/artifacts/hub-poll-delta/hub-after-idle.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --path /clips --out .cursor/skills/verify-cliphub/artifacts/hub-poll-delta/hub-after-idle.png
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /clips?vista=clips --out .cursor/skills/verify-cliphub/artifacts/hub-poll-delta/clips-lens.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs cleanup --json
```

A throwaway in-memory `zv-orchestrator` on `127.0.0.1:8080` answered list GETs so the 304 path could be captured. No job was enqueued. No generate POST. After cleanup the run file is gone, `127.0.0.1:4173` is down, and these files remain.

- `launch.json` is the isolated Next origin `http://127.0.0.1:4173` and pid.
- `doctor.json` has `web.ok=true` and gap `hlae_cs2_windows_studio`. `capture_recertification` is `unavailable`.
- `drive.json` is the user path: hub, `Crear Short`, return via the rail.
- `hub.aria.txt` and `hub.png` are the empty hub after that return (`¿Qué quieres crear?`, rail `Clips y vídeos`).
- `hub-after-idle.aria.txt` and `hub-after-idle.png` are the same empty hub after a 12s wait past the 10s idle poll. The Partidas/hub shell did not collapse.
- `clips-lens.aria.txt` is `/clips?vista=clips` on the same empty first-run ready state.
- `idle-poll.json` plus `jobs-*.headers.txt` / `streams-*.headers.txt`: live `GET /api/jobs` and `GET /api/stream-jobs` against the in-memory orchestrator. First poll is 200 / `{"jobs":[]}` (11 B). Idle `If-None-Match` is 304 / 0 B. Roster-inlined 200 body shape is unchanged.
- `cleanup.json` records `evidence_exists=true`.

Studio web on this VM has no desktop session token, so the hub banner `La última actualización falló` is the honest BFF read. That is not a broken Partidas list. Empty-hub copy is the first-run proof.

This is not Full Demo Pass and not live 9:16 Pass. Gap `hlae_cs2_windows_studio` stays closed.
