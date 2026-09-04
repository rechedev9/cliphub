# Multi-surface journeys

Cross-feature walks. A change that touches a structural flow must prove that flow. Unknown on a touched flow is a merge block.

## demo-to-shorts

Demo parser → 9:16 Shorts.

1. Land on **Inicio** (`/onboarding`) or **Subir demo** (`/upload`).
2. Accept a local `.dem`, pick a player, parse. Job appears in **Partidas**.
3. Start a 9:16 Short from the match. Stay on **Biblioteca** while capture runs ([shorts-9x16-wait](shorts-9x16-wait.md)).
4. Ready card: MP4 download + PREPARAR PUBLICACIÓN without a cover pick.

Cheap: map + unit tests on parse/plan/progress. Live: granted Windows Studio + HLAE + running `cs2.exe`. Hosted CI green is not this journey.

## full-demo-to-recap

Full Demo → 16:9 recap.

1. **Demo completa** (`/full-demo`). Contract rows: landscape 16:9, native HUD, comms, no music, no punch-in.
2. Pick a parsed match → `/full-demo/{jobId}`. Start the POV recap.
3. Watch Biblioteca ([full-demo-16x9-wait](full-demo-16x9-wait.md)) until the landscape file exists.
4. Ready card shows PARTIDA COMPLETA, not a vertical Short.

Fails closed on Cloud Linux (`hlae_cs2_windows_studio`). PR #120 overlay walk is still draft.

## stream-to-pack

VOD → persisted edit plan → render.

1. **Inicio** door **Corta un stream**, or rail **Clips de stream**.
2. Local MP4 or fetched VOD. Persist the plan before render. Crop/audio/fades/text/`music.volume` live on the plan.
3. Changing the plan invalidates the creative brief. A stream dry-run does not create its `--out` artifact.

## first-run-doors

Empty Studio.

1. Open Studio → EMPIEZA AQUÍ. `/` must not stay on a blank marketing root.
2. Three doors only: **Sube una demo**, **Corta un stream**, **Busca un jugador**.
3. Empty **Partidas** sends the user to Inicio (`EMPIEZA AQUÍ`), not a hidden upload shortcut.
4. Steam GC is an explicit user action. Never open it at startup.
