# Publicar vídeo largo

A finished Full Demo opens **Publicar** with English POV YouTube templates (up to five): selectable labels, editable title, description and tags, and copy. Shorts keep `Títulos recomendados`. This is publication copy, not capture.

## Sub-features

- `publicar-open` follows `Publicar` from a finished long video, or opens `/clips/<id>/publicar/<clipId>`.
- `publicar-templates` shows `Plantillas para vídeo largo` when the assistant returns `template` labels.
- `publicar-edit` lets the user change title, description, and comma-separated tags.
- `publicar-copy` copies one field or `Copiar todo`. Drafts stay local until copied.

## How to get to it (user POV)

- From Clips, open a finished vídeo largo and choose `Publicar`.
- Open `/clips/<job>/publicar/<videoId>` after a ready long render exists.
- Run `control-cliphub.mjs drive --feature publicar-video-largo`.

## Driving it with control-cliphub

Preconditions:

- Studio web is healthy at the launched origin.
- `control-cliphub.mjs doctor --json` reports `web.ok=true`.
- Linux doctor names `hlae_cs2_windows_studio`. Continue anyway. This feature does not recertify capture.
- Template UI needs a **ready** long video and a live publish-assistant response. An empty hub without an orchestrator is the honest Cloud Linux first-run state.

- **Open hub.** Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs goto --path /clips`. The rail link `Clips y vídeos` has `aria-current="page"`.
- **Finished long video.** When a ready Full Demo exists, a `Publicar` link is on that row. Choose it. The URL matches `/clips/<id>/publicar/<clipId>`. Wait until `Cargando el clip` is hidden.
- **Templates.** Ready long videos show heading `Plantillas para vídeo largo` (not `Títulos recomendados`). Buttons are named `Usar título recomendado: <title>` and may show labels `Bajas destacadas`, `POV clásico`, `Sesión de juego`, `Mapa protagonista`, `Duelo de la demo`. Fields are `Título`, `Descripción`, and `Etiquetas, separadas por comas`. `Copiar todo` copies the current draft.
- **Failed video.** Status `failed` must not show the waiting copy. The assistant alert is `No se pudo preparar la publicación porque el vídeo falló. Reintenta el render desde Clips.`
- **No finished video.** On an empty hub there is no `Publicar` row. `goto --path /clips/<uuid>/publicar/<clipId>` settles on `Clip no encontrado` or `No se pudo cargar el clip`. That is the walk. Do not stub `/api/demos/jobs` or `publish-assistant` to fake templates.
- **One-shot drive.** Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature publicar-video-largo`. Exit code `0` when the hub and Publicar route settle. `drive.json` records whether templates were reachable. Missing templates on Cloud Linux is a precondition, not a Pass on capture.
- **Proof.** Snapshot and screenshot the settled Publicar page (templates, failed alert, or missing clip). Both identify ClipHub.

## Gotchas

- `Publicar` is a child of a finished video, not a rail row. The catalog route stays `/clips`.
- Shorts on the same page show `Títulos recomendados` and a score. Do not treat that as the long-video template UI.
- Waiting copy (`La preparación para YouTube estará disponible…`) is only for queued, recording, composing, and review. Failed videos use the failed alert.
- `web/e2e/long-video-publish.spec.ts` stubs jobs and `publish-assistant`. Those screenshots are the presentation contract, not this skill's live proof.
- Opening Publicar is not Full Demo Pass and not live 9:16 Pass. Gap `hlae_cs2_windows_studio` stays closed on Cloud Linux.
