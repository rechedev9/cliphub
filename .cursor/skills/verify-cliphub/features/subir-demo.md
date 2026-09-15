# Subir demo

Subir demo is the `/clips/nueva` flow. The user drops a `.dem`, picks a POV, and continues to Short settings. Preparing the video is not HLAE capture.

## Sub-features

- `upload-open` opens `/clips/nueva` from the hub and from `Cargar demo`.
- `upload-file` exposes `input[type="file"]` for a CS2 demo.
- `upload-roster` shows the player picker after a scan.
- `upload-continue` continues to Short settings with `Continuar al Short`.

## How to get to it (user POV)

- From the empty hub, choose `Crear Short`.
- Choose `Cargar demo` once partidas exist.
- Open `/clips/nueva?job=<id>` to resume a scanned roster.
- Run `control-cliphub.mjs goto --path /clips/nueva`.

## Driving it with control-cliphub

Preconditions:

- Studio web is healthy at the launched origin.
- `control-cliphub.mjs doctor --json` reports `web.ok=true`.
- A real scan POSTs `/api/demos/scan` and needs an orchestrator. Opening the dropzone does not.

- **Open upload.** Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs goto --path /clips/nueva`. The tab title is `Cargar demo` (the root ` · ClipHub` template does not appear on this route). The heading is `Crea un Short`.
- **File input.** The page has `input[type="file"]` named `Elegir demos de CS2`. Origin tabs use `aria-label="Origen de la demo"`.
- **Hub door.** From `/clips`, choose `Crear Short`. The URL is `/clips/nueva?formato=short`.
- **Proof.** Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --out .cursor/skills/verify-cliphub/artifacts/subir-demo/form.aria.txt` and `screenshot --out .cursor/skills/verify-cliphub/artifacts/subir-demo/form.png`. Both identify ClipHub and the upload page.

## Gotchas

- The live tab title is `Cargar demo`, not `Cargar demo · ClipHub`. `goto` still accepts the page because the brand lockup is present.
- Scan, roster, and `Continuar al Short` / `Continuar al vídeo largo` need orchestrator routes. Without them, stop after the dropzone is visible. Do not invent a roster. The accessible file chooser `Elegir demos de CS2` stays disabled while the local service is offline. The Short roster also offers `Preparar vídeo largo` as a destination switch.
- `?job=` resumes a scanned partida. It is not an upload.
- This path can open the Short constructor. It cannot recertify 9:16 capture on Cloud Linux.
