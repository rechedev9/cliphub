# Experiencia de creación de ClipHub Studio

Revisión del 4 de septiembre de 2026 sobre `dd51c93` (Studio 2.4.54). La copia local estaba atrasada y se actualizó antes de implementar. `/onboarding` es una redirección de compatibilidad a `/clips`, no una fase del producto.

## Modelo de experiencia

| Intención | Origen | Selección | Resultado | Preparación |
| --- | --- | --- | --- | --- |
| Short de CS2 | Demo `.dem` o archivo comprimido | Jugador y jugadas | Un vídeo vertical 9:16 que reúne las jugadas | Estilo, música, textos, revisión y aprobación |
| Vídeo largo de CS2 | Demo `.dem` | Jugador; todas las rondas del plan | Un vídeo horizontal 16:9 | HUD nativo, voces del equipo, tema de overlays y aprobación |
| Clips de stream | URL o MP4 | Intervalos de la grabación | Un Short 9:16 independiente por corte | Encuadre, facecam cuando corresponde, banners/música opcionales y aprobación |

La entrada debe nombrar el resultado antes de pedir el archivo. Las demos comparten importación y análisis; sus dos constructores conservan contratos y ajustes independientes. Los streams parten de vídeo existente y no requieren grabar una demo con CS2.

## Hallazgos y cambios

| Prioridad | Superficie | Problema observado | Resolución |
| --- | --- | --- | --- |
| Alta | Inicio vacío | Solo ofrece una demo; el usuario con un VOD debe descubrir otra sección. | Tres entradas visibles con origen, formato y resultado; se conserva la carga directa por arrastre. |
| Alta | Inicio con contenido | La guía de primera ejecución ocupa espacio sin distinguir bien los resultados. | Accesos directos permanentes a los tres recorridos, dentro del espacio de trabajo. |
| Alta | Importación | La intención no existe en la URL de carga; el camino corto vuelve a la lista tras el análisis. | `formato` se conserva en la carga y recuperación del jugador. Una demo individual abre el constructor elegido. |
| Alta | Selector de jugador | «Grabar Full POV» aparece antes de la revisión y junto a «Parsear POV». | Un siguiente paso según el objetivo: continuar al Short o al vídeo largo. No inicia captura. |
| Alta | Constructores | Cambiar de formato desmonta el constructor y borra las opciones. | Ambos conservan su estado durante el cambio; selección, tema y aprobación pertenecen a cada formato. |
| Media | Carga | No se explica la posición dentro del recorrido. | Indicador de cargar demo → elegir jugador → preparar vídeo, derivado del estado real. |
| Media | Tipos de vídeo | El selector solo muestra relaciones de aspecto y «Full POV». | Tarjetas comparables: jugadas seleccionadas frente a todas las rondas, vertical frente a horizontal. |
| Media | Streams | Campos identificados por placeholders y botón «Traer clip», incluso para VOD completo. | Etiquetas persistentes e «Importar vídeo»; explicación de un Short por corte. |
| Media | Streams | Se muestra cero proyectos mientras aún no hay datos o falló la carga. | El recuento aparece únicamente cuando existe una lista obtenida del servicio. |
| Media | Constructores en móvil | La barra fija ocupa parte de la pantalla y tapa opciones. | En contenedores estrechos la revisión permanece en el flujo del documento, con acciones apilables. |
| Media | Edición de streams | La banda de aprobación y acciones exige una anchura grande. | Los elementos se apilan y las acciones pueden pasar de línea en contenedores estrechos. |
| Media | Revisión | «Brief creativo» no explica que es el paso previo a producir. | «Revisar antes de crear», manteniendo el contenido y las condiciones de aprobación. |
| Media | Guardado | «Local + servidor» expone arquitectura de una aplicación local. | Mensaje de borrador guardado en este PC, con estado diferenciado cuando falta guardar. |
| Media | Navegación | «Stream clips» y «Players» mezclan idioma y términos. | «Clips de stream» y «Jugadores». |
| Media | Estado global | «Todo listo» se muestra incluso con el servicio local desconectado. | «Sin trabajos activos»: describe la cola, sin afirmar disponibilidad para capturar. |
| Alta | Interacción inicial | Las pruebas detectaron selecciones de archivo antes de que el componente estuviera interactivo. | El campo de carga permanece deshabilitado hasta que React conecta sus eventos; las pruebas esperan ese estado. |

## Recorrido del resto del producto

- **Partidas y resultados:** la partida sigue siendo el punto de agrupación; la vista de vídeos mantiene filtros, aspectos reales, descarga, borrado y acceso a publicación. La nueva cabecera aclara que los resultados de esa vista proceden de demos. Los proyectos de stream mantienen su propia lista y recuperación.
- **Series:** una serie conserva la selección conjunta de jugador. Se informa de que el formato se elige por mapa después del análisis; no se presupone que un vídeo largo pueda reunir varias demos mediante el constructor individual.
- **Steam:** la importación por código y el historial siguen disponibles en carga. No se abre una sesión de juego por visitar el inicio ni se cambian credenciales.
- **Jugadores:** su función es encontrar partidas en FACEIT y acceder a la descarga manual, no crear vídeo directamente. Conserva estados de configuración ausente y servicio sin conexión.
- **Táctica y Anti-cheat:** son recorridos de análisis de una demo, separados de producir vídeo. Se mantiene la advertencia de que la detección de anomalías no es un veredicto.
- **Ajustes:** mantiene cuenta de Steam, diagnóstico y configuración. La disponibilidad de captura sigue informándose desde el shell.
- **Publicación:** el MP4 y los metadatos se revisan localmente. La carga y programación en YouTube se hacen en YouTube Studio; no se añade publicación automática.

## Invariantes y límites

Los planes del backend siguen siendo la autoridad para rangos, rondas, formato y configuración efectiva. No se modifica el parser, la cola, los endpoints de captura/render ni sus permisos. Cambiar el tipo de vídeo es navegación; confirmar jugador solo analiza. La aprobación se invalida cuando cambia la configuración del constructor correspondiente.

El estado de los dos constructores se conserva mientras la página está montada, no tras recargar o cerrar la aplicación. El autoguardado existente del editor de streams se mantiene separado. Las lecturas y efectos conservan sus limpiezas al desmontarse.

## Próximos problemas a resolver

1. Persistir borradores de demos después de recargar, con revisión del plan y sin reutilizar una aprobación antigua. Requiere un contrato de recuperación, no solo guardar controles sueltos.
2. Revisar el lenguaje de operaciones avanzadas y metadatos históricos, como «REC» y «QA», conservando los códigos técnicos necesarios para diagnosticar fallos.
3. Ofrecer una vista conjunta de entregas de streams y demos respetando sus modelos y URLs diferentes; actualmente cada origen conserva sus propios resultados.
4. Revisar con usuarios reales el tamaño de las tarjetas del inicio cuando hay muchas partidas, y medir tiempo hasta la primera creación y abandonos por etapa. Esta revisión no inventa métricas de mejora.

## Verificación

Se verifica la interfaz en una compilación de producción con Playwright CLI y pruebas E2E que simulan respuestas en el límite HTTP. Los estados sin servicio se verifican sin simular un backend disponible. Se miden 390, 768, 1024, 1280, 1440 y 1920 px. Resultados: lint, TypeScript y pruebas unitarias correctos; compilación de producción correcta. Pasaron 98 pruebas E2E de hub, importación, constructores, biblioteca, streams, shell y responsive. Tras los últimos ajustes del editor de streams se repitieron sus suites afectadas: 62 pruebas correctas.

Playwright CLI verificó inicio, carga, importación de streams y los tres constructores en seis anchos, sin desbordamiento horizontal. También se comprobó ampliación CSS al 200% en cinco pantallas, sin desbordamiento. Las capturas de escritorio y móvil se revisaron visualmente. No se registraron excepciones JavaScript en los recorridos instrumentados; los errores HTTP 503 corresponden al servicio local ausente y los 404 de medios a las fuentes no incluidas en las fixtures. No se interpreta esa simulación como reproducción multimedia correcta.

Evidencias locales (fuera de Git): `/tmp/cliphub-after.png`, `/tmp/cliphub-short-desktop.png`, `/tmp/cliphub-full-desktop.png`, `/tmp/cliphub-full-mobile.png`, `/tmp/cliphub-stream-editor-desktop.png`, `/tmp/cliphub-stream-editor-mobile.png`. Logs: `/tmp/cliphub-e2e.log`, `/tmp/cliphub-final-e2e.log` y `/tmp/cliphub-final-build.log`. Las sesiones de navegador de verificación se cerraron.

No se ha realizado captura ni render real en Windows. La comprobación con Studio + CS2/HLAE (`zv verify doctor` / `prove`) requiere ese entorno y autorización para usarlo. Los tests de navegador no certifican el resultado multimedia.

### Correcciones de la review de Bugbot

Se corrigieron dos hallazgos: los contenedores de los constructores vuelven a ser columnas flexibles para situar la barra de creación al fondo, y un Short ya inicializado permanece montado cuando una actualización temporal devuelve un plan vacío o falla. Mientras no hay jugadas, se muestra el estado vacío/error y se oculta el constructor; no se ofrece crear con un plan vacío.

Las pruebas reprodujeron los fallos antes de corregirlos. Después pasaron 18 E2E de constructores y estados, incluidas cuatro regresiones nuevas para posición de las barras y conservación de selección/música tras actualizaciones vacías o fallidas. Lint, TypeScript, unitarios y build correctos. Se repitió la inspección visual con Playwright CLI en escritorio/móvil y seis anchos, sin desbordamiento ni excepciones JavaScript en los recorridos instrumentados.
