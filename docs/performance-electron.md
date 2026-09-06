# Optimizaciones de rendimiento: Electron y engine

La continuación (telemetría por lotes, replay, Full Demo y Demo → Shorts) está en [performance-pipeline.md](performance-pipeline.md), con sus mediciones y límites de validación.

## Cambios

- `desktop/src/copy-and-hash.ts`, `runtime-tools.ts`: copia del archivo HLAE con streams, backpressure, SHA256 en la misma pasada y cancelación. Evita bloquear el proceso principal con `copyFileSync` y cargar el archivo completo en RAM con `readFileSync`.
- `desktop/src/runtime-tools.ts`: el manifiesto reutiliza los hashes de cada fichero que ya se verificaron contra el hash del árbol fijado en código. Evita volver a leer y hashear toda la instalación. Los arranques siguientes siguen verificando todos los archivos; no se sustituye la verificación por confianza en el manifiesto local.
- `desktop/src/telemetry-journal.ts`: no publica de nuevo cursores idénticos durante el polling en reposo; una publicación fallida se reintenta. Consulta generaciones anteriores de spans solamente cuando cambia la identidad del archivo actual. Conserva consentimiento, filtrado y persistencia de eventos.
- `internal/tacticalplan/positions.go`: índice local de 16 slots para escribir muestras ordenadas sin búsquedas repetidas; conteo de bits presentes al decodificar; reserva del buffer por muestras realmente presentes, no por el mayor slot utilizado.
- `web/lib/tactical-replay.ts`, `web/components/tactical/tactical-replay.tsx`: construcción lineal de estelas, sin `unshift` repetidos; caché de un único frame por lector/ronda. La interpolación sigue actualizándose a la frecuencia de pantalla, pero las estelas se recalculan solamente al cambiar el frame de muestras. No hay caché global creciente.

No se modifican el formato de posiciones, precisión, cadencia de reproducción, calidad de exportación ni controles de usuario.

## Mediciones locales

Windows/amd64, Ryzen 7 9800X3D, Go 1.26.6, Node 24.18.0. Medianas de tres ejecuciones para Go y cinco para JavaScript.

| Operación sintética | Antes | Después | Cambio |
| --- | ---: | ---: | ---: |
| Codificar 24 rondas × 720 frames × 10 jugadores | 5,123 ms | 3,568 ms | ~30 % menos tiempo |
| Memoria asignada al codificar 24 × 720 frames con solo el slot 15 | 2.876.672 B/op | 279.808 B/op | ~90 % menos memoria |
| Calcular estelas: 60.000 evaluaciones a 60 Hz sobre muestras de 8 Hz | 249,365 ms | 10,802 ms | ~23× más rápido |

La comparación de memoria dispersa se tomó antes/después de ajustar la reserva del buffer, con el índice de slots ya aplicado en ambas variantes. No implica una reducción del 90 % de la RAM total de Electron.

Estos son microbenchmarks, **no mediciones de FPS, CPU o memoria global de la aplicación**. Las optimizaciones de instalación afectan a la preparación de herramientas; no se atribuyen al arranque con caché ya válida. Se descartó una variante de iteración por bits del decodificador que empeoraba su benchmark.

### Reproducir

Desde la raíz:

```sh
go test ./internal/tacticalplan -run '^$' -bench 'Benchmark(EncodePositions|EncodeSparsePositions|DecodePositionsRound)$' -benchmem -count=3
node web/scripts/bench-tactical-replay.mjs
```

El benchmark JavaScript incluye el algoritmo previo y comprueba igualdad antes de medir. Para una comparación global de procesos siguen disponibles `scripts/measure-desktop-efficiency.ps1` y `desktop/scripts/compare-efficiency.mjs`; no se ha realizado aquí esa comparación global.

## Validación

- `go test ./...`: pasa.
- Unitarias completas de desktop y web: pasan.
- Typecheck y lint de desktop y web: pasan.
- Build de desktop y producción Next.js: pasan.
- Electron real con perfil temporal y recursos compilados: 7 pruebas pasan; 1 omitida porque falta `CLIPHUB_PLAYBACK_TEST_MP4`, el fixture aprobado para reproducción/exportación real.
- Regresión binaria: las 65.536 máscaras posibles de slots se comparan con el escritor anterior, con entradas desordenadas, sin mutar las muestras originales.
- Regresión de estelas: igualdad de contenido y orden para frames dispersos y ventanas distintas; reutilización entre valores de alpha, avance y búsqueda hacia atrás.
- Regresión de Electron: copia byte a byte y SHA256, cesión al event loop, cancelación/cierre de archivos, errores de I/O, manifiestos verificados, polling sin escrituras y reintento tras fallo de publicación.

Los logs de esta ejecución están en `.local/*performance*`, `.local/positions-*.txt` y `.local/tactical-trails-after.json` (no versionados). No se ha generado un instalador de distribución.
