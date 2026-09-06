# Optimizaciones de rendimiento: poll del hub y proceso principal

## Cambios

- `GET /api/jobs` y `GET /api/stream-jobs` responden `304 Not Modified` cuando `If-None-Match` coincide con el ETag débil de la lista. Un poll idéntico no vuelve a serializar ni envía el cuerpo.
- El proxy de Studio (`/api/demos/jobs`, `/api/streams`) reenvía el validador y el ETag. La forma inlined del roster (#134) no cambia.
- `RealApiClient.fetchJobs` y `RealStreamsApiClient.listJobs` reutilizan el último cuerpo en un 304.
- `ClipsHub` no llama `setModel` cuando `hubPollUnchanged` ve el mismo modelo, los mismos streams y el mismo fallo.
- `playback-info` reutiliza el resumen GPU durante 30 s. Un remount de Ajustes no vuelve a llamar `getGPUFeatureStatus`.

No se modifican semánticas de job, el payload de un 200, ni la UX.

## Mediciones locales

Linux/amd64, Go 1.26.6, Node 24.20.0. Medianas de cinco corridas de bench (`-count=5`) y de la prueba unitaria de GPU.

| Operación | Antes | Después | Cambio |
| --- | ---: | ---: | ---: |
| `GET /api/jobs` idle (50 jobs con roster), cuerpo | 34.360 B | 0 B | 100 % menos |
| `GET /api/jobs` idle (50 jobs con roster), tiempo | 185.742 ns/op | 70.775 ns/op | ~62 % menos |
| `GET /api/jobs` idle (50 jobs con roster), allocs | 231.025 B/op | 134.613 B/op | ~42 % menos |
| `GET /api/jobs` idle, estado HTTP | 200 | 304 | short-circuit |
| `getGPUFeatureStatus` en un segundo `playback-info` dentro del TTL | 1 | 0 | skip |

### Reproducir

Desde la raíz:

```sh
go test ./internal/httpapi -run 'TestListJobsNotModified|TestListStreamJobsNotModified|TestListJobsWritesBodyWhenStatusChanges' -count=1
go test ./internal/httpapi -run '^$' -bench 'BenchmarkListJobs(UnchangedPoll|FirstPoll)$' -benchmem -count=5
node web/scripts/bench-hub-poll.mjs
pnpm --dir web run test:unit
pnpm --dir desktop run test:unit
```

`BenchmarkListJobsUnchangedPoll` es la regla congelada: un GET caliente con `If-None-Match`. Antes de este cambio el segundo GET seguía siendo 200 y copiaba el cuerpo completo. Después es 304 y `body-B` vale 0.

`bench-hub-poll.mjs` comprueba que un modelo reconstruido de 50 partidas se detecta como sin cambios. No es una medida de FPS.

## Validación

- Unitarias de `internal/httpapi` para 304, cambio de status y fingerprint del roster.
- Unitarias web para 304 del cliente, reenvío del proxy y `hubPollUnchanged`.
- Unitarias desktop para el TTL de `playback-info`.
- Typecheck y lint de desktop y web.
- Gap de captura: `hlae_cs2_windows_studio` en este host Linux. No se afirma un Pass de Full Demo ni 9:16.

El seed de música del catálogo empaquetado no tiene tracks remotos, así que un skip-hash al arranque no mueve la regla. El catálogo de renders de stream tampoco se memoriza por `UpdatedAt`: un cambio de draft o de `status.json` puede dejar esa marca quieta y un 304 escondería `stale`. Quedan como follow-up.
