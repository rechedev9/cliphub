# Feature map

What a user can reach, by rail entry, and which edition shows it. `web/lib/edition.ts` decides the editions (`isPathAvailable`). When this table and that file disagree, the file is right: fix the table in the same change.

A change is verified in every edition that shows it. In Studio an agent changes state through `zv-mcp.exe` and only reads and clicks in the browser. In ClipMind the browser is the real client, so the journey runs through the UI ([ClipMind](clipmind.md)).

| Rail entry | Route | Studio | ClipMind | Feature file | Last proven |
|---|---|---|---|---|---|
| Demo completa | `/videos-largos` | yes | no | [full-demo.md](full-demo.md) | never |
| Short | `/shorts`, `/series/<id>` | yes | no | [short.md](short.md) | never |
| Clips de stream | `/streams` | yes | no | none yet | never |
| Jugadores FACEIT | `/jugadores/faceit` | yes | no | none yet | never |
| Mis vídeos | `/mis-videos` | yes | no | [short.md](short.md) | never |
| Consejos | `/ideas` | yes | no | none yet | never |
| Partidas | `/partidas`, `/partidas/<id>` | yes | yes | [partidas.md](partidas.md), [clipmind.md](clipmind.md) | Studio and ClipMind 2026-11-19 |
| Partidas, Públicas | `/partidas/pro` | yes | yes | none yet | never |
| Partidas, Series y veto | `/partidas/series` | yes | yes | none yet | never |
| Replay 2D | `/replay`, match tab `?tab=replay` | yes | yes | [replay.md](replay.md) | Studio 2026-11-19 |
| Buscador de patrones | `/patrones` | yes | yes | none yet | never |
| Estadísticas de equipo | `/equipos`, `/equipos/<id>` | yes | yes | none yet | never |
| Estadísticas de jugador | `/jugadores` | yes | yes | none yet | never |
| Ranking | `/ranking` | yes | yes | none yet | never |
| Editor de tácticas | `/tacticas/nueva` | yes | yes | none yet | never |
| Playbook | `/equipos?tab=playbook` | yes | yes | none yet | never |
| Lineups pro | `/lineups` | yes | yes | none yet | never |
| Simulación de veto | `/equipos?tab=veto` | yes | yes | none yet | never |
| Preparar rival | `/equipos?tab=preparar` | yes | yes | none yet | never |
| Almacenamiento | `/almacenamiento` | yes | no | none yet | never |
| Ajustes | `/settings` | yes | yes | [clipmind.md](clipmind.md) for "Cuenta ClipMind" | never |
| (no rail) account pages | `/crear-cuenta`, `/iniciar-sesion`, `/confirmar`, `/recuperar`, `/restablecer`, `/invitacion` | no | yes | [clipmind.md](clipmind.md) | `/iniciar-sesion` 2026-11-19 |
| (no rail) workspace and account | `/equipo`, `/cuenta` | no | yes | [clipmind.md](clipmind.md) | never |
| (no rail) browser import | `/importar` | no | yes | [clipmind.md](clipmind.md) | 2026-11-19 |

Each feature file has four sections: Sub-features, How to get to it (user POV), Driving it with the browser and zv-mcp, Gotchas. A run that proves a journey updates "Last proven" here and in the file. A journey that fails because the test data lacks a precondition stays failed, and the reason goes in that file's Gotchas.
