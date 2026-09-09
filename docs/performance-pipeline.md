# Rendimiento: Electron, replay, Full Demo y Demo → Shorts

Continuación de [las optimizaciones iniciales](performance-electron.md). Baseline: `d45e41f`. No se reducen resolución, FPS, muestreo, calidad de codificación, controles de integridad ni validaciones de audio.

## Cambios

### Electron y análisis táctico

- **Telemetría por lotes:** un polling importa errores y spans con una sola publicación atómica de la cola. Conserva muestreo, consentimiento, cola máxima de 200 eventos y reintentos. Si falla la publicación, ninguno de los dos cursores avanza. El camino individual comparte la misma normalización. No se añade un debounce que pueda perder eventos pendientes al cerrar.
- **Demo en una pasada:** el parser lee a través del SHA256; los bytes que deja sin consumir se drenan por el mismo lector. Incluye read-ahead y bytes posteriores al final del parser, sin contarlos dos veces. La lectura respeta cancelación y un SHA256 suministrado se verifica, no se da por válido.
- **Recolector:** 16 slots temporales y reserva exacta de muestras presentes, sin ordenar cada frame válido ni crecer repetidamente su slice. Conserva duplicados/slots inválidos para que la validación posterior siga rechazándolos.
- **Interpolación:** emparejamiento de slots y deltas espaciales/angulares por pareja de frames, no por refresco. Mantiene ángulo corto, muertes, huecos, seeks e inmutabilidad de resultados.
- **Dibujo:** medidas de etiquetas en LRU de 32 entradas y coordenadas de eventos en WeakMap. Invalida coordenadas por geometría/tamaño y medidas al cargar fuentes. El dibujo se detiene en el primer evento futuro sin crear un array por frame; reloj y texto ARIA solo se escriben si cambian.
- **Memoria del replay:** LRU de cuatro rondas decodificadas. Una ronda expulsada se recupera del blob ya verificado, sin otra petición.

Se evaluaron las variantes opcionales de la propuesta: no se introduce Web Worker sin evidencia de long tasks del decodificador, ni un bitmap adicional de eventos estabilizados que cambie el compositado de transparencias. Las coordenadas y textos sí se cachean; el orden y el fade siguen dibujándose exactamente igual.

### Full Demo y Demo → Shorts

- **Una decodificación completa de verificación:** `ffprobe` conserva la comprobación de metadatos, pero deja de hacer una primera decodificación con `-count_frames`. La decodificación obligatoria de FFmpeg cuenta los frames mediante `-progress`, exige `progress=end`, cero duplicados/drops y el conteo canónico. `-xerror`, audio, duración, formato, SHA256 y verificación de AAC/loudness siguen vigentes. `-fps_mode passthrough` evita corregir silenciosamente el conteo durante la comprobación.
- **Progreso por pipe:** el editor consume registros de stdout incrementalmente, en vez de releer un archivo creciente cada 250 ms. Retiene como máximo una línea parcial de 4096 bytes, drena líneas inválidas, publica progreso monótono y espera el cierre del lector antes de regresar/cancelar. La publicación atómica del vídeo no cambia.
- **Detección de killfeed:** acceso concreto a RGBA/NRGBA sin crear una interfaz de color por píxel; conversión de alpha idéntica y fallback para otros formatos. El BFS consume su propia máscara, eliminando la segunda matriz de visitados. No se cambian umbrales, ventanas, crops ni fotogramas elegidos.
- **Carrera encontrada en E2E de reproducción:** un seek a la posición actual podía notificar `paused` antes de finalizar `AudioContext.resume()` y cancelar el Play del usuario. El estado de inicio pendiente protege esa transición, sin permitir reproducir tras pausa/cancelación. La prueba observa el evento real `playing` y la parada al final, en vez de depender de ver un botón transitorio durante un clip de un segundo.

## Microbenchmarks locales

Windows/amd64, Ryzen 7 9800X3D, Go 1.26.6, Node 24.18.0, FFmpeg 8.1.1. Medianas de tres ejecuciones Go; cinco para interpolación. No son tiempos de captura ni mejoras porcentuales de toda la aplicación.

| Operación | Antes | Después |
| --- | ---: | ---: |
| Recolectar un frame de 10 jugadores | 779 ns; 1880 B; 8 allocs | 167 ns; 576 B; 1 alloc |
| Componentes rojos del killfeed 1080p | 2,102 ms; 1.510.450 B; 248.838 allocs | 0,796 ms; 261.168 B; 5 allocs |
| 60.000 interpolaciones a 60 Hz sobre muestras de 8 Hz | 14,371 ms | 9,966 ms |
| Verificación Full Demo de 5 s / 300 frames sintéticos | 1,892 s | 1,556 s |
| Heap retenido por rondas decodificadas de una demo real | 22.162.304 B / 19 rondas | 4.697.600 B / 4 rondas |

El heap se midió en Node con GC explícito y el mismo blob de 1.403.048 bytes. Es una observación del decodificador/caché, **no** una reducción de la RAM total de Electron. En esa ejecución la decodificación por ronda quedó por debajo de 1 ms; eso tampoco sustituye un perfil de long tasks del navegador.

## Mediciones globales y límites

[Resultados exploratorios completos](performance-pipeline-results.json): tres ensayos alternados por revisión, perfiles desechables, misma demo de 19 rondas, viewport 1440×1000, DPR 1, monitor de 360 Hz y ventanas de 15 s. Se conserva la mediana de cada métrica, no se fabrica un informe de proceso individual con esas medianas.

- Replay: CPU p95 **11,147 → 4,999 %** del equipo; intervalo de frame p95 **5,6 → 2,9 ms**. Working set mediano de los picos **948.031.488 → 869.748.736 B**. El comparador conservador acepta ese escenario.
- Análisis: **no pasa** el criterio de no regresión global: CPU p95 **12,277 → 12,727 %**, working set **786.706.432 → 805.777.408 B** y private bytes **773.775.360 → 833.650.688 B**. La medida de apertura incluye cola/polling/primer pintado, no solo el parser. Los tiempos de apertura varían entre ensayos y no justifican atribuir su diferencia exclusivamente a la optimización del parser.
- Las mediciones de memoria entre perfiles fríos varían, especialmente en Next y otros hijos después del upload. No se afirma una mejora global universal de RAM ni del arranque. Queda pendiente aislar esos costes de upload/GC en un perfil estable.

También se corrigió el **medidor**: Windows PowerShell seleccionaba `Math.Max(int,int)` con un cero entero y redondeaba a cero deltas pequeños de CPU. `cpu_sampling_version: 2` fuerza aritmética flotante; no se comparan informes entre versiones ni con distinto modo de muestreo GPU. `-SignalReady` sincroniza el inicio del trabajo con el primer contador; `-SkipGpu` evita que los contadores GPU lentos perturben operaciones cortas. GPU omitida no significa consumo GPU nulo. Se descartaron las cifras obtenidas con la versión anterior del contador.

## Verificación

- Suites completas Go, desktop y web; typecheck/lint; `go vet` de los paquetes cambiados; race detector dirigido: pasan.
- Demo real: parser/roster y escaneo táctico; las 65.536 máscaras de slots; documento idéntico al baseline salvo timestamp y blob **idéntico byte a byte**.
- Electron real, recursos de producción recompilados: 8 pruebas UI, incluida importación, preview, exportación y reproducción del MP4 aprobado; ninguna omitida.
- E2E táctico real: upload → Go → SHA256 → decodificación → canvas, reproducción, seeks y vuelta a una ronda expulsada de la LRU con píxeles idénticos y sin refetch del blob. Perfiles temporales, sin usar la base de datos habitual.
- Playwright: **57 pruebas pasan**, cubriendo Full Demo, drafts de Shorts, editor de streams, medios reales y upload/roster.
- FFmpeg real: verificación de conteo, rechazo de vídeo truncado, progreso/cancelación, equivalencia de píxeles en overlays/killfeed, mezcla de voces, loudness/AAC, sponsor y playlist.
- **Capture Lab Full: 22 etapas pasan, nivel L4.** Incluye scripts en simulador, render sintético real, oráculos de medios, HTTP/colas y Studio. Es evidencia compuesta; no una captura productiva continua. No se lanzó CS2/HLAE ni se recertificó compatibilidad L5; los artefactos sintéticos siguen excluidos de reutilización productiva.

No se genera ni publica instalador. Logs y medios locales en `.local/perf2-*` y `desktop/e2e/artifacts/` (no versionados).

## Reproducir

```sh
go test ./...
go test -race ./internal/editor ./internal/tactical -run 'Test(ProgressPipe|DecodedDelivery|FFmpegProgressPipe|RedPixel|SampleSlots|ScanHashed|.*Concurrent)'
go test ./internal/tactical -run '^$' -bench BenchmarkCollectSamples -benchmem -count=3
go test ./internal/editor -run '^$' -bench 'Benchmark(KillfeedComponents|ProgressPipe|FullDemoDelivery)' -benchmem -count=3
node web/scripts/bench-tactical-replay.mjs
node --expose-gc web/scripts/bench-tactical-decode.mjs <index.json> <positions.bin>

pnpm --dir desktop run test:unit
pnpm --dir web run test:unit
# Recompilar Go con scripts/build.ps1 y después:
pnpm --dir desktop run build
pnpm --dir desktop run assemble
# TEST_DEMO_PATH apunta a una demo local de seis o más rondas.
# CLIPHUB_MEASURE_TACTICAL=1 activa los informes sincronizados de procesos.
pnpm --dir desktop run test:e2e:tactical
# CLIPHUB_PLAYBACK_TEST_MP4 apunta al fixture aprobado por su hash.
pnpm --dir desktop run test:e2e:ui
node scripts/capturelab/lab.mjs --mode Full --iterations 1 --timeout-seconds 300
```

Para comparar revisiones hay que reconstruir ambas con las mismas dependencias y usar **el mismo medidor v2**. Los informes tácticos requieren además `workload_id` y duración/cadencia obtenidas por el driver, no solo renombrar una muestra en reposo.
