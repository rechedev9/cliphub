# Clips de stream

Stream import turns a Twitch, YouTube, or Kick URL, or a local MP4, into a project the user trims into Shorts. Stream clips do not need CS2 or HLAE.

## Sub-features

- `stream-open` opens `/streams` from the rail and from the hub card `Recortar un stream`.
- `stream-url` imports a trimmed URL and optional project name.
- `stream-file` sends an MP4 through `Seleccionar archivo MP4`.
- `stream-error` keeps URL errors beside the field and clears them on edit.

## How to get to it (user POV)

- Choose rail row `02 Clips de stream`.
- From the empty hub, choose `Recortar un stream`.
- Run `control-cliphub.mjs goto --path /streams`.

## Driving it with control-cliphub

Preconditions:

- Studio web is healthy at the launched origin.
- `control-cliphub.mjs doctor --json` reports `web.ok=true`.
- No live orchestrator is required to open the form. A real import POST needs `/api/streams`.

- **Rail entry.** Choose `Clips de stream`. Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs goto --path /streams`. The title is `Clips de stream · ClipHub`. The rail link `Clips de stream` has `aria-current="page"`.
- **Hub entry.** From `/clips`, choose `Recortar un stream`. Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs click --role link --name "Recortar un stream"`. The URL is `/streams`.
- **URL field.** The textbox name is `Enlace del vídeo de Twitch, YouTube o Kick`. The optional title name is `Nombre del proyecto`. The submit name is `Importar vídeo`.
- **Empty submit.** Choose `Importar vídeo` with an empty URL. The URL field is `aria-invalid` and `#stream-url-error` contains `Pega una URL`.
- **Proof.** Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --out .cursor/skills/verify-cliphub/artifacts/clips-de-stream/form.aria.txt` and `screenshot --out .cursor/skills/verify-cliphub/artifacts/clips-de-stream/form.png`. Both identify ClipHub and the import form.

## Gotchas

- A successful import navigates to `/streams/<id>` and POSTs `/api/streams`. Without an orchestrator the POST fails. Opening the form is still a valid walk. Creating a project is not proven until the POST returns the new id.
- File import uses a file chooser. Do not click-xy the dropzone.
- Stream clips are not Full Demo. Do not attach the HLAE gap to a failed URL validation.
