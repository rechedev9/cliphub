Actualizado el 21/10/2026. Combina capturas del tráfico real de YouTube y Google con fuentes públicas, revisadas una a una.

**Leyenda de evidencia (se usa en toda la guía):**

| Marca | Nivel | Qué significa |
|---|---|---|
| **[R]** | 1. Observado con REA en el cliente | Tráfico real capturado con REA, una herramienta de ingeniería inversa (navegador headless, sin sesión, región ES). |
| **[R*]** | 1. Leído con REA en el código del cliente | Código del cliente (base.js, kevlar.js, xjs) exportado con REA y leído en estático, sin ejecutar. |
| **[C]** | 2. Confirmado | Google/YouTube (blogs, Help, Creator Insider), papers de Google/YouTube, juicio US v. Google o la filtración de Content Warehouse (autenticidad confirmada por Google). |
| **[I]** | 3. Inferencia de la industria | Estudios de terceros, prensa, interpretación propia. |
| *(debilitado)* | | Al revisar las fuentes aparecieron matices o contradicciones; se usa la versión corregida. |
| [no verificable] | | Solo hay una fuente indirecta. |

**Límite clave:** REA solo ve el lado cliente: qué señales se recogen y se envían. No ve cómo los servidores las ponderan, combinan o umbralizan. Todo lo que diga "el ranking usa X con peso Y" viene de nivel 2 o 3, nunca de REA.

---

## 1. Resumen en 10 líneas

1. YouTube no "empuja" un vídeo a N personas: cada espectador recibe su propio ranking; lo viral es la suma de muchas victorias individuales [C].
2. El sistema es un embudo: candidatos (millones a cientos, por co-visionado, embeddings, contenido nuevo y, desde 2025, recuperación con LLM) y luego un ranking multiobjetivo [C].
3. El objetivo declarado es satisfacción a largo plazo, no clics: Test & Compare elige miniatura por cuota de tiempo de visionado, no por CTR [C].
4. El cliente mide cada impresión visible, cada clic con su superficie, cada segundo visto por segmentos, silencios, saltos, pausas, el punto exacto de salida y el siguiente vídeo [R].
5. En Shorts, un swipe se reporta en milisegundos con el segundo exacto de salida; los bucles aparecen como vueltas en los segmentos [R]. YouTube entrena con los skips como señal negativa [C].
6. Un vídeo nuevo se prueba con públicos diversos (suscriptores y no suscriptores) desde la primera hora, pero "las primeras 24 h lo deciden todo" es un mito que YouTube niega [C].
7. En Google Search los clics importan (Navboost: 13 meses de memoria de clics por consulta), pero de forma corregida por posición y con calidad del clic, no CTR bruto [C].
8. YouTube domina los carruseles de vídeo de Google (estudios de 2020) y es el dominio más citado en AI Overviews (2026) [I].
9. Cambios críticos para ClipHub: desde el 1 de octubre de 2026 Shorts reduce el alcance de canales que agregan o resuben clips ajenos; un Short de más de 60 s con cualquier reclamación de Content ID queda bloqueado globalmente [C].
10. Estrategia: abrir con la acción, cumplir la promesa del título, finales que enlacen, narrativa propia por encima de plantillas idénticas, y publicar rápido alrededor de los Majors.

---

## 2. El ciclo end to end: de la subida al vídeo viral

```
SUBIDA
  |
  v
[1 Clasificación de formato]  vertical/cuadrado <= 3 min -> Shorts (desde 15/10/2024)
  |                           16:9 o > 3 min -> vídeo largo                       [C]
  v
[2 Comprensión del contenido] metadatos, VideoObject para Google, Semantic IDs
  |                           (códigos aprendidos del contenido) para cold start  [C]
  v
[3 Generación de candidatos]  millones -> cientos
  |   co-visionado, two-tower, secuencias, embudo de contenido nuevo,
  |   recuperación generativa con LLM (PLUM, 2025)                               [C]
  v
[4 Ranking multiobjetivo]     P(clic), tiempo de visionado, likes, descartes,
  |   encuestas previstas; corrección de sesgo de posición                       [C]
  v
[5 Exploración]               impresiones de prueba a públicos diversos,
  |   también no suscriptores, desde la primera hora                             [C]
  v
[6 Superficie]  Home | Siguiente (sugeridos) | Búsqueda | feed de Shorts |
  |             Suscripciones | notificaciones | Google: carrusel, Discover, AIO
  v
[7 Telemetría del cliente]  impresión visible -> clic (superficie, consulta) ->
  |   playback -> segmentos st/et cada 10/20/30/40 s -> pausa/seek/mute ->
  |   salida (final=1, cmt) -> siguiente vídeo (sdetail=rv:<id>) -> QoE, atestación [R]
  v
[8 Aprendizaje]  datos frescos por vídeo con esos espectadores; afinidad
  |   espectador-canal; negativos (ignorado, descartado, skip, "No me interesa") [C]
  v
[9 Expansión]  si gana, entra como candidato para espectadores parecidos
     (co-visionado, temas). Si un vídeo es "el siguiente natural" de un
     clúster, se propaga. Puede despegar semanas después (evergreen).            [C]
```

Notas por etapa:

- **Candidatos.** Arquitectura de dos etapas (Covington 2016) mantenida en papers 2019 a 2025 [C]. Entrenan con visionados de todo YouTube, incluidos embeds, así que el tráfico externo siembra co-visionado [C]. Desde 2025, PLUM (LLM de 900M parámetros activos, unos 4.200M totales) añadido al pool de candidatos: Shorts CTR del panel +4,96 %, vistas +0,39 %, satisfacción +0,39 %; largo CTR +0,76 %, vistas +0,80 %; reparte impresiones entre 2,6 veces (largo) y 13,24 veces (Shorts) más vídeos distintos [C, corregido]. Semantic IDs mejoran la generalización a vídeos nuevos [C].
- **Ranking.** Zhao 2019: MMoE predice objetivos de engagement (clics, visionados, tiempo) y de satisfacción (likes, descartes, valoraciones), combinados con pesos ajustados a mano [C]. Covington 2016: predice tiempo de visionado esperado por impresión porque ordenar por CTR premia el clickbait [C].
- **Exploración.** REINFORCE off-policy en candidatos [C]. Un paper de Google sobre contenido nuevo (KDD 2023, coautor Cristos Goodrow, plataforma no nombrada) describe un canal reservado para contenido nuevo [C]. El paper de valor a largo plazo de la exploración (WSDM 2024) describe una plataforma de vídeo corto con miles de millones de usuarios; asociarla a Shorts es inferencia [C, debilitado].
- **Distribución "pull".** Beaupré (YouTube, septiembre de 2026): las recomendaciones se "extraen para cada espectador", el sistema se centra más en vídeos individuales que en medias de canal, aprende por separado para suscriptores y no suscriptores, y "no decide solo con esos momentos iniciales" [C].
- **Frescura.** Covington modela la edad del ejemplo para captar el pico tras la subida [C]. No prueba una ventana dura de 24 h.

---

## 3. Qué mide el cliente (señales observadas con REA)

Capturas: navegador headless, sin sesión, perfil nuevo, ES. Vídeos: búsqueda "cs2 highlights" y "cs2 shorts", clic, mute, seeks, pausa, clic en relacionado, swipes.

### 3.1 Vídeo largo (página /watch)

| Endpoint | Campo | Qué codifica | Cuándo se envía | Nivel |
|---|---|---|---|---|
| `ytInitialPlayerResponse.playbackTracking` | URLs de stats, `ei`, `plid`, flush `[10,20,30]` y `40` | El servidor vincula la sesión a un id de evento y de reproducción antes de reproducir | Carga de página | [R] |
| `/youtubei/v1/log_event` `visualElementShown` | `trackingParams`, `viewData` (x, y, ancho, alto) | Impresión real en viewport, con geometría | Al entrar en pantalla | [R] |
| `log_event` `visualElementGestured` | `GENERIC_CLICK`, `KEY_PRESS` con `seekData` (desde, hasta, `SEEK_FORWARD_10S/5S`) | Clic en un elemento concreto; seeks explícitos | Al interactuar | [R] |
| `log_event` hover | `durationHoveredMs` (solo entre 500 ms y 600.000 ms), `isMovingThumbnail` | Tiempo de hover y vista previa en el feed | Al pasar el ratón | [R*] |
| `/youtubei/v1/get_watch` | `clickTrackingParams` | Contiene la superficie ("related"/"search"), el texto de la consulta e ids tipo VE; la semántica de la posición no está confirmada | Al hacer clic (sustituye a /player + /next) | [R] *(debilitado)* |
| `/api/stats/playback` | `feature` (search/related), `sourceid` (ys/yw), `sdetail=rv:<vídeo anterior>`, `referrer`, `lact`, `muted`, `volume`, `len` | Inicio de visionado con fuente de tráfico y vídeo semilla | Solo cuando arranca la reproducción real (cmt ≈ 0,02) | [R] |
| `/api/stats/watchtime` | `st`/`et` (listas de segmentos), `muted`, `volume`, `state`, `cmt`, `rt`, `rti`/`rtn`, `lact`, `final=1` | Segundos vistos por segmento; huecos = saltos; mute parte el segmento | rt ~10/20/30 s, luego cada 40 s; al pausar, reanudar y salir | [R] |
| `watchtime` (código) | `vis` (1 minimizado, 2 pantalla completa, 3 segundo plano, 5 inline, 7 PiP, 8 vista previa, 10 teatro) | Estado de visibilidad por segmento | En cada segmento | [R*] |
| `watchtime` (código) | `subscribed`, `autonav`, `list` | Suscrito, autoplay siguiente, playlist | Solo en código, no observados | [R*] *(debilitado)* |
| Salida a relacionado | `final=1`, `cmt` de salida, luego `sdetail=rv:<id>` en el siguiente | Punto exacto de abandono y siguiente vídeo elegido, enlazados | Al cambiar de vídeo | [R] |
| `/api/stats/qoe` | `vps` (estados), `bh` (buffer), `bwe`, `view`, `vis`, `df`, errores | Calidad de reproducción; comparte cpn/docid con watchtime, así que el servidor puede separar abandono técnico de abandono por contenido | Serie continua | [R] |
| `/api/stats/atr`, `poToken`, `/ptracking` | blob de atestación, token de integridad | Comprobación de cliente (anti bots, lectura apoyada en el código) | ~1 s tras empezar | [R] |
| `log_event` `foregroundHeartbeat` | `firstActivityMs`, `lastEventDeltaMs` | Atención en primer plano | Periódico | [R] |
| `lact` en todos los pings | ms desde la última entrada del usuario | Activo frente a pestaña olvidada; alimenta el aviso "¿Sigues ahí?" | Cada ping | [R] |
| `delay` / `dtm=1` | umbral por vídeo fijado por el servidor | Ligado a telemetría de delayplay y remarketing (anuncios), no es un umbral de ranking documentado | Al superar el umbral | [R*] *(debilitado, corregido)* |
| `/youtubei/v1/feedback` | `feedbackTokens` opacos | "No me interesa" y "No recomendar canal" llegan como menús del servidor con su token. Sin sesión, los menús no los ofrecían; hubo una llamada automática con un token opaco de origen no determinado | Al usar el menú | [R] *(debilitado)* |
| like, dislike, subscribe, share, playlist | `likeParams`, `botguardResponse`, `clientFeature` | Acciones explícitas con contexto de superficie | En código, no ejercitado (sin sesión) | [R*] *(debilitado)* |
| Encuestas | `inlineSurveyRenderer`, `DislikeSurveyForm`, HaTS | Satisfacción explícita por vídeo | En código; ninguna apareció | [R*] *(debilitado)* |
| Anuncios | `el=adunit`, `host_cpn`, `content_v` | Tiempo de anuncio separado pero enlazado al contenido | Pre-roll | [R] |

Ejemplo real (diL4UcaWM7E): `st=4.474,7.988,13.029 et=7.929,12.942,14.474 muted=0,1,0` tras mute y unmute; tras +10 s y +5 s, huecos 16,9 a 27,0 y 30,8 a 35,9 [R].

### 3.2 Shorts

| Endpoint | Campo | Qué codifica | Cuándo se envía | Nivel |
|---|---|---|---|---|
| `/youtubei/v1/player` + `reel/reel_item_watch` + `reel/reel_watch_sequence` | `clickTrackingParams` | Consulta ("cs2 shorts") y posición del resultado clicado | Al entrar a un Short | [R] |
| `reel_item_watch` `params` | varint 20 (entrada desde búsqueda) frente a 12 (swipes en el feed) | Etiqueta tipo punto de entrada; semántica no confirmada | Cada Short | [R] *(debilitado, corregido)* |
| `sequenceParams` | consulta, locale, dos vectores opacos de 128 dimensiones, 29 ids de vídeo | Semilla de la sesión; significado de los vectores desconocido | Al abrir la secuencia | [R] *(debilitado, corregido)* |
| Respuesta de `reel_watch_sequence` | 26 elementos prefetch + `continuation` con historial servido | Lotes de unos 26 Shorts; con qué frecuencia se reordena es desconocido | Al abrir; sin nueva petición en swipes 1 a 3 | [R] *(debilitado: obtenida por replay)* |
| `/api/stats/watchtime` `el=shortspage` | `st`/`et`, `cmt`, `len`, `lact`, `sourceid`, `referrer` | Segundos vistos | rt 9, 19, 29 s y luego cada 40 s | [R] |
| Bucles | vuelta en `st`/`et` (p. ej. `st=18.841,0 et=21.213,7.5`), `pe=previouslyEnded` | Revisionados; tiempo visto > 100 % de la duración. `pe` no aparece en todas las vueltas | Durante la reproducción | [R] |
| Swipe | ping `state=paused` + ping `final=1` en ~2 ms | Salida exacta. Skip rápido: 1,448 s de 28,76 s | Al deslizar | [R] |
| `log_event` | `html5PlayerHealthEvent.playTimeSecs`, `scrollEvent`, `screenCreated` (gesto `KEY_PRESS` con teclado; `SWIPE` en táctil según el código) | Segundo canal de dwell y gesto de navegación | Cada swipe | [R] |
| Búsqueda | `visualElementShown` por miniatura de Short (219x408) | Impresión y clic de Shorts en búsqueda y estanterías | Al verse | [R] |
| Prefetch | `prefetchPlaybackContext`, `isCacheHit` | El siguiente Short ya está decidido antes de que decidas sobre el actual | Mientras se reproduce el anterior | [R] |
| Overlay | like (corazón), comentar, compartir, remix | Sin dislike (eliminado para todos en junio de 2026); "No me interesa" solo con sesión | Al cargar | [R] + [C] *(corregido)* |
| `context` | dispositivo, `visitorData`, `poToken` | Identidad de dispositivo e integridad | Cada petición | [R] |

Diferencias con el largo: `el=shortspage`, sin barra de progreso (`REEL_SCRUBBER_STATE_DISABLED`), bucle por defecto, secuencia en lotes; el largo web usa `get_watch` (player + next combinados) con `el=detailpage` [R] *(corregido)*.

### 3.3 Google Search

La página de resultados de Google bloqueó todas las capturas con `/sorry` (reCAPTCHA) desde la IP usada para las capturas, con y sin headless. Lo siguiente sale de la portada de Google (misma infraestructura de logging) y del código xjs [R].

| Endpoint | Campo | Qué codifica | Cuándo se envía | Nivel |
|---|---|---|---|---|
| Cada página | `ei` (los 4 primeros bytes son la hora de servido); `sei` en /search | Id de impresión de página; une clics y beacons a un servido concreto | Cada carga | [R] |
| Cada elemento | `ved` (protobuf: marca de tiempo + 2 ids; tipo e índice del elemento inferidos de 3 muestras) | Qué bloque y qué hueco se clicó | En cada enlace | [R] *(debilitado en la semántica)* |
| `/url?sa=t&q&cd&ved&url&usg` | reescrito en `mousedown` | Clic saliente con consulta, hueco y destino, antes de llegar al sitio | Al hacer clic | [R] |
| Atributo `ping=` | ruta alternativa de log de clic | Solo en plantillas de código | No observado | [R*] *(debilitado)* |
| `gen_204?vet=...s/h/i` | impresión mostrada, oculta, insertada | Visibilidad real de módulos cargados tarde (Vídeos, Short videos) | Al aparecer o desaparecer | [R] |
| `gen_204 ct=slh` (`me=`) | viewport, rects de resultados visibles (R), scroll (S), clic dentro del resultado (G) y en la página (c), scroll de carrusel (f/Z), motivo de salida (e,C clic / H oculto / B blur / U unload) | Registro de atención: qué se vio, cuánto se tardó en clicar y cómo se salió. Ejemplo: clic a ~4,2 s, salida por clic | Continuo en la sesión | [R] |
| `pageshow` -> nueva sesión slh + `gen_204 ct=srpf` | volver a la SERP | Materia prima para detectar pogo-sticking | Solo en código; Back no se pudo ejecutar | [R*] *(debilitado)* |
| `/search` y `/complete/s` | `oq`, `uact=5`, `gs_lp`, `cp`, `psi` | Consulta tecleada frente a sugerida o refinada | Al buscar | [R] |
| `client_204`, `gen_204 atyp=csi` | `biw`, `bih`, `dpr`, tiempos | Tamaño de viewport (qué estaba sobre el pliegue) | Cada impresión | [R] |

En el cliente no se calcula ninguna etiqueta "good/bad click" ni "last longest click" (no aparece en 8 bundles xjs); si existen, se derivan en el servidor [I, debilitado].

Observación aparte, sin REA: una página /watch de YouTube servida con user agent de Googlebot (curl, un solo vídeo) contenía JSON-LD VideoObject con título, miniatura, fecha y 10 comentarios destacados, más microdata con duración, contador de interacciones, género y keywords [debilitado: un solo vídeo, no Googlebot real].

---

## 4. Cómo usa YouTube esas señales

### 4.1 Señales y objetivo (nivel 2)

- Más de 80.000 millones de señales: clics, tiempo de visionado, encuestas, compartidos, likes, dislikes. Los pesos se personalizan (los compartidos de quien comparte todo cuentan menos). En 2012 pasaron de clics a tiempo de visionado y las vistas cayeron un 20 % [C].
- **Tiempo de visionado valorado:** encuestas de 1 a 5 estrellas; solo cuenta el tiempo de 4 y 5 estrellas; un modelo predice la respuesta de quien no contesta [C].
- Help 2026: objetivo de satisfacción a largo plazo; usa vídeos vistos, ignorados y descartados, duración, búsquedas, likes, compartidos, comentarios, suscripciones, espectadores parecidos, dispositivo y hora; juzga cada vídeo con datos frescos de su rendimiento cuando se ofrece [C].
- Los pesos cambian por persona y momento ("no es igual para ti en todo momento", Beaupré) [C]. Lo de "cientos de experimentos" y "en TV pesa más el tiempo, en móvil los likes" es narración del artículo, no cita [debilitado].
- Covington: las señales más fuertes son la interacción previa del espectador con el canal y el tema; las impresiones repetidas sin clic reducen la probabilidad futura con ese espectador [C]. Beaupré mencionó un test con un tope de unas 8 recomendaciones del mismo vídeo por espectador; estado en producción desconocido [C, debilitado].
- Corrección de sesgo de posición (shallow tower): no se premia un vídeo solo por salir arriba [C].
- Suscripciones: CTR del feed de suscripciones de ~10 % o menos incluso para los mejores vídeos; el sitio donde más ven los suscriptores un canal es "Sugeridos" [C]. Notificaciones "personalizadas" solo para algunas subidas [C]; el tope de unas 3 por canal y día no está verificado [debilitado].

### 4.2 Largo frente a Shorts

| Aspecto | Vídeo largo | Shorts |
|---|---|---|
| Sistema | Recomendador propio | Recomendador separado, optimizado para el formato [C] |
| Entrada | Miniatura y título en Home, Sugeridos, Búsqueda | El feed muestra el vídeo, no la miniatura: el 99,9 % de las vistas de Shorts vienen del feed (dato de enero de 2025) [C] |
| Métrica central visible | CTR de impresiones, duración media, retención (sobre vistas comprometidas) [C] | "Viewed vs swiped away" [C]; los umbrales "buenos" del 70 al 90 % son folclore [I] |
| Lo que ve el cliente | Segmentos, saltos, pausa, salida y siguiente vídeo [R] | Segmentos, bucles, swipe con segundo exacto [R] |
| Negativos | Ignorar, descartar, dislike, "No me interesa", "No recomendar canal" [C] | Swipe como etiqueta negativa entrenada [C]; dislike eliminado en junio de 2026 [C] |
| Distribución | Sugeridos por co-visionado, Home por paquete y satisfacción, Búsqueda por relevancia [C] | Lotes de ~26 [R, debilitado]; exploración fuerte; puede priorizar contenido reciente [C, debilitado] |
| Conversión cruzada | | Vínculo "vídeo relacionado" (2023) [C]; sin datos oficiales de conversión |

YouTube afirma no tener evidencia de que los Shorts perjudiquen al largo [C] (declaración de 2024 y 2025; ver contradicción con el freno a nivel de canal de octubre de 2026 en la sección 6).

### 4.3 Señales negativas

- Wang et al. (RecSys 2023, plataforma de vídeo corto con miles de millones de usuarios, no nombrada): entrenar "no recomendar" con dislikes y skips. En vivo: tasa de dislike -2,44 %, dislikes repetidos del mismo creador -9,60 %, disfrute +0,40 % con skips; en simulación, recomendaciones similares -60,8 % (contenido) y -64,1 % (creador). El negativo se generaliza al creador [C].
- Mozilla 2022 (22.722 participantes, muestra no representativa): "No recomendar canal" evitó un 43 % de recomendaciones similares no deseadas, borrar del historial un 29 %, dislike un 12 %, "No me interesa" un 11 % [I, datos antiguos].
- Contenido limítrofe: degradado por clasificadores entrenados con evaluadores humanos; -70 % de tiempo recomendado de ese contenido en EE. UU. (2019) [C].
- Help: compartir con frecuencia contenido que no resuena puede afectar con el tiempo al rendimiento global del canal; un experimento fallido no lo hunde [C, debilitado en la redacción].

### 4.4 Satisfacción

El cliente transporta las piezas para medirla [R] [R*]: encuestas renderizadas por el servidor, tokens de feedback, "¿Sigues ahí?" con `lact`, foco en primer plano. Cómo pesan es nivel 2: el tiempo de visionado valorado y las encuestas previstas entran en el ranking multiobjetivo [C]. Un vídeo que retiene pero deja insatisfecho (clickbait, relleno) pierde contra uno que cumple.

---

## 5. Google Search, Discover y AI Overviews

### 5.1 Cómo rankea Google (juicio y filtración)

- **Pila (memo Nayak, PXR0357, enero de 2025):** más de 100 señales brutas, agregadas en señales de nivel superior como "combinación lineal del log de cada señal", que dan un "Final IR". Después los "twiddlers" reordenan. Señales nombradas: Q* (calidad), Navboost (frecuencia de clics por consulta y documento, por ubicación y dispositivo, últimos 13 meses), RankEmbed, PageRank [C].
- **ABC (memo HJ Kim, PXR0356):** Anchors, Body y Clicks ("cuánto tiempo se quedó el usuario antes de volver a la SERP") forman la topicalidad T*, combinada a mano. Q* es sobre todo estática y de sitio, "increíblemente importante". P* (popularidad) usa datos de Chrome [C].
- **Funciones de la SERP:** cada función (carrusel de vídeo, paneles) tiene su propio ranking; Tangram (antes Tetris) decide qué funciones aparecen, aprendido de logs [C]. Glue es el "super log de consultas" que guarda clics, hovers y duración en la SERP sobre todas las funciones [C, debilitado en citas exactas].
- **Clics:** Google "evita predecir clics" porque se manipulan y son un mal proxy; la señal antigua de CTR estaba sesgada por posición y se reemplazó por una corregida [C]. La filtración (autenticada por Google el 29/05/2024) lista `goodClicks`, `badClicks`, `lastLongestClicks`, `unsquashedClicks` [C]. Lehman (2023) describe un sistema rápido que usa clics en temas frescos [C, debilitado: transcripción no reverificada].
- **RankEmbed:** contradicción sin resolver. PXR0357 dice que funciona bien en consultas comunes y "puede rendir mal en la cola larga" y se entrenó con una muestra de un mes; la sentencia de remedios habla de ayuda en la cola larga y 70 días, quizá refiriéndose a RankEmbedBERT [debilitado].
- **Twiddlers:** confirmados [C]; la lista de degradaciones (Panda, SERP demotion, etc.) viene de análisis de la filtración [debilitado].
- **Página oficial "How Search works":** usa "datos de interacción agregados y anonimizados" para evaluar relevancia [C]. Gary Illyes llamó en 2019 "made up crap" a dwell y CTR; las pruebas del juicio lo contradicen [C].

### 5.2 Cómo entra un vídeo en Google

| Requisito o hecho | Nivel |
|---|---|
| El vídeo debe estar en una página de visionado indexada que "ya rinda bien en Search", con miniatura estable; metadatos por VideoObject, sitemap de vídeo u Open Graph | [C] |
| Desde diciembre de 2023 solo hay miniatura de vídeo si el vídeo es el contenido principal de la página | [C] |
| Momentos clave: en YouTube, marcas de tiempo con etiqueta en la descripción; Google también los autodetecta | [C] |
| YouTube aporta el 83 % (Perficient 2020) y hasta el 94 al 99 % (otros) de los carruseles de vídeo; no hay estudio riguroso 2024-2026 | [I, debilitado] |
| Filtro "Short videos" (Shorts, TikTok, Reels) en pruebas en móvil; Perspectives (2023) documentado | [C] |
| Google usa datos de usuario para decidir qué rastrear y con qué frecuencia; vídeo en índice propio | [C, debilitado] |
| YouTube Search: relevancia (título, descripción), engagement (tiempo de visionado para esa consulta), calidad (pericia del canal) y personalización | [C] |

### 5.3 Discover

Elegible con indexación y políticas; basado en intereses; advierte contra previsualizaciones engañosas y engagement inflado; imágenes de 1200 px o más [C]. Desde el 17/09/2025: botón Seguir y publicaciones de X, Instagram y YouTube Shorts [C]. El 05/02/2026 llegó el primer core update solo de Discover: menos sensacionalismo y clickbait, más contenido original y con pericia [C].

### 5.4 AI Overviews y AI Mode

- Seleccionan enlaces con query fan-out (búsquedas relacionadas por subtema); basta con estar indexado y ser apto para snippet [C].
- Grounding con FastSearch, basado en RankEmbed, con menos resultados y más rápidos que la búsqueda completa [C]. El post-entrenamiento MAGIT no está verificado [debilitado].
- AI Mode: más de 1.000 millones de usuarios mensuales; las consultas se duplican cada trimestre (I/O 2026) [C]. La cifra de 2.500 millones para AI Overviews no está en ese post [debilitado].
- Pew (2025): con resumen IA, clic en resultado tradicional 8 % frente a 15 %; clic dentro del resumen 1 % [I]. Ahrefs: -34,5 % de CTR en posición 1 [I].
- YouTube es el dominio más citado en AI Overviews (Ahrefs Brand Radar, ~20,9 %, +34 % en 6 meses); solo el 38 % de las citas viene del top 10 tras Gemini 3 (con un cambio de parsing de por medio) [I, debilitado en otras cifras].
- Desde el 03/06/2026, Search Console permite excluirse de las funciones de IA y muestra impresiones en ellas; excluirse no es señal de ranking fuera de esas funciones [C].
- Google Trends tiene la propiedad "Búsqueda de YouTube" y excluye las búsquedas internas de AI Mode/AIO; hay API alfa desde julio de 2025 [C].

### 5.5 Peso de los clics, en una frase

Los clics pesan, sobre todo en consultas con mucho volumen, con memoria de 13 meses y corregidos por posición; lo que se premia es el clic que termina la búsqueda (no volver a la SERP), no el CTR bruto [C]. En la cola larga (jugador, mapa, partido concreto) hay pocos datos de clics, así que pesan más la topicalidad y la calidad [C]; qué sistema decide ahí está en disputa (RankEmbed) [debilitado].

---

## 6. Cambios 2024-2026 que importan

| Fecha | Cambio | Impacto para CS2 y ClipHub | Nivel |
|---|---|---|---|
| 05/03/2024 | Helpful content integrado en el core; nuevas políticas de spam: abuso de contenido escalado | Muchas páginas de clips autogeneradas en un dominio propio: riesgo de degradación del sitio | [C] |
| 18/03/2024 | Divulgación obligatoria de contenido sintético realista | Gameplay no; una voz clonada de caster o jugador sí | [C] |
| 29/05/2024 | Google confirma la autenticidad de la filtración de Content Warehouse | Atributos de clics y calidad confirmados, sin pesos | [C] |
| 15/10/2024 | Shorts de hasta 3 min (vertical o cuadrado); usar 16:9 para no ser Short | El formato decide qué recomendador te juzga | [C] |
| 10/2024 | Un Short de más de 1 min con cualquier reclamación activa de Content ID se bloquea globalmente | Música con licencia o feed de torneo = alcance cero | [C] |
| 10/2024 | YouTube prometió mejorar la recomendación de Shorts largos; sin datos posteriores | No asumir igual distribución que < 60 s | [C] |
| 31/03/2025 | Vista de Short = cada inicio o bucle; la métrica anterior pasa a "vistas comprometidas" | Las vistas públicas inflan; medir con vistas comprometidas | [C] |
| 2025 | Recomendadores separados para Shorts y largo (blog de mitos, 06/05/2025) | Un Short viral no empuja solo al largo | [C] |
| 10/07/2025 | Fin de la página Trending; quedan Charts y la página Gaming | No hay hueco global de tendencia | [C] |
| 15/07/2025 | "Contenido repetitivo" pasa a "inauténtico" en YPP | Clips de momentos "con poca o ninguna narrativa" no monetizan | [C] |
| 24/07/2025 | Espectadores Nuevos / Casuales / Habituales en Studio | Lo viral se ve en Nuevos; el valor, en Habituales | [C] |
| 26/08/2025 | Hype global (39 países), leaderboard con filtro Gaming | Vía no algorítmica para canales de 500 a 500k | [C] |
| 16/09/2025 | Test A/B de títulos; Ask Studio; pestaña de Inspiración | Ganador por cuota de tiempo de visionado | [C] |
| 17/09/2025 | Discover muestra Shorts y permite Seguir | Nueva superficie sin consulta para Shorts | [C] |
| 10/2025 | PLUM (LLM) en el pool de candidatos | Más diversidad, sobre todo en Shorts | [C] |
| 17/11/2025 | Política de juego: prohibido dirigir a apuestas con skins | Nada de enlaces a sitios de skins ni case opening | [C, debilitado] |
| 03/02/2026 | Sentencia de remedios en vigor: Google comparte datos de Glue/RankEmbed con competidores | Dinámicas de "lo clicado gana" pueden extenderse a otros motores | [C] |
| 05/02/2026 | Primer core update solo de Discover | Contra clickbait | [C] |
| 26/06/2026 | Shorts: fuera el dislike, like en corazón, velocidad 2x | El swipe es el negativo visible dominante | [C] |
| 03/06/2026 | Exclusión de funciones IA en Search Console e impresiones IA | Medir citas en AIO | [C] |
| 20/07/2026 | Aclaración de inauténtico: plantillas y automatización sin narrativa única | Riesgo directo para pipelines automáticos | [C] |
| 25/07/2026 | Miniaturas personalizadas en Shorts (YPP, escritorio, sin A/B) | Útiles en búsqueda y canal, no en el feed | [C] |
| 24/08/2026 | Vista pública desde el primer fotograma en todos los formatos; "sin cambios en recomendaciones" | No comparar vistas antes y después | [C] |
| 01/09/2026 | Beaupré: pull, no push; no se decide solo con los primeros momentos | Evergreen sigue vivo | [C] |
| 23/09/2026 | Custom Feeds (EE. UU.), Ask YouTube; A/B de cortes de Shorts anunciado (detalles y fecha 2027 solo en fuentes secundarias) | Metadatos semánticamente claros | [C, debilitado] |
| **01/10/2026** | **Shorts prioriza contenido original; menos alcance para canales que principalmente agregan o resuben sin cambios significativos; voz en off descriptiva, ediciones técnicas menores y cambios masivos con plantilla no cuentan** | **El cambio más directo para canales de clips** | [C] |
| 01/02/2027 | YPP nuevo: 8.000 h en 365 días o 20M vistas de Shorts en 90 días; todos los socios necesitan 10M vistas cualificadas de Shorts en 90 días para cobrar ingresos de Shorts; el largo no cambia | Ventana para monetizar canales nuevos | [C, corregido] |

Contexto de demanda: 67,2M Shorts de gaming en 2025 (de 3,1M a 6,15M al mes), gaming es ~13 % de las vistas de Shorts, Counter-Strike 342M horas en directo en YouTube (+30 %) [I]. Austin Major 2025: 76,1M horas, 1,79M de pico (el más visto por horas; por pico, PGL Stockholm 2021 tuvo ~2,7M) [I, debilitado].

Gameplay y derechos: Valve permite monetizar vídeos con contenido de sus juegos vía YPP; la música de terceros necesita permiso [C]. El gameplay en general no es referencia válida de Content ID; los detalles de excepciones no están verificados [debilitado]. La política de PGL (uso transformativo permitido, resubida no) solo consta en un snippet [no verificable].

---

## 7. Mitos refutados o no confirmados

| Mito | Estado | Evidencia |
|---|---|---|
| "Las primeras 24 h lo deciden todo" | **Negado por YouTube** | Beaupré 2026; Help: la hora de publicación no afecta al rendimiento a largo plazo. Muchos evergreen tienen la mayoría de vistas un mes después [C] |
| "Retrasa la publicación 24 a 48 h" / "sube en hora punta" | **Negado / no confirmado** | Ritchie (11/2025): "no es distinto de esperar 24 a 48 segundos" [C]. Sin efecto confirmado en Google |
| "El CTR alto gana" / "el clickbait funciona" | **Contradicho** | Test & Compare elige por tiempo de visionado; Covington; Google "evita predecir clics" y corrige posición [C] |
| "Dwell y CTR no importan en Google" (Illyes 2019) | **Contradicho** | Navboost, ABC, lastLongestClicks [C] |
| "El contador de vistas público es lo que mira el algoritmo" | **Falso** | "Sin cambios en recomendaciones"; la analítica sigue en vistas comprometidas [C] |
| "Los bucles disparan el ranking" | **No confirmado** | Los bucles se miden [R] y suman vistas públicas [C], pero YouTube no ha dicho si cuentan para el ranking |
| "Hook de 1 a 3 s obligatorio" | **No publicado** | El swipe se reporta al instante [R] y los skips son negativos [C]; la regla de segundos es inferencia [I] |
| "Publica X Shorts al día" | **Sin base oficial** | Cada vídeo se juzga por su respuesta [C]; el volumen solo da más intentos [I] |
| "Los Shorts canibalizan el largo" | **Negado (2024-2025)** | Sistemas separados [C] |
| "Si los suscriptores no responden, el vídeo muere" | **Negado** | Se aprende aparte para no suscriptores, desde la primera hora [C] |
| "Hay penalty box tras un parón o un fracaso" | **Negado con matiz** | Un fracaso no hunde; contenido que no resuena de forma repetida sí afecta con el tiempo [C] |
| "El Hype sube en recomendaciones" | **No confirmado** | El blog dice que no influye en las recomendaciones de quien hace hype; exposición en leaderboard real [C, debilitado] |
| "Umbral de 70 a 90 % viewed vs swiped" | **Folclore** | YouTube no publica umbrales [C] |
| "Reconstrucción del algoritmo con Gemini en enero de 2026" | **Sin fuente oficial** | Solo blogs [I] |
| "Shorts sin dislike porque estabas sin sesión" | **Corregido** | Eliminado para todos en junio de 2026 [C] |

---

## 8. Playbook para Shorts y vídeos largos de CS2 con ClipHub

### 8.1 Shorts

| Palanca | Qué hacer | Por qué (nivel) |
|---|---|---|
| Hook | Primer fotograma = acción (primera kill, setup del clutch, marcador tenso). Nada de menús, fase de compra ni intros | Swipe con segundo exacto en ~2 ms [R]; skips entrenados como negativo y generalizados al creador [C] |
| Primer fotograma | Trátalo como miniatura del feed | 99,9 % de vistas desde el feed sin miniatura [C] |
| Duración | 15 a 45 s para frags (folclore [I]); hasta 60 s si hay música no segura; 61 a 180 s solo con audio sin reclamaciones posibles | > 60 s + cualquier reclamación = bloqueo global [C] |
| Bucle | Que el final enlace con el principio (corte en acción, misma toma) | Los bucles generan tiempo visto > 100 % [R]; efecto en ranking no confirmado |
| Retención | Sin relleno entre kills; cortes al ritmo de la acción | El cliente envía segmentos a nivel de segundo [R] |
| Originalidad | Narrativa propia: contexto de ronda, economía, por qué es difícil; variar estructura | Freno de alcance a agregadores (01/10/2026) [C]; plantillas idénticas = inauténtico [C] |
| Fuente del material | Demos propias o del usuario renderizadas, no feeds de torneo ni clips de streamers | Feeds reclamables, PGL solo tolera uso transformativo [no verificable], freno a resubidas [C]. Si un render desde una demo de partido profesional cuenta como "original" no lo ha dicho YouTube [I] |
| Puente a largo | Vínculo "vídeo relacionado" al partido o ronda completa, publicado en pocos días | Herramienta oficial [C]; el dato de Spotter (2 % de canales) no está verificado |
| Medición | Viewed vs swiped y vistas comprometidas, contra tu propia media | Vistas públicas infladas [C] |

### 8.2 Vídeos largos

| Palanca | Qué hacer | Por qué (nivel) |
|---|---|---|
| Formato | 16:9; recaps de partido, historias de Major, análisis de 15 a 40 min | Fuera de Shorts; TV es el dispositivo principal en EE. UU. [C] |
| Título y miniatura | Que prometan exactamente lo que entrega el vídeo; nombres de jugador, equipo, mapa y evento | Ganador por tiempo de visionado [C]; impresiones visibles y clic enlazados [R] |
| Test & Compare | Hasta 3 títulos, miniaturas o pares por vídeo; dejar ~2 semanas | Herramienta oficial [C] |
| Retención | Cortar el tiempo muerto entre rondas; no esconder la recompensa | Saltos visibles como huecos y seekData [R] |
| Siguiente vídeo | Final que lleve a otro vídeo del canal (pantalla final, serie) | El cliente enlaza la salida con el siguiente vídeo elegido [R]; Sugeridos es la superficie principal de los suscriptores [C] |
| Capítulos | Marcas de tiempo con etiquetas descriptivas (ronda, jugada) | Momentos clave en Google [C]; fan-out de AIO por subtema [C] |
| Satisfacción | Nada de cebos; la retención inflada con relleno puede volverse en contra | Tiempo valorado solo con 4 y 5 estrellas [C] |
| Re-empaquetado | Si un vídeo no arranca, cambiar título o miniatura | Impresiones repetidas sin clic lo agotan [C] |

### 8.3 Google y descubrimiento externo

- Títulos y descripciones precisos para la cola larga ("ace de X en Mirage, final de Y"), que es donde Google tiene pocos clics y pesa la topicalidad [C].
- Que el vídeo resuelva la consulta para que no se vuelva a la SERP (lastLongestClicks) [C].
- Publicar en YouTube para los carruseles; en un dominio propio, una página por clip y sin páginas finas a escala [C].
- Tráfico externo (Discord, Reddit, X) siembra co-visionado que el recomendador propaga [C].

### 8.4 Cadencia y eventos

- La cadencia no es palanca del algoritmo [C]; más vídeos solo dan más intentos [I], y el volumen de clips casi idénticos ahora es un riesgo [C].
- La hora de publicación no importa para el algoritmo [C]; la demanda externa sí: publicar en horas tras un partido de Major, un parche o un clutch viral [I].
- Hype: pedir a la comunidad (Discord, fans) que hagan hype en los primeros 7 días [C].
- Sin enlaces a apuestas de skins en descripción ni en comentario fijado [C, debilitado].

### 8.5 Política de contenido reutilizado

- Permitido: replays deportivos o de torneo con explicación de lo que hizo el jugador, revisión crítica, material ajeno con narrativa y comentario [C].
- No permitido para monetizar: "clips de momentos editados juntos con poca o ninguna narrativa", recopilaciones de otras redes, reacciones sin comentario; se evalúa a nivel de canal, incluso con permiso del autor [C].
- Para recomendación en Shorts (01/10/2026): la voz en off descriptiva, las ediciones técnicas menores y los cambios masivos con plantilla no cuentan como valor añadido [C].

### 8.6 Ideas para próximas versiones de ClipHub Studio

Estas funciones todavía no están en Studio: son ideas que salen de la evidencia de esta guía.

| Idea | Qué hace | Evidencia |
|---|---|---|
| **Guardia de música y duración** | Si un Short pasa de 60 s y el audio no es seguro (biblioteca de YouTube o propio), avisar y ofrecer recortar a 60 s o cambiar la pista. Por defecto, sin música con licencia | Bloqueo global > 60 s con reclamación [C] |
| **Selector de formato explícito** | "Short (vertical, ≤ 3 min)" o "Largo (16:9)" con explicación de qué recomendador lo juzgará | Clasificación por relación de aspecto [C] |
| **Hook automático** | Empezar el render en el primer evento de alto valor detectado en la demo (kill, clutch) y marcar si los primeros 2 s son estáticos (compra, menú) | Swipe instantáneo [R]; skips negativos [C] |
| **Finales en bucle** | Opción de cerrar el Short en un corte que encadene con el primer fotograma | Bucles medidos [R]; efecto no confirmado |
| **Motor anti-plantilla** | Variar estructura, ritmo, texto y orden entre exportaciones; avisar cuando un lote sale casi idéntico | Inauténtico por plantilla [C]; freno a agregadores [C] |
| **Capa de narrativa** | Overlays generados desde la demo: ronda, economía, vida restante, 1vsX, contexto del partido; plantilla de guion para comentario del usuario | Narrativa como valor añadido [C] |
| **Avisos de origen** | Marcar si el material viene de un feed de torneo o de un streamer y explicar el riesgo (reclamaciones, alcance, monetización) | [C], PGL [no verificable] |
| **Generador de metadatos** | Títulos con jugador, equipo, mapa, evento y jugada; descripción con capítulos de ronda; 3 variantes para Test & Compare | Cola larga [C]; momentos clave [C]; A/B por tiempo de visionado [C] |
| **Pareja Short + largo** | Del mismo render, sacar el Short y el vídeo largo, y recordar enlazar "vídeo relacionado" | Vínculo oficial [C] |
| **Modo Major** | Flujo rápido con plantillas de evento para publicar en horas | Picos de demanda [I] |
| **Panel de métricas correctas** | Importar desde la API de Analytics: viewed vs swiped, vistas comprometidas, duración media, fuentes de tráfico, Nuevos/Casuales/Habituales; comparar con la media del propio canal | Vistas públicas infladas [C]; sin umbrales oficiales [C] |
| **Checklist de cumplimiento** | Sin enlaces a skins; divulgación si se usa voz sintética realista | [C] |
| **Exportación multiplataforma** | Misma pieza para TikTok y Reels, que compiten en los módulos de vídeo corto de Google | Filtro en pruebas [C]; efecto [I] |

---

## 9. Límites de este análisis y fuentes

### 9.1 Límites

- **REA solo ve el cliente.** Muestra qué se mide y se envía, no cómo se pondera en el servidor. Ninguna afirmación de peso viene de REA.
- Todas las capturas: Chrome headless, sin sesión, perfil nuevo, región ES. No se ejercitaron likes, suscripciones, "No me interesa", "No recomendar canal", encuestas ni feeds personalizados. No hubo captura de apps móviles ni de TV, donde ocurre la mayor parte del consumo de Shorts y del consumo en EE. UU.
- La SERP de Google se bloqueó siempre (`/sorry`); los carruseles de vídeo y el módulo Short videos no se vieron. La vuelta a la SERP solo se leyó en el código.
- REA guarda como máximo 64 KiB por respuesta; la respuesta de `reel_watch_sequence` se obtuvo con un replay, no con captura pasiva. En una ejecución la reproducción headless falló (SABR).
- Muchos significados de campos (`idpj`, `ldpj`, `of`, `vm`, vectores, enums 20/12, pares id:puntuación) se infieren de valores, no de documentación.
- Los papers de 2016 y 2019 describen arquitecturas de hace 7 a 10 años; varios papers recientes no nombran YouTube.
- Varias citas de 2026 (Beaupré, YPP, dislike, miniaturas de Shorts) llegan por prensa especializada, no por el vídeo o el blog original.
- No hay datos públicos de rendimiento de Shorts específicos de CS2 ni analítica real de vídeos hechos con ClipHub que confirme estas recomendaciones.
- No se probó Content ID con feeds de torneo ni music kits, ni si las vistas headless cuentan en el contador público.

### 9.2 Fuentes

**Papers**
- Covington et al. 2016: https://static.googleusercontent.com/media/research.google.com/en//pubs/archive/45530.pdf
- Zhao et al. 2019 (MMoE): https://dl.acm.org/doi/10.1145/3298689.3346997
- Chen et al. 2019 (REINFORCE): https://arxiv.org/abs/1812.02353
- Yi et al. 2019 (two-tower): https://research.google/pubs/sampling-bias-corrected-neural-modeling-for-large-corpus-item-recommendations/
- Fresh content, KDD 2023: https://arxiv.org/abs/2306.01720
- Long-term value of exploration, WSDM 2024: https://arxiv.org/abs/2305.07764
- Negative feedback, RecSys 2023: https://arxiv.org/abs/2308.12256
- Semantic IDs, RecSys 2024: https://arxiv.org/abs/2306.08121
- PLUM 2025: https://arxiv.org/abs/2510.07784
- Reward shaping for satisfaction: https://arxiv.org/abs/2209.15166

**YouTube oficial**
- Goodrow 2021: https://blog.youtube/inside-youtube/on-youtubes-recommendation-system/
- Recommendation system (Help): https://support.google.com/youtube/answer/16533387
- Good to know about recommendations: https://support.google.com/youtube/answer/16559651
- Test & Compare: https://support.google.com/youtube/answer/13861714
- Shorts de 3 minutos: https://support.google.com/youtube/answer/15424877
- Analítica de Shorts: https://support.google.com/youtube/answer/12942217
- Métricas de audiencia: https://support.google.com/youtube/answer/9314415
- Políticas de monetización: https://support.google.com/youtube/answer/1311392
- Elegibilidad de Content ID: https://support.google.com/youtube/answer/2605065
- Hype: https://support.google.com/youtube/answer/15334500 , https://blog.youtube/news-and-events/youtube-hype/ , https://blog.youtube/news-and-events/hype-global-expansion/
- Notificaciones: https://support.google.com/youtube/answer/3382248
- Búsqueda de YouTube: https://support.google.com/youtube/answer/16090438
- Vistas comprometidas: https://blog.youtube/inside-youtube/engaged-views-youtube-explained/
- Vistas desde el primer fotograma: https://support.google.com/youtube/thread/433409976
- Originalidad en Shorts: https://support.google.com/youtube/thread/470890423
- Mitos de Shorts: https://blog.youtube/creator-and-artist-stories/debunking-common-myths-about-youtube-shorts/
- Made on YouTube 2025: https://blog.youtube/news-and-events/youtube-studio-made-on-youtube-2025/
- Made on YouTube 2026: https://blog.youtube/madeonyoutube/ , https://blog.google/products-and-platforms/products/youtube/made-on-youtube-updates-2026/
- Divulgación de IA: https://blog.youtube/news-and-events/disclosing-ai-generated-content
- Carta del CEO 2026: https://blog.youtube/inside-youtube/the-future-of-youtube-2026/
- Tráfico de vídeos relacionados: https://blog.youtube/creator-and-artist-stories/youtube-related-videos-traffic-guide/
- Creator Insider 2026 (Beaupré): https://www.youtube.com/watch?v=rHLjxrbXmmY
- Creator Insider (originalidad): https://www.youtube.com/watch?v=XS2A6aExLaA
- How YouTube Works: https://www.youtube.com/howyouworks/product-features/recommendations/

**Google Search oficial**
- How Search works: https://www.google.com/search/howsearchworks/how-search-works/ranking-results/
- Core update y spam 2024: https://developers.google.com/search/blog/2024/03/core-update-spam-policies
- Ranking systems guide: https://developers.google.com/search/docs/appearance/ranking-systems-guide
- Status dashboard: https://status.search.google.com/products/rGHU1u87FJnkP6W2GwMi/history
- Vídeo en Search: https://developers.google.com/search/docs/appearance/video
- Vídeo como contenido principal: https://developers.google.com/search/blog/2023/12/video-is-the-main-content
- Momentos clave: https://developers.google.com/search/blog/2021/07/new-way-key-moments
- Discover: https://developers.google.com/search/docs/appearance/google-discover , https://developers.google.com/search/blog/2026/02/discover-core-update
- Funciones de IA: https://developers.google.com/search/docs/appearance/ai-features
- Liz Reid 2025: https://blog.google/products/search/ai-search-driving-more-queries-higher-quality-clicks/
- I/O 2026: https://blog.google/products-and-platforms/products/search/search-io-2026/
- Controles para sitios: https://blog.google/products-and-platforms/products/search/new-controls-website-owners/
- Perspectives: https://blog.google/products-and-platforms/products/search/google-search-perspectives/
- Trends: https://support.google.com/trends/answer/4365533 , https://developers.google.com/search/blog/2025/07/trends-api

**Juicio y filtración**
- PXR0357 (Nayak): https://www.justice.gov/atr/media/1398866/dl
- PXR0356 (HJ Kim): https://www.justice.gov/atr/media/1398871/dl
- Sentencia de remedios Dkt. 1436: https://storage.courtlistener.com/recap/gov.uscourts.dcd.223205/gov.uscourts.dcd.223205.1436.0.pdf
- Transcripción Lehman 21/09/2023: https://thecapitolforum.com/wp-content/uploads/2023/10/2023.09.21-AM-Trial-Transcript_FINAL.pdf
- Filtración: https://ipullrank.com/google-algo-leak , https://en.wikipedia.org/wiki/2024_Google_Search_documentation_leak
- Confirmación de Google: https://www.theverge.com/2024/5/29/24167407/google-search-algorithm-documents-leak-confirmation

**Terceros (nivel 3)**
- Beaupré 2026: https://ppc.land/subscribers-skip-90-of-uploads-in-their-feed-youtube-director-says/
- Mitos 2024: https://www.searchenginejournal.com/youtube-algorithm-myths-debunked-insights-from-the-growth-team/510091/
- Retraso de 24 a 48 h: https://www.searchenginejournal.com/youtube-debunks-24-48-hour-upload-delay-recommendation/560613/
- Originalidad en Shorts: https://www.searchenginejournal.com/youtube-original-shorts-reposted-clips-reach/591774/
- Inauténtico 2026: https://techcrunch.com/2026/07/20/youtube-clarifies-policies-around-ai-slop-and-upsetting-videos/
- Vistas de Shorts 2025: https://techcrunch.com/2025/03/26/youtube-is-changing-how-youtube-shorts-views-are-counted/
- Trending retirado: https://techcrunch.com/2025/07/10/youtube-is-getting-rid-of-its-trending-page-and-trending-now-list
- Dislike eliminado: https://www.engadget.com/2202221/youtube-shorts-nixes-dislikes-adds-double-playback-speed/
- Miniaturas de Shorts: https://ppc.land/youtube-ends-2-year-wait-for-shorts-thumbnails-but-blocks-a-b-testing/
- YPP 2027: https://www.techtimes.com/articles/323954/20260811/youtube-partner-program-doubles-entry-bar-adds-monthly-shorts-earnings-gate.htm
- Mozilla 2022: https://www.mozillafoundation.org/en/youtube/user-controls/
- Pew 2025: https://www.pewresearch.org/short-reads/2025/07/22/google-users-are-less-likely-to-click-on-links-when-an-ai-summary-appears-in-the-results/
- Ahrefs: https://ahrefs.com/blog/ai-overviews-reduce-clicks-update/ , https://ahrefs.com/blog/ai-overview-citations-top-10/
- Carruseles 2020: https://www.digitalinformationworld.com/2020/12/study-suggests-google-favors-youtube-in.html
- Stream Hatchet: https://hatchet.gg/blog/how-youtube-shorts-impact-game-discovery/
- Austin Major: https://respawn.outlookindia.com/esports/esports-news/blasttv-austin-major-2025-becomes-most-watched-cs-tournament-ever
- Política de vídeo de Valve: https://store.steampowered.com/video_policy/
- PGL (no verificable): https://press.pglesports.com/287269-pgl-esports-copyright-policy
- Juego con skins: https://sbcamericas.com/2025/11/05/youtube-update-policy-gambling-content/
- Related links: https://tubefilter.com/2023/09/01/youtube-related-links-multi-format-creators-shorts-live-long-form/
- Ritchie, "el algoritmo sigue a la audiencia": https://www.tubefilter.com/2024/08/26/rene-ritchie-shorts-creator-faqs/
