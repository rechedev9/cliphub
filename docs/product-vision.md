# ClipHub Studio: visión de producto (contenido + análisis)

Escrito el 2026-10-30. Este documento es el contexto de partida para rediseñar Studio como una suite.
Recoge la idea, lo que ya existe en el repo, un estudio del análisis de CS2 a día de hoy y un plan
por fases. Las decisiones abiertas están marcadas al final.

## 1. La idea en una frase

ClipHub Studio pasa a ser "el Salesforce de CS2": una sola aplicación de escritorio, sobria y densa
en datos, con dos áreas de trabajo que comparten la misma base de demos, partidas, equipos y
jugadores:

1. **Crear**: convertir demos y streams en vídeos (lo que Studio ya hace).
2. **Analizar**: análisis profesional de demos para analistas, coaches e IGLs: scouting de rivales,
   anti-strats, stratbook y métricas avanzadas.

**El producto que se vende es la aplicación de escritorio para Windows.** `web/` es solo su interfaz:
Electron la empaqueta en el instalador. No hay un producto web: abrir Studio en un navegador sirve
para probar la interfaz, no para venderlo. Cada decisión de diseño, de alcance y cada verificación
se piensa para el escritorio de Windows (instalador, actualizaciones, CS2 y HLAE en el mismo PC).

Lo que une las dos áreas es el registro común. Una partida es un objeto con su ficha, sus rondas,
sus jugadores y sus vídeos, igual que en un CRM un cliente tiene su ficha y sus listas relacionadas.

## 2. Punto de partida (estado del repo)

### Navegación actual
`web/lib/nav.ts` y `web/components/shell/app-sidebar.tsx` ya agrupan el menú:

| Grupo | Secciones |
|---|---|
| Crear | Shorts (`/shorts`), Vídeos largos (`/videos-largos`), Clips de stream (`/streams`) |
| Tus vídeos | Resultados (`/resultados`) |
| Análisis | Jugadores (`/players`, FACEIT), Táctica (`/tactical`), Anti-cheat (`/cheaters`) |
| Aprender | Algoritmos (`/algoritmos`, tendencias de YouTube) |
| Pie | Ajustes |

La estructura de dos áreas ya está insinuada. El trabajo es darle peso al área de análisis y cambiar
el lenguaje visual.

### Lo que ya existe para análisis
- `internal/tactical` + `internal/tacticalplan`: documento táctico determinista por demo. Clasifica
  cada ronda (T: `execute`, `default`, `split`, `fast`, `eco_rush`, `save`; CT: `hold`, `retake`,
  `aggression`, `stack`, `save`), economía (pistol, eco, force, full), sitios, timings,
  aperturas y tendencias agregadas con filtros (`Aggregate(Document, Filter) Tendencies`).
- `web/components/tactical/*`: replay 2D, timeline, lista de rondas, filtros, gráficos y tendencias.
- `internal/radarmap`: calibración de radares. `internal/lineups`: catálogo de lineups.
  `internal/utilityaudit`: auditoría de cada granada (tick de lanzamiento, postura, movimiento).
- `internal/voicecomms`: voz de los jugadores extraída de la demo.
- `internal/anticheat`: señales estadísticas de sospecha contra una distribución de referencia.
- `internal/faceit`: API de FACEIT (la clave ya va embebida en `zv-orchestrator.exe`).
- Parser: `github.com/markus-wa/demoinfocs-golang/v5` (Go). Base de datos local: SQLite
  (`modernc.org/sqlite`, `internal/store`).

### Lo que ya existe para contenido
- Vídeo largo de demo completa (Full Demo, `docs/full-demo-*.md`).
- Shorts de una demo (pipeline determinista en `internal/pipeline`, lista "Prepara tu Short").
- Clips de stream de Twitch, Kick y YouTube (`internal/vodfetch`, `internal/streamclips`).

### El problema visual
El shell actual es "futurista": fondo WebGL animado (`studio-ambient.tsx`), fuentes Chakra Petch y
Share Tech Mono, cian `#22d9ee` sobre navy. Funciona para un producto de creadores, pero choca con
una herramienta de trabajo para analistas, que pasan horas en tablas, filtros y comparativas.

## 3. Dirección de diseño: "Salesforce", no "nave espacial"

Qué tomamos de Salesforce (y de herramientas tipo Linear o Notion):

- **Objetos y fichas.** Cada cosa importante es un registro con su página: Partida, Serie, Equipo,
  Jugador, Demo, Jugada (strat), Informe, Vídeo. Una ficha tiene cabecera con datos clave, pestañas
  y listas relacionadas ("Partidas de este equipo", "Vídeos de esta partida").
- **Vistas de lista con filtros guardados.** Tablas densas, ordenables, con columnas configurables y
  vistas guardadas ("Pistols de Vitality en Mirage, últimos 3 meses").
- **Navegación estable a la izquierda**, dividida en las dos áreas. Nada se mueve ni se anima sin
  motivo.
- **Densidad y legibilidad.** Una fuente sans neutra con cifras tabulares, una sola tinta de acento,
  tema claro y oscuro, color reservado para significado (CT azul, T ámbar, ganada o perdida).

Cambios concretos propuestos:
1. Retirar el fondo WebGL y las gradientes ambientales del shell (`studio-ambient.tsx`,
   `shell-canvas.tsx`). Fondo plano.
2. Sustituir Chakra Petch y Share Tech Mono por una sans neutra y una mono neutra con cifras
   tabulares.
3. Paleta neutra (grises) con un acento. El cian puede quedarse como acento de marca, pero no como
   color de superficie.
4. Componentes base nuevos que ambas áreas usan: `RecordHeader`, `RecordTabs`, `RelatedList`,
   `DataTable` con vistas guardadas, `FilterBar`, `EmptyState`.

## 4. Arquitectura de información

Barra lateral propuesta:

```
CREAR
  Vídeo largo          (demo completa)
  Short                (de una demo)
  Clips de stream      (Twitch, Kick, YouTube)
  Mis vídeos           (resultados, antes "Resultados")
  Ideas                (antes "Algoritmos": tendencias de YouTube)

ANALIZAR
  Partidas             (biblioteca de demos parseadas, filtros, fichas)
  Equipos              (fichas de equipo, scouting)
  Jugadores            (fichas de jugador, FACEIT, roles)
  Stratbook            (jugadas guardadas, propias y del rival)
  Informes             (informes de scouting exportables)
  Integridad           (antes "Anti-cheat")

Ajustes
```

La **Biblioteca de demos** es la pieza común: una demo importada una vez sirve para crear un Short y
para alimentar el scouting de un equipo. Las dos áreas leen la misma tabla de partidas.

## 5. Modelo de datos común (los "objetos")

| Objeto | Clave | Qué contiene | Relaciones |
|---|---|---|---|
| Demo | hash del fichero | ruta, fuente (Valve, FACEIT, HLTV, scrim), build de CS2, estado del parseo | 1 Partida |
| Partida (mapa) | id | mapa, fecha, marcador, lados, documento táctico, posiciones | Demo, Serie, 2 Equipos, 10 Jugadores, Rondas, Vídeos |
| Serie | id | BO1/BO3/BO5, evento, veto si se conoce | Partidas, 2 Equipos |
| Equipo | id interno + alias | nombre, logo, plantilla por fecha | Jugadores, Partidas |
| Jugador | SteamID64 | nicks, FACEIT, rol inferido, equipo por fecha | Partidas, Rondas |
| Ronda | partida + número | economía, patrón T y CT, sitio, timings, ganador, motivo | Eventos |
| Evento | partida + tick | kill, daño, granada, plant, defuse, voz | Ronda, Jugador |
| Jugada (strat) | id | mapa, lado, tipo, granadas y posiciones, notas, rondas de ejemplo | Equipo, Rondas |
| Informe | id | rival, mapas, secciones, export | Equipo, Partidas, Jugadas |
| Vídeo | id | tipo (largo, Short, clip), render, publicación | Partida o stream |

Regla de diseño: **cada cifra enlaza a su evidencia** (partida, ronda y tick), y desde ahí se puede
abrir en el replay 2D o grabar el clip. Es lo que hace DAK Studio ("every stat links back to the
exact round and tick") y encaja con el `internal/tactical` actual, que ya es determinista.

## 6. Área Crear

Tres flujos principales, sin cambios de alcance. Lo que cambia es la forma:

| Flujo | Entrada | Salida | Ruta actual |
|---|---|---|---|
| Vídeo largo | una demo | vídeo de la partida completa con overlays y transiciones | `/videos-largos` |
| Short | una demo | Shorts verticales de las mejores jugadas | `/shorts` |
| Clips de stream | URL o VOD de Twitch, Kick o YouTube | clips editados | `/streams` |

Propuesta: los tres flujos siguen el mismo esqueleto de tres pasos (fuente, ajustes, resultado),
con la misma cabecera y el mismo panel de progreso, y todos dejan su resultado en "Mis vídeos".
Una demo elegida aquí entra en la Biblioteca y aparece también en Analizar.

## 7. Área Analizar: estudio del análisis en CS2 (octubre de 2026)

### 7.1 Cómo trabajan hoy los analistas y coaches
Fuentes de datos:
- **Demos oficiales de torneo** (HLTV las publica), **demos de FACEIT** (API de descargas),
  **demos de matchmaking/Premier** y, sobre todo en tier 1, **demos de scrims/práctica**.
- **Estadística pública**: HLTV (Rating 3.0 desde agosto de 2025, con "Round Swing" y ajuste por
  economía, revisado en octubre de 2025).

Flujo típico de preparación de un rival (anti-strat):
1. **Pistols**: son las rondas con patrones más fijos; se revisan primero.
2. **Defaults** de T y **setups** de CT en rondas de compra completa, vistos en 2D desde arriba.
3. **Executes**: qué granadas, en qué orden y con qué timings, por sitio.
4. **Ajustes mid-round, rotaciones, post-plant y retakes.**
5. **Economía**: cuándo fuerzan, cuándo salvan, qué hacen tras perder el pistol.
6. **Jugadores clave**: posiciones del AWP, agresiones tempranas, quién abre.
7. **Informe** para el coach y el IGL, y una **contra-estrategia** por mapa. Veto si es BO3.

Avisos que se repiten en las fuentes: una lectura de 3 a 5 demos es una lectura, no un libro; los
anti-strats rígidos fallan cuando el rival cambia; el análisis tiene que adaptarse a los cinco
jugadores propios, no solo a los datos.

Lo que los analistas dicen que les falta (artículo de HLTV "Why we're still waiting for
Counter-Strike's data revolution", 2024, con analistas de Liquid, GamerLegion y EG):
- Rendimiento ajustado por **rol/posición** y por **economía** (buy, eco, anti-eco).
- Duelos **justos frente a injustos**, no solo kills.
- **Intentos de apertura** por tipo de ronda.
- **Cantidad y calidad de utilidad.**
- **Tendencias de equipo**, retakes y post-plant, también para auto-scouting.
- Análisis **más rápido y personalizable** sin depender del proveedor para cada estadística nueva.
- Coste: un analista de datos bueno es caro, y pocas personas juntan código, estadística y juego.

### 7.2 Mercado: herramientas comerciales

| Herramienta | Enfoque | Precio (según fuentes, sin verificar) | Lo relevante |
|---|---|---|---|
| **Skybox EDGE** | Estándar en equipos pro | Gratis con 1 mes de retraso; Pro 5,99 €/mes; equipos 29,95 €, 349 €, 1.299 €/mes | Replay 2D, filtros por compra, stats por rol y economía, Playbook (detección de tácticas), Match Prep y simulador de veto en el tier alto, sync de voz. Permite ver scrims desde el lado del rival |
| **CS2.CAM** | Coaches y analistas | Individual 20 $/mes; equipos 80 a 120 $/mes | 2D y 3D, Strategic Board, heatmaps de movimiento, muerte, utilidad y timing, Playbook (2026), veto, sync de voz con IA, **grabación de rondas y POVs lanzando CS2** |
| **Noesis** | Equipos serios | ~10 €/mes (2025) | 2D con agregador de posiciones y utilidad sobre varias rondas |
| **RoundIQ** | Gratis para empezar | Freemium | 2D y 3D, anotaciones compartidas, Strat Builder, "Stuff Book" de lineups |
| **Metastack** | Workspace de equipo (sept. 2025, con Gooseman) | n/d | Repositorio táctico del equipo (sustituye a Google Drive), hub comunitario |
| **Leetify** | Jugador individual | Freemium | Aim, Utility y Positioning Rating con benchmarks, informes automáticos |
| **Scope.gg** | Jugador individual | Freemium | Desglose por jugador, highlights automáticos |
| **Refrag** | Práctica | Suscripción | Servidores de práctica, Bootcamp, 2D sencillo |
| **HLTV** | Estadística pública | Gratis | Rating 3.0, Target Rating por posición, base de partidas |

Conclusión del mercado: el análisis de equipo de verdad (Playbook, Match Prep, veto) está en tiers
de 350 a 1.300 €/mes y en la nube. Nadie combina **análisis local sin límites** con **producción de
vídeo**. CS2.CAM cobra su tier más alto por grabar POVs de rondas, algo que el recorder de Studio ya
hace.

### 7.3 Open source en GitHub (consultado el 2026-10-30)

Parsers y librerías:

| Repo | Estrellas | Lenguaje | Licencia | Uso para nosotros |
|---|---|---|---|---|
| [markus-wa/demoinfocs-golang](https://github.com/markus-wa/demoinfocs-golang) | 1.073 | Go | MIT | Ya es nuestro parser |
| [akiver/cs-demo-analyzer](https://github.com/akiver/cs-demo-analyzer) | 125 | Go | MIT | Motor de CS Demo Manager. API Go (`api.AnalyzeDemo`) con stats, posiciones y fuentes (FACEIT, ESL, 5E...). Referencia o dependencia para métricas por jugador |
| [LaihoE/demoparser](https://github.com/LaihoE/demoparser) (demoparser2) | 739 | Rust | MIT | Parser rápido con bindings Python/JS. Referencia |
| [pnxenopoulos/awpy](https://github.com/pnxenopoulos/awpy) | 627 | Python/Rust | MIT | Análisis y visualización, radares calibrados, ADR/KAST/rating. Referencia de fórmulas |
| [akiver/csgo-voice-extractor](https://github.com/akiver/csgo-voice-extractor) | 42 | Go | MIT | Extracción de voz (ya tenemos `internal/voicecomms`) |

Productos open source para inspirarse:

| Repo | Qué es | Ideas que tomar |
|---|---|---|
| [akiver/cs-demo-manager](https://github.com/akiver/cs-demo-manager) (2.019 estrellas, TS, MIT) | Gestor de demos de escritorio | Biblioteca de demos, stats, exportes. El referente de la Biblioteca |
| [Starfie1d1272/cs2-demo-analysis-kit](https://github.com/Starfie1d1272/cs2-demo-analysis-kit) (DAK Studio) | Escritorio local con 9 módulos: Library, Match workspace, Duel Lab, Utility Lab, Economy, Tournament Hub, Coach Workbench | "Query-first, no black boxes": cada cifra enlaza a ronda y tick. Muy cercano a lo que queremos |
| [Twoos123/cs2-meta-engine](https://github.com/Twoos123/cs2-meta-engine) | Plataforma web sobre demos de HLTV | Informe de scouting con defaults, utilidad, posiciones de AWP y export a PDF; comparar tu lineup con el pro más cercano; exportar `.cfg` de práctica |
| [EytleBB/CS-Scout](https://github.com/EytleBB/CS-Scout) | Scouting pre-partida (5E, Perfect World) | Descarga automática de demos del rival y replay de varias rondas superpuestas |
| [zenojunior/cs2d](https://github.com/zenojunior/cs2d) | Visor 2D en el navegador (Rust/WASM) | Heatmaps por lado/jugador/ronda con pisos, trayectorias de granadas, eficacia de flashes, telestrator, comentarios en el replay, export compacto |
| [sparkoo/csgo-2d-demo-viewer](https://github.com/sparkoo/csgo-2d-demo-viewer) | Visor 2D web | Referencia de replay |
| [Gary2005/cs-net](https://github.com/Gary2005/cs-net) | Transformer sobre demos | Probabilidad de victoria por tick y predicción de trayectorias |
| [DrEAmSs59/CS2-insight-agent](https://github.com/DrEAmSs59/CS2-insight-agent) (960 estrellas) | Suite china de creación + análisis: biblioteca, highlights, 2D, heatmaps, grabación por OBS, editor | **Competidor directo** de la idea de suite: confirma que la combinación tiene demanda |

Investigación académica (Peter Xenopoulos, autor de awpy):
- **ESTA** (2022): dataset de 1.558 partidas pro con benchmarks de probabilidad de victoria.
- **ggViz** (CHI Play 2022): buscar situaciones parecidas dibujando un estado en el radar, guiado
  por curvas de probabilidad de victoria y heatmaps.
- **Valuing Player Actions in CS:GO**: Win Probability Added (WPA) por acción, la misma idea que el
  Round Swing de HLTV Rating 3.0.
- **Optimal Team Economic Decisions**: modelo de victoria al inicio de ronda para decidir compras.

### 7.4 Riesgos del dominio
- **Valve cambia el formato sin aviso.** En julio de 2026 un mensaje nuevo (tipo 138) rompió el
  parseo de demos antiguas en herramientas de terceros; el 9 o 10 de septiembre de 2026 Valve
  cifró el chat dentro de las demos. Cada parche puede romper el parser: hace falta un test de
  regresión con demos de varias builds y diagnósticos claros (ya nombramos la build en los
  diagnósticos desde la 5.5.2).
- **Debate sobre demos de práctica.** En septiembre de 2026 snappi pidió a Valve que las demos de
  scrim solo muestren las acciones propias, porque Skybox permite estudiar al rival. Si Valve lo
  hace, el análisis de scrims ajenas desaparece; el de partidas oficiales no.
- **Acceso a datos.** HLTV no tiene API pública; descargar en masa puede violar sus términos. La
  vía segura es que el usuario importe sus demos (o las de FACEIT por API) y que Studio las
  procese en local.

## 8. Qué nos diferencia

1. **Local y sin límites.** Studio parsea en el PC del usuario: sin coste de servidor por demo, sin
   retraso de un mes como el free de Skybox, y las demos de scrim no salen del equipo.
2. **Del dato al vídeo en un clic.** Un hallazgo del análisis ("su execute de B con 4 smokes")
   se convierte en un clip grabado con el recorder, o en un vídeo de review para el equipo, o en un
   Short para redes. Eso no lo tiene ningún competidor de análisis.
3. **Determinista y explicable.** Los clasificadores de `internal/tactical` usan umbrales en
   segundos y unidades del mundo, a la vista. Cada cifra enlaza a la ronda y el tick.
4. **En español primero**, con un mercado hispanohablante poco atendido.

## 9. Plan por fases del área Analizar

Cada fase es usable por sí sola. El orden prioriza lo que reutiliza código existente.

**F0. Cimientos (rediseño + objetos)**
- Nuevo shell sobrio (sección 3) y la barra lateral de la sección 4.
- Biblioteca de demos común: importar varias demos a la vez, deduplicar por hash, detectar fuente,
  estado de parseo, etiquetas.
- Objetos Equipo y Jugador con fichas. Asignar equipos de una partida (por nombres de clan en la
  demo, con corrección manual).

**F1. Scouting de un rival (sobre varias demos)**
- Seleccionar un equipo y N partidas: agregar `Tendencies` de `tacticalplan` sobre varias demos
  (hoy agrega una).
- Secciones: pistols, defaults T, setups CT, executes por sitio con granadas y timings, economía
  (fuerza o salva tras perder), posiciones del AWP, aperturas por jugador.
- Cada fila enlaza a las rondas de ejemplo.
- Export del informe (PDF o HTML) para el coach.

**F2. Replay 2D multi-ronda**
- Superponer varias rondas filtradas en el mismo radar (lo que Noesis vende).
- Heatmaps de posición, muerte, utilidad y timing, por lado y piso.
- Telestrator y notas ancladas a un tick.

**F3. Stratbook**
- Guardar una ronda (propia o del rival) como Jugada: mapa, lado, tipo, granadas, posiciones,
  notas, rondas de ejemplo.
- Detección automática de jugadas repetidas (el "Playbook" de Skybox): agrupar rondas por patrón
  de posiciones y utilidad.
- Lineups: comparar las granadas de una ronda con `internal/lineups`.

**F4. Métricas avanzadas**
- Ajuste por economía de duelos y kills.
- Modelo de probabilidad de victoria por estado de ronda (jugadores vivos, equipo, bomba, tiempo)
  y WPA/Round Swing por jugador.
- Duelos justos e injustos, trades, calidad de utilidad (flash a enemigo, daño de HE y molotov).
- Rendimiento por rol y posición.

**F5. Veto y serie**
- Historial de mapas por equipo, picks y bans, rendimiento por lado. Simulador de veto BO1/BO3.

**F6. Del análisis al vídeo**
- Desde cualquier ronda o jugada: grabar POV, montar un vídeo de review, o enviar a Short.

Fuera de alcance por ahora: colaboración en la nube entre miembros del equipo (exige backend y
cuentas; se puede cubrir exportando informes y stratbooks como fichero).

## 10. Primeros pasos concretos

1. Cambiar `web/lib/nav.ts` y `app-sidebar.tsx` a las dos áreas de la sección 4 (renombres y
   redirecciones en `RETIRED_ROUTES`).
2. Retirar el fondo WebGL y cambiar tipografía y paleta en `web/app/globals.css`.
3. Crear los componentes base de ficha y tabla (sección 3).
4. Diseñar las tablas de Biblioteca, Equipo y Jugador en `internal/store` (migración SQLite).
5. Extender `tacticalplan.Aggregate` para varias partidas filtradas por equipo.

## 11. Decisiones abiertas

1. **Usuario principal del área Analizar**: coach o analista de equipo semipro/pro, o jugador de
   FACEIT que quiere mejorar. Cambia la densidad, el precio y la prioridad de F1 frente a métricas
   individuales tipo Leetify.
2. **Modelo de negocio**: si el análisis es de pago, cómo se cobra en una app local.
3. **Qué hacer con Integridad (anti-cheat) e Ideas (algoritmos)**: mantenerlas, moverlas o
   retirarlas.
4. **Tipografía y acento** del nuevo lenguaje visual.
5. **Fuentes de demos**: solo importación manual y FACEIT, o también ayuda para descargar demos
   de torneo.

## Fuentes

- HLTV, "Why we're still waiting for Counter-Strike's data revolution" (2024):
  https://www.hltv.org/news/40128/why-were-still-waiting-for-counter-strikes-data-revolution
- Pley.gg, snappi sobre demos de práctica (2026-09-25):
  https://pley.gg/cs2/it-ruined-innovation-snappi-calls-for-valve-to-restrict-practice-demo-visibility/
- Comparativa CS2.CAM y Skybox EDGE (precios y funciones, fuente de un competidor):
  https://cs2.cam/compare/skybox/article
- Dust2.us, Metastack: https://www.dust2.us/news/66054/gooseman-teams-up-with-former-heroic-coach-to-create-new-analytical-tool-for-cs2-teams
- Refrag, visores de demos online: https://refrag.gg/blog/how-to-view-cs2-demos-online/
- bo3.gg, analizadores gratuitos: https://bo3.gg/articles/best-free-cs2-demo-analyzers-online-tools-downloads-resources
- CS2 analyst role guide (marzo 2026): https://cs2hype.com/roles/analyst
- Anti-strats: https://csspot.org/wiki/anti-strats-in-cs2-how-teams-prepare-for-specific-opponents/
  y https://cs2guide.net/competitive-play/team-anti-strat-preparation/
- HLTV Rating 3.0: https://www.dust2.us/news/67183/hltv-releases-update-for-rating-30 y
  https://escorenews.com/en/csgo/article/71475-how-hltv-rating-3-0-formula-actually-works-round-swing-and-eco-adjustment-explained
- Leetify benchmarks: https://leetify.com/blog/cs2-benchmarks/
- Cambios de Valve en demos: https://hub.tcno.co/games/cs2/demo-fix/ y
  https://timesaver.gg/news/cs2-update-demo-chat-encryption-cache-fixes
- ESTA: https://arxiv.org/abs/2209.09861. ggViz: https://cs.paperswithcode.com/paper/ggviz-accelerating-large-scale-esports-game.
  Valuing Player Actions: https://arxiv.org/abs/2011.01324
- Repos de GitHub: enlazados en la sección 7.3 (estrellas consultadas con la API de GitHub el
  2026-10-30).
