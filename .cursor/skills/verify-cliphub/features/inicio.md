# Clips y vídeos

The hub is the Studio home. A first-run user lands on `/clips`, sees three creation paths, and can open Short, vídeo largo, or stream import without an onboarding wizard.

## Sub-features

- `inicio-open` opens `/clips` from the root redirect and from the brand lockup.
- `inicio-empty` shows the empty hub `¿Qué quieres crear?` when no partidas exist.
- `inicio-rail` marks `Clips y vídeos` with `aria-current="page"`.
- `inicio-short-door` follows `Crear Short` to `/clips/nueva?formato=short`.
- `inicio-full-door` follows `Crear vídeo largo` to `/clips/nueva?formato=full`. That door is not Full Demo Pass.

## How to get to it (user POV)

- Open Studio. The root page redirects to `/clips`.
- Choose the brand lockup. Visible text is `ClipHub`; accessible name is `Ir a Clips y vídeos`.
- Choose rail row `01 Clips y vídeos`.
- Run `control-cliphub.mjs goto --path /clips`.

## Driving it with control-cliphub

Preconditions:

- Studio web is healthy at the launched origin.
- `control-cliphub.mjs doctor --json` reports `web.ok=true`.
- Linux doctor names `hlae_cs2_windows_studio`. Continue anyway. This feature does not need CS2.

- **Open hub.** Land on Clips. Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs goto --path /clips`. The document title is `Clips y vídeos · ClipHub`. The rail link `Clips y vídeos` has `aria-current="page"`.
- **Empty first-run.** Look for the empty region. Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --out .cursor/skills/verify-cliphub/artifacts/inicio/hub.aria.txt` from the repo root. Relative `--out` resolves from that root, not the evidence directory. The snapshot contains `¿Qué quieres crear?` or, when jobs exist, `Tus demos y vídeos`. The empty hub also shows the first-run guide `De la demo a tu vídeo` and a compact dropzone. That is still empty-hub, not a populated partida list.
- **Creation doors.** The empty hub offers three links. `Crear Short` href is `/clips/nueva?formato=short`. `Crear vídeo largo` href is `/clips/nueva?formato=full`. `Recortar un stream` href is `/streams`.
- **Short door.** Choose `Crear Short`. Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs click --role link --name "Crear Short"`. The URL matches `/clips/nueva?formato=short`. The tab title is `Cargar demo`. The heading is `Crea un Short`. The file chooser `Elegir demos de CS2` is enabled after hydration.
- **Return to hub.** Choose the rail row. Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs click --role link --name "Clips y vídeos" --within "[data-slot=sidebar]"`. The URL is `/clips` and `aria-current="page"` is back on that row.
- **One-shot drive.** Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs drive --feature inicio`. Exit code `0`. `drive.json` lists the routes above.
- **Proof.** Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs screenshot --out .cursor/skills/verify-cliphub/artifacts/inicio/hub.png`. The PNG shows the ClipHub shell and the hub copy.

## Gotchas

- `/` redirects to `/clips`. Assert the final URL, not the request URL.
- The brand lockup accessible name is `Ir a Clips y vídeos`. The rail row accessible name is `Clips y vídeos` (the `01` prefix is `aria-hidden`). Scope rail clicks with `--within "[data-slot=sidebar]"` so you do not hit the lockup.
- Empty hub copy is the honest state without an orchestrator. Do not stub `/api/demos/jobs` to fake partidas for this skill. The compact empty-hub dropzone `Elegir demos de CS2` stays enabled after hydration (it only stashes files for `/clips/nueva`). The upload-page chooser on `/clips/nueva` is the same control: `DemoDropzone` starts `interactive=false` and enables in `useEffect`. It is not gated on the orchestrator. A disabled snapshot is first paint, not the offline product state. `goto` / `snapshot` / `screenshot` wait until that input is enabled.
- First-run guide step 3 still says `descarga el MP4 desde Demos y vídeos`. That is live leftover copy (the rail is `Clips y vídeos`). Keep the snapshot; do not edit product copy from this skill.
- `/clips` first paints `Cargando partidas`. `snapshot` and `screenshot` wait until that status is hidden, then for the empty hub, `Tus demos y vídeos`, or the clips-lens heading `Tus vídeos de demos` on `/clips?vista=clips`. A capture taken too early is only the skeleton.
- Opening `Crear vídeo largo` is a constructor door. It is not HLAE capture and not Full Demo Pass.
- `web/e2e/clips-hub.spec.ts` stubs jobs. Those screenshots are presentation fixtures, not this skill's proof.
