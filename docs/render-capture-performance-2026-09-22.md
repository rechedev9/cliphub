# Rendimiento de Full Demo y cierre de captura

Mediciones locales del 22 de septiembre de 2026, frente a `1e7e5e4a`
(Studio 4.0.0), en Windows, Ryzen 7 9800X3D, RTX 5080 y FFmpeg 8.1.2
distribuido con Studio. Los resultados pertenecen a este contenido y equipo.

## Cambios

- Los cortes completamente fuera de las ventanas de intro/outro dejan de
  cargar y animar sus imágenes. Se conserva la conversión RGBA → YUV420P:
  eliminar también esa conversión cambia los píxeles aunque el overlay esté
  desactivado. La detección respeta extremos inclusivos, redondeo a milisegundos
  y el reloj entero de 60 fps.
- Los filtros de cada proceso de vídeo Full Demo tienen un máximo de cuatro
  hilos, limitado también por las CPU disponibles. El pool de tres procesos
  mantiene su tamaño; los ajustes de decodificación, encoder, calidad y audio
  conservan sus valores. La mejora se evalúa con vídeo y audio trabajando
  simultáneamente, como en el render real.
- El cierre de captura procesa hasta cuatro destinos de mux/probe en paralelo.
  Los resultados conservan el orden de entrada, las tomas del mismo destino
  se serializan (también entre mayúsculas y minúsculas), y los clips publicados
  durante la captura se reutilizan. Los errores siguen asociados a cada toma.
  La publicación atómica y la espera de todos los procesos se mantienen.

## Medición con renders completos

Se ejecutan los binarios baseline y candidato sobre las mismas tomas reales,
con HUD, transiciones, overlays FACEIT y cinco pistas de voz. La salida larga
tiene 55.836 fotogramas y 930,6 segundos. Cada ejecución incluye la mezcla,
los intentos de mastering AAC y recuperación, el mux final y la decodificación
completa de entrega. Las mediciones no se solapan con otros renders ni tests.

| Prueba | Baseline | Candidato | Reducción |
| --- | ---: | ---: | ---: |
| Render de 17 rondas, primera pareja | 253,579 s | 240,920 s | 4,99 % |
| Render de 17 rondas, repetición | 265,070 s | 241,881 s | 8,75 % |
| Render de 3 rondas recién capturadas | 49,934 s | 47,764 s | 4,35 % |

Los cuatro MP4 largos son idénticos byte a byte, así como los dos de la captura
nueva entre sí. El objeto completo
de evidencia Full Demo también coincide, incluidos los planes, transiciones,
HUD, mediciones de loudness y entrega.

| Entrega | Fotogramas | Duración | SHA-256 del MP4 |
| --- | ---: | ---: | --- |
| 17 rondas | 55.836 | 930,6 s | `2b69b26322d2a9f56f7adf7ee1c68bd527a636b4aee391312404b1e8ca258fb8` |
| 3 rondas nuevas | 7.646 | 127,433333 s | `4c76d4b50f1a9875d3c4062a010394901628a30467ca56bf24e937631724abcd` |

Ambas entregas conservan H.264 1920×1080 a 60 fps, AAC estéreo a 48 kHz,
conteo canónico de frames y `full_decode=true`. La advertencia de imágenes
congeladas de la demo larga aparece también en el baseline.

Una comprobación independiente vuelve a leer los archivos entregados, calcula
su SHA-256, comprueba metadatos con ffprobe y compara la evidencia completa.
También coincide el PCM del programa largo antes de mastering en las cuatro
ejecuciones: SHA-256 `68baf2bace35ff3a660e5a5bbd61fbed5e913032dfd73a5652576e38942d407a`.
La inspección visual de muestras de la captura nueva incluye intro, gameplay,
ambos lados de una transición y outro.

La fixture histórica tenía telemetría HUD v2, rechazada por el contrato actual
v3. Se generó una fixture local nueva desde la misma demo, verificando hash y
jugador, y se usó para ambos binarios. No se modificó la aprobación guardada
del usuario ni se relajó la validación de producción.

## Captura real y finalización

CS2/HLAE capturó tres rondas nuevas con el recorder candidato. El proceso
tardó 84,290 s; `capture_verified=true`, sin errores ni advertencias, y las
evidencias de restauración de configuración y archivos fueron positivas.
Se usó [HLAE 2.192.2](https://github.com/advancedfx/advancedfx/releases/tag/v2.192.2)
en un directorio de prueba local. Las tomas produjeron los dos renders de
127,433333 s de la tabla anterior.

El cierre se comparó sobre esas mismas tomas, con tres repeticiones por variante
y orden alternado. Incluye mux AAC y recogida de metadatos. Todos los MP4 de
segmento resultaron idénticos byte a byte entre variantes y repeticiones.

| Estado al terminar HLAE | Mediana baseline | Mediana candidato | Reducción |
| --- | ---: | ---: | ---: |
| Tres tomas pendientes | 2,092 s | 1,002 s | 52,10 % |
| Dos publicadas durante captura, última pendiente | 1,088 s | 0,972 s | 10,66 % |

Estas mejoras corresponden al cierre, no a todo el tiempo de grabación. El
segundo caso representa mejor una captura normal con mux incremental activo.
El ahorro total depende de cuántas tomas queden pendientes. El cambio no altera
el plan de captura, los ticks ni la tolerancia de dos frames de cola.

## Decisiones de rendimiento

La supresión de imágenes inactivas aceleró alrededor de un 10 % el ensayo
aislado de tres cortes, pero empeoró el primer render integrado
(253,579 → 266,300 s) por competencia con el audio. Por eso se comprobó el
presupuesto de filtros en el flujo completo antes de aceptar la combinación.
Limitar hilos de decodificación a dos o cuatro no mejoró las mediciones y se
descartó. Los presets y la calidad de codificación no se modificaron.

## Validación

Con el FFmpeg distribuido y Go 1.26.6:

- Pasan las seis suites completas de `internal/editor`, `internal/recording`,
  `internal/workers`, `cmd/zv-recorder`, `internal/demooverlay` e
  `internal/recapplan`: 2.248 tests/subtests aprobados, sin fallos.
- La ejecución general omite 16 tests auxiliares u opcionales, incluidos
  previews/benchmarks y cuatro tests de parser/tácticas que requieren otra
  demo de test no disponible. Los dos tests Chromium omitidos por falta de
  variable de entorno se ejecutan después con el renderer instalado y pasan.
- Pasa `go test -race ./internal/recording` limitado a los tests del pool,
  equivalencia de mux, cancelación y reutilización de clips.
- Pasa `go vet ./...`; revisión del diff con `code-review` nivel medium sin
  hallazgos; `git diff --check` limpio.

Las regresiones añadidas comprueban píxeles antes de codificación, extremos
inclusivos/redondeados, separación del presupuesto de filtros y encoder,
concurrencia acotada, orden, destinos compartidos, fallo de una toma,
cancelación, publicación/reutilización y conservación de 122 frames con
paquetes H.264 reordenados y un WAV ligeramente más corto.

## Evidencia reproducible

El [resumen JSON](render-capture-performance-2026-09-22.json) conserva los
tiempos de cada ejecución, hashes, metadatos y comandos de validación.

Los artefactos locales se conservan bajo `.local/render-analysis/`:

- `phase-a-out/<label>/`: inputs, comando, logs, timings y entrega larga;
  `baseline-current` y `candidate-filter4` son la primera pareja;
  `baseline-repeat` y `candidate-repeat` son la segunda.
- `fresh-capture/`: captura CS2/HLAE y certificaciones.
- `fresh-renders/{baseline,candidate}/`: entregas de las tomas nuevas.
- `mux-sweep/results.json`: las doce ejecuciones de finalización, con hashes.
- `filter-sweep`, `decoder-sweep`, `overlay-sweep`: ensayos de componentes.
- `verify-final-evidence.py`: comprobación independiente de SHA-256, códecs,
  frames, PCM y equivalencia de la evidencia Full Demo.
- `verified-results.json`: resultados de esa comprobación independiente.

Los archivos multimedia y las entradas específicas del usuario permanecen en
el directorio local ignorado por Git.
