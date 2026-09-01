# ClipHub Studio feature map

User-POV routes the verification lever can name. Dump with:

```text
./bin/zv verify features --format json
```

| id | Title | Route | User-path on Cloud Linux |
| --- | --- | --- | --- |
| `inicio` | Inicio | `/onboarding` | cheap proof only |
| `partidas` | Partidas | `/matches` | cheap proof only |
| `subir-demo` | Subir demo | `/upload` | cheap proof only |
| `demo-completa` | Demo completa | `/full-demo` | **gap** — HLAE/CS2 recap |
| `cheaterdetect` | CheaterDetect | `/cheaters` | cheap proof only |
| `tactica` | Táctica | `/tactical` | cheap proof only |
| `jugadores` | Jugadores | `/players` | cheap proof only |
| `clips-de-stream` | Clips de stream | `/streams` | cheap proof only |
| `biblioteca` | Biblioteca cards | `/videos` | cheap proof only |
| `shorts-9x16-wait` | 9:16 Shorts wait | `/videos` | **gap** — live overlay |
| `full-demo-16x9-wait` | Full Demo 16:9 wait | `/videos` | **gap** — live overlay |

Sidebar order lives in `web/lib/nav.ts`. `/` redirects to `/onboarding`.
