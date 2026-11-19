# Replay 2D

The 2D replay of a match on the map radar: player positions, the round bar, events and the timeline. Both editions show it. It opens as the Replay tab of a match (`/partidas/<match_id>?tab=replay`), from the rail "Replay 2D" (`/replay`), and from tool links as a playlist (`/replay?r=<match_id>:<round>`).

Last proven: 2026-11-19 on Studio 5.25.0 (`83ce9fff`): the match tab and a round switch, as below.

## Sub-features

- Views: "Repetición", "Mapa de calor", "Recorridos", "Granadas".
- The round bar under the map, "MULTI RONDA", "Eventos", "Jugadores", the zoom and the speed buttons (0,25x to 4x).
- Team voice: "IMPORTAR VOZ" and "PLUGIN DE TEAMSPEAK" under the round bar.
- Playlists: `/replay/playlists`, and `/replay?p=<id>` for one playlist.

## How to get to it (user POV)

1. Open a match from [Partidas](partidas.md), then the tab "Replay". Or click "2D" on the match row.
2. Round 1 loads. The header reads "Ronda 1" and the radar shows both teams.
3. Click a round number in the bar. The header, the score box and the player lists switch to that round.

## Driving it with the browser and zv-mcp

- Click `role=tab` "Replay" and wait until the URL has `tab=replay` and a `canvas` has a width.
- Each round is `button[aria-label^="Ronda <n>:"]`. Its label tells the result, such as `Ronda 13: gana <equipo B> (CT), bomba desactivada, 4:9 <equipo A> - <equipo B>`.
- The selected round has `aria-current="true"`.

Proof: click `button[aria-label^="Ronda 13:"]`. That button gets `aria-current="true"`, the header reads "Ronda 13", and the score box reads the score before that round (8 and 4 after a 4-8 first half). Screenshot before and after. The same rounds come from the MCP with `replay_rounds` or `get_round`.

## Gotchas

- The score box lists the team on the CT side first, so it swaps order at the half.
- The header shows a warnings chip ("2 avisos" on the proven demo). Its content was not checked.
- A screenshot taken during a transition can show the old round still highlighted. Read `aria-current`, not the colour.
