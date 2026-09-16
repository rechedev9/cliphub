# Preparar un vídeo largo para YouTube

En **Clips → vídeo largo terminado → Publicar**, el asistente ofrece plantillas
en inglés con título, descripción y etiquetas editables. Puedes copiar cada
campo o todo el paquete, descargar el MP4 y abrir YouTube Studio.

| Enfoque | Plantilla |
| --- | --- |
| Bajas destacadas | `{player} Drops {kills} KILLS on {source}! POV{comms} ({map})` |
| POV clásico | `{player} POV{comms} on {source} ({map})` |
| Sesión de juego | `{player} Plays {source}! POV{comms} ({map})` |
| Mapa protagonista | `{map} {source} — {player} POV{comms} \| CS2` |
| Duelo de la demo | `{player} vs {opponent} on {source}! POV{comms} ({map})` |

Las bajas proceden del manifiesto del vídeo: son las incluidas en el montaje,
no necesariamente el total de la partida. La opción de bajas se omite si son
cero. El rival se identifica en las bajas de la demo por SteamID y equipos;
se omite si no hay evidencia suficiente. No se atribuye identidad profesional.

El origen procede del plan efectivo del render (`source_kind`), con CS2 como
base si no hay FACEIT o Premier. La configuración visual de los overlays no
se usa para inferir el origen. `with COMMS` requiere voces disponibles,
paquetes seleccionados, ganancia positiva, una pista de voz medida y actividad
de voz dentro de los tramos de gameplay conservados en el render.
Los vídeos antiguos sin evidencia reciben textos genéricos.

Las descripciones incluyen jugador, mapa y bajas; no llevan `#Shorts`, ni
presentan el montaje como una partida sin cortes. Las plantillas no inventan
ELO, nivel 10, Rank 1, victorias, aces ni condición de PRO. La selección de
una plantilla reemplaza los tres campos del borrador actual; las ediciones
son temporales hasta copiarlas. No hay subida automática.
