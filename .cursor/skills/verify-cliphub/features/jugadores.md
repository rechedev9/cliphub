# Jugadores

Jugadores is the FACEIT sidebar. A user searches followed players, opens a profile, and sorts the rail. The FACEIT Download API is not an approved path.

## Sub-features

- `players-open` opens `/players` from the rail.
- `players-search` filters the followed rail by nickname.
- `players-profile` opens `Perfil de <nickname>`.
- `players-sort` changes `Ordenar jugadores`.

## How to get to it (user POV)

- Choose rail row `03 Jugadores`. The accessible name is `Jugadores FACEIT`.
- Run `control-cliphub.mjs goto --path /players`.

## Driving it with control-cliphub

Preconditions:

- Studio web is healthy at the launched origin.
- `control-cliphub.mjs doctor --json` reports `web.ok=true`.
- Followed players come from `/api/faceit/followed`. Without a key or orchestrator the page is an empty or offline FACEIT state. That state is still the user path.

- **Open players.** Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs goto --path /players`. The tab title is `ClipHub` (this route has no title segment). The heading is `Jugadores`. The rail link `Jugadores FACEIT` has `aria-current="page"`.
- **Settled empty host.** After `Cargando jugadores` hides, Cloud Linux without an orchestrator lands on `FACEIT no está configurado`, `Servicio local sin conexión`, or `Aún no sigues a nadie`. That state is the walk. The header follow form is textbox `Nick o URL de FACEIT` and submit `Seguir jugador` (disabled while FACEIT is unconfigured or offline).
- **Followed rail.** When players exist, the navigation name is `Jugadores seguidos`. Search uses textbox `Buscar jugador seguido`. Sort uses combobox `Ordenar jugadores`. Do not look for `Buscar jugador seguido` on an empty or offline host.
- **Proof.** Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --path /players --out .cursor/skills/verify-cliphub/artifacts/jugadores/rail.aria.txt` and `screenshot --path /players --out .cursor/skills/verify-cliphub/artifacts/jugadores/rail.png`. Both identify ClipHub and the settled Jugadores heading. Do not keep a snapshot that still shows `Cargando jugadores`.

## Gotchas

- This route has no `metadata` title segment. The tab stays `ClipHub`. Do not assert `Jugadores · ClipHub`.
- A snapshot taken while `Cargando jugadores` is visible is only the skeleton. `snapshot` and `goto` wait until that status is hidden.
- A FACEIT API key is optional. Production embeds one in `zv-orchestrator.exe`. Never put a key in this skill.
- `web/e2e/players.spec.ts` stubs FACEIT. Those rows are not live proof.
- Do not call the FACEIT Download API. Listing followed players and opening a profile is the walk.
