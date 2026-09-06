# Jugadores

Jugadores is the FACEIT sidebar. A user searches followed players, opens a profile, and sorts the rail. The FACEIT Download API is not an approved path.

## Sub-features

- `players-open` opens `/players` from the rail.
- `players-search` filters the followed rail by nickname.
- `players-profile` opens `Perfil de <nickname>`.
- `players-sort` changes `Ordenar jugadores`.

## How to get to it (user POV)

- Choose rail row `03 Jugadores`.
- Run `control-cliphub.mjs goto --path /players`.

## Driving it with control-cliphub

Preconditions:

- Studio web is healthy at the launched origin.
- `control-cliphub.mjs doctor --json` reports `web.ok=true`.
- Followed players come from `/api/faceit/followed`. Without a key or orchestrator the page is an empty or offline FACEIT state. That state is still the user path.

- **Open players.** Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs goto --path /players`. The title is `Jugadores · ClipHub` or the live heading the page serves. The rail link `Jugadores` has `aria-current="page"`.
- **Followed rail.** When players exist, the navigation name is `Jugadores seguidos`. Search uses textbox `Buscar jugador seguido`. Sort uses combobox `Ordenar jugadores`.
- **Proof.** Run `node .cursor/skills/verify-cliphub/control-cliphub.mjs snapshot --out .cursor/skills/verify-cliphub/artifacts/jugadores/rail.aria.txt` and `screenshot --out .cursor/skills/verify-cliphub/artifacts/jugadores/rail.png`. Both identify ClipHub and the Jugadores rail.

## Gotchas

- A FACEIT API key is optional. Production embeds one in `zv-orchestrator.exe`. Never put a key in this skill.
- `web/e2e/players.spec.ts` stubs FACEIT. Those rows are not live proof.
- Do not call the FACEIT Download API. Listing followed players and opening a profile is the walk.
