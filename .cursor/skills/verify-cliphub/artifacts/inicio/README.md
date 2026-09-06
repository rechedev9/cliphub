# inicio prove run

Cloud Linux walk of `inicio` on 2026-09-06 after the two remaining Bugbot mediums. Commands, in order:

```sh
node .cursor/skills/verify-cliphub/control-cliphub.mjs launch --evidence .cursor/skills/verify-cliphub/artifacts/inicio
node .cursor/skills/verify-cliphub/control-cliphub.mjs doctor --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature inicio --json
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --out .cursor/skills/verify-cliphub/artifacts/inicio/hub.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --out .cursor/skills/verify-cliphub/artifacts/inicio/hub.png
node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /clips?vista=clips --out /tmp/clips-lens.aria.txt
node .cursor/skills/verify-cliphub/control-cliphub.mjs cleanup --json
```

A second `launch --port 4174` while 4173 was live and 4174 was already bound failed with the taken-port error and left pid 5228 recorded and answering `/clips`. After cleanup the run file is gone and `127.0.0.1:4173` is down. These files remain.

- `launch.json` is the isolated Next origin and pid.
- `doctor.json` has `web.ok=true` and gap `hlae_cs2_windows_studio`.
- `drive.json` is the user path: hub, `Crear Short`, return via the rail.
- `hub.aria.txt` and `hub.png` are the empty hub after that return. `/clips?vista=clips` snapshot finished on the same empty-hub ready state (no timeout). The clips-lens heading is accepted when that lens is populated; this VM has no jobs.
- `cleanup.json` records `evidence_exists=true`.

This is not Full Demo Pass. Capture recertification stayed `unavailable`.
