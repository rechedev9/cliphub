# Preview local de WebMCP

La preview publica cinco herramientas nativas de la página: consultar navegación,
abrir una sección, consultar la biblioteca de demos, cambiar su vista y abrir una
partida existente. Se activan con `NEXT_PUBLIC_WEBMCP_PREVIEW=1`, en loopback y con
un Chromium que exponga `document.modelContext`. No hay un polyfill alternativo.

```bash
cd web
NEXT_PUBLIC_WEBMCP_PREVIEW=1 pnpm exec next dev --webpack --hostname 127.0.0.1 --port 3002
```

Los adaptadores utilizan el router y el modelo de React de la aplicación. El
registro se cancela al desmontar; los callbacks consultan estado confirmado en
el commit. Los navegadores sin soporte mantienen su comportamiento habitual.

Verificado el 7 de septiembre de 2026 con Chromium 151 en Omarchy y el puente Python
de `omarchy-computer-use` 0.4: cinco herramientas descubiertas, consulta de biblioteca,
cambio de vista, navegación a streams y vuelta, retirada al desmontar, rechazo
de argumentos inválidos y partida inexistente. Pasaron las tres pruebas de
`lib/webmcp.test.ts`, `tsc --noEmit` y el lint de los archivos modificados.

El orquestador estaba apagado: la API devuelve estado de datos parcial/no disponible,
igual que la UI. No se verificó abrir partidas válidas, editar vídeos ni renderizar.
La biblioteca expuesta incluye las demos y clips de su modelo, hasta 100 por lista;
los resultados de streams todavía no se incluyen. La activación es optativa y
no expone operaciones de creación, borrado o renderizado en esta revisión.

La reproducción completa, fuentes recientes y evidencias están en
`/home/luisreche/projects/omarchy-computer-use/docs/webmcp-cliphub.md`.

## Punto de reanudación — 7 de septiembre de 2026

La siguiente ampliación quedó en exploración, sin cambios de implementación.
El primer recorrido propuesto es listar y abrir proyectos de stream, consultar
su plan, añadir o corregir momentos, elegir formato y consultar guardado,
bloqueos y resultados. La página `app/(app)/streams/[id]/page.tsx` mantiene el
plan, el autoguardado y el estado del render; el editor visual recibe ese estado.
Las herramientas deberán reutilizar las validaciones de `lib/clip-edit.ts` y
los helpers de `lib/streams/plan.ts`, conservar la revisión de facecam y evitar
sobrescribir cambios concurrentes de la interfaz.

Antes de ampliar las operaciones conviene definir respuestas de error
estructuradas: Chromium oculta las excepciones del callback tras un error
genérico. También hay que distinguir un cambio confirmado en React de un
guardado aceptado por el servidor. Verificar el recorrido con datos de prueba,
incluyendo fallos de guardado y recuperación, y señalar si el backend es real
o simulado. Edición, render y persistencia siguen pendientes de prueba.
