# ClipHub verification map

This directory is the maintained source for verifying user-facing ClipHub Studio behavior. Read the index before driving the app, then use the matching feature file as the recipe.

Feature ids match `internal/verify/catalog.go` and `zv verify features`. Cheap catalog inspect is not a user-path walk.

## Baseline preconditions

- Launch Studio web with `node .cursor/skills/verify-cliphub/control-cliphub.mjs launch`.
- Default origin is `http://127.0.0.1:4173`. Do not reuse landing (`3100`) or a stray `next dev` (`3000`).
- Run `control-cliphub.mjs doctor --json` and require `web.ok=true`.
- On Linux, require gap id `hlae_cs2_windows_studio` in `zv.gaps`. That host cannot recertify capture.
- Drive only the instance this run started. The PID is in `.cursor/skills/verify-cliphub/.run/state.json`.
- Put `./bin/zv` on `PATH` after `go build -o bin/zv ./cmd/zv`, or call `go run ./cmd/zv`.

## Driving conventions

- Start every recipe from `/clips` unless the feature file says otherwise.
- Prefer ARIA roles, accessible names, and `[data-slot="sidebar"]` over CSS position or click coordinates.
- Treat every command as literal. Keep quoted names and flags unchanged.
- Run browser actions through `control-cliphub.mjs`.
- Run Studio/HLAE inspect through `./bin/zv verify`.
- Restore nothing on the empty first-run hub. Do not delete proof artifacts during cleanup.

## Proof and skip reporting

- Capture the user action and the resulting state, not only the final screen.
- UI proof includes an ARIA snapshot and a screenshot with the ClipHub title visible.
- Mutation proof includes a second read of the stored value. This map's first recipes are read-only walks.
- Record the feature id and entry point with every artifact.
- Report an unreachable path with the attempted command and the unmet precondition.
- Do not report `demo-completa` as verified through the empty hub. That feature needs Windows Studio, HLAE, and running CS2.

## Feature entry contract

Each feature file starts with an H1 title and one paragraph describing the user-visible behavior. It then uses exactly four H2 sections in this order.

1. `Sub-features` lists short IDs with one line for each behavior.
2. `How to get to it (user POV)` lists every user entry point.
3. `Driving it with control-cliphub` starts with `Preconditions:` and uses labeled bullets that pair each user action with an exact command and observable result.
4. `Gotchas` lists traps that can waste or invalidate a verification run.

## Features

- [Clips y vídeos](./inicio.md) is the first-run hub. Drive this one on Cloud Linux.
- [Clips de stream](./clips-de-stream.md) imports a Twitch, YouTube, or Kick URL, or an MP4.
- [Jugadores](./jugadores.md) is the FACEIT sidebar. Download API stays unapproved.
- [Subir demo](./subir-demo.md) is the `/clips/nueva` roster flow. Preparing a Short is not capture.
- [Demo completa](./demo-completa.md) is Full Demo 16:9. Closed on Cloud Linux.
