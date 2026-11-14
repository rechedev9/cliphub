# Spec: nueva landing de ClipHub

Estado: ready-for-agent

## Problem Statement

ClipHub ya tiene análisis de CS2 en dos sitios: la app de Windows (Studio) y el navegador (ClipHub Analytics, antes ClipMind). En los dos es el mismo análisis, y Studio añade las herramientas de clipping para creadores. La landing actual no lo refleja. Es una escena 3D de un ciber que solo vende la descarga de Windows con el mensaje "Gratis y en local". No enseña que el análisis funciona en el navegador, no muestra el producto de forma clara y su estética (cinta de carrocero, letra tipo marker, 3D) es decorado, no producto.

El competidor directo, cs2lens, no tiene landing. Su home es una tabla de partidos pro y sus funciones están en /about como nombres sueltos sin explicar. Cobra 19 $/mes por usuario y 79 $/mes para organizaciones (el plan que incluye las comms de voz). Su plan gratuito corta el partido en la ronda 12. Un coach, analista o IGL hispanohablante que busca una herramienta para preparar partidos no tiene hoy una página que le diga, en su idioma y con el producto real delante, que ClipHub hace más que cs2lens y gratis.

## Solution

Una landing nueva, en español, en la misma URL (cliphub.rechedev.cloud). Primero el producto, en la línea de cs2lens, pero explicada. Arriba va un titular de una línea con dos acciones ("Abrir en el navegador" y "Descargar para Windows") y, sin hacer scroll, una vista grande del producto real que se cambia por pestañas (Replay, Recorridos, Bombsites, Preparar, Probabilidad). Debajo van secciones concretas sobre lo que hace ClipHub para preparar a un rival, revisar las comms y trabajar en equipo. Después, una tabla frente a "otras herramientas" sin nombrar marcas, un bloque para creadores con lo exclusivo de Windows y la sección de descarga.

La estética es la de la app: fondo casi negro, verde de marca, azul CT y ámbar T, Saira con IBM Plex Mono para los números. Se ilustra con capturas reales anotadas como callouts y con los vídeos en bucle que ya existen. No lleva degradados, brillos, bentos genéricos, iconos de chispas, 3D ni textos vagos. ClipHub es un solo producto y todo es gratis.

## User Stories

1. Como coach de un equipo semipro, quiero entender en una frase qué es ClipHub nada más entrar, para decidir si sigo leyendo.
2. Como coach, quiero ver el producto real en el primer pantallazo, para saber que es una herramienta de verdad y no una promesa.
3. Como coach, quiero cambiar entre vistas del producto (replay, scouting, playbook, veto, informes) sin salir del hero, para ver de un vistazo lo que me interesa.
4. Como analista, quiero saber que puedo usar el análisis desde el navegador sin instalar nada, para probarlo en el momento.
5. Como analista, quiero abrir ClipHub Analytics con un solo clic desde la landing, para empezar a crear mi cuenta.
6. Como jugador con Windows, quiero un botón de descarga visible con la versión y el tamaño del instalador, para saber lo que me bajo.
7. Como usuario de Windows, quiero ayuda para el aviso de SmartScreen, para instalar sin miedo.
8. Como IGL, quiero ver cómo ClipHub me muestra las tendencias de un rival por mapa y lado, para preparar el anti-strat.
9. Como IGL, quiero ver el uso de bombsites del rival con porcentajes reales, para saber dónde va a ejecutar.
10. Como IGL, quiero ver el playbook detectado de un equipo, para reconocer sus jugadas repetidas.
11. Como coach, quiero ver el simulador de veto, para planificar qué mapas banear y elegir.
12. Como coach, quiero ver la pantalla de "preparar partido", para saber que tengo un flujo de preparación completo.
13. Como coach, quiero saber que puedo exportar informes de scouting en HTML o PDF, para pasárselos al equipo.
14. Como coach, quiero saber que ClipHub extrae y transcribe las comms de voz sincronizadas con cada ronda, para revisar la comunicación.
15. Como coach, quiero saber que las comms están incluidas gratis, para compararlo con herramientas que las cobran.
16. Como capitán, quiero saber que puedo crear un espacio de equipo e invitar a mis jugadores, para trabajar juntos.
17. Como miembro del equipo, quiero saber que la libreta (stratbook) es compartida, para que todos vean las mismas jugadas.
18. Como usuario de las dos versiones, quiero saber que web y escritorio se sincronizan, para no duplicar trabajo.
19. Como analista, quiero ver el replay 2D con heatmap, para entender cómo se analiza una ronda.
20. Como analista, quiero saber que veo el partido completo y no solo la primera mitad, para comparar con otras herramientas.
21. Como coach, quiero saber que puedo analizar partidos pro además de los míos, para estudiar a equipos de primer nivel.
22. Como coach, quiero ver lineups pro por mapa, para enseñar granadas al equipo.
23. Como visitante que compara herramientas, quiero una tabla clara de lo que incluye ClipHub frente a otras herramientas, para decidir rápido.
24. Como visitante que compara, quiero ver que todo es gratis, para no tener que buscar una página de precios.
25. Como creador de contenido, quiero una sección propia que me diga qué gano con la app de Windows, para saber si me interesa.
26. Como creador, quiero ver un ejemplo en vídeo de un Short vertical generado desde una demo, para juzgar la calidad.
27. Como creador, quiero saber que puedo renderizar una demo completa con HUD de retransmisión, para publicar partidos enteros.
28. Como streamer, quiero saber que puedo sacar clips de mis streams de Twitch, YouTube o Kick, para publicar sin editar a mano.
29. Como creador, quiero saber que las herramientas de clipping son solo de Windows, para no buscarlas en la web.
30. Como visitante desde el móvil, quiero que la landing se lea bien sin scroll horizontal, para poder evaluarla desde el teléfono.
31. Como visitante desde el móvil, quiero que las capturas se puedan leer o ampliar, para ver el producto en pantalla pequeña.
32. Como visitante con conexión lenta, quiero que la página cargue rápido y que los vídeos no bloqueen la lectura, para no irme.
33. Como visitante que prefiere menos movimiento, quiero que los vídeos y las animaciones respeten esa preferencia, para navegar cómodo.
34. Como visitante con lector de pantalla, quiero que las capturas tengan texto alternativo descriptivo y que las pestañas sean accesibles con teclado, para usar la página.
35. Como persona que comparte el enlace en Discord o X, quiero una vista previa con imagen y descripción correctas, para que el enlace se entienda.
36. Como visitante que busca en Google "análisis de demos CS2", quiero encontrar la landing con un título y una descripción claros, para llegar a ClipHub.
37. Como visitante escéptico, quiero ver datos reales de uso (usuarios, versiones publicadas), para confiar en el producto.
38. Como visitante, quiero navegar a cada sección desde la barra superior, para ir directo a lo que busco.
39. Como visitante, quiero un footer con el repositorio y los enlaces básicos, para saber quién está detrás.
40. Como mantenedor, quiero que la versión y el enlace del instalador sigan viniendo de un único sitio, para que cada release solo cambie ese punto.
41. Como mantenedor, quiero que la landing no dependa de three.js ni de modelos 3D, para que pese menos y sea más fácil de mantener.

## Implementation Decisions

- La landing sigue siendo el paquete `landing` independiente, con su stack actual (Next 15, React 19, Tailwind v4). Se rehace la página entera y se mantiene la URL pública.
- Se eliminan la escena 3D del ciber, su lógica, el modelo GLB, sus fotogramas, la dependencia de three.js y las fuentes que dejan de usarse (Chakra Petch, Share Tech Mono, Permanent Marker). Todo en el mismo cambio.
- Se adoptan los tokens visuales de la app Studio: fondo casi negro, verde de marca, rojo anaranjado de acento, azul CT y ámbar T, Saira para títulos y texto e IBM Plex Mono para números y callouts. Se copian como tokens propios de la landing, sin importar nada del paquete `web`.
- Orden de las secciones:
  1. Barra superior: marca, enlaces a Análisis, Creadores y Descargar, botón "Abrir ClipHub".
  2. Hero: titular de una línea sobre análisis de CS2 para preparar partidos, gratis. CTA principal "Abrir en el navegador" (ClipHub Analytics) y CTA secundario "Descargar para Windows". Visor de producto con pestañas Replay, Recorridos, Bombsites, Preparar y Probabilidad, elegidas entre las capturas que muestran datos reales (las de Playbook, Veto e Informes salen vacías y se explican en texto).
  3. Preparar al rival: scouting, bombsites, playbook, veto y preparar partido.
  4. Comms de voz: extracción y transcripción sincronizadas con la ronda.
  5. Trabajo en equipo: espacios de equipo, invitaciones, libreta compartida, sincronización entre web y escritorio.
  6. Tabla comparativa frente a "otras herramientas", sin nombrar marcas: precio, partido completo, comms, informes, veto, playbook, partidos pro, vídeo.
  7. Creadores (solo Windows): Shorts, demo completa con HUD, clips de stream, con los vídeos en bucle existentes.
  8. Descarga: versión, tamaño, requisitos y ayuda de SmartScreen, reutilizando el contenido actual.
  9. Footer.
- El visor de producto del hero es un componente de pestañas accesible (roles de tabs, navegación con teclado). Cada pestaña muestra una captura real con dos o tres callouts anotados en mono con datos concretos.
- Marca: un solo producto, ClipHub. La versión web se llama ClipHub Analytics y su logotipo es "ClipHub" con "analytics" en letra pequeña en mono. La landing usa ese logotipo cuando se refiere a la versión web y "ClipHub" a secas en el resto del texto. "ClipMind" no aparece en ninguna parte de la landing.
- Las capturas salen de las 32 capturas de análisis ya hechas. Se recortan y optimizan (WebP o AVIF con tamaño intrínseco declarado para no mover el layout). La primera captura del hero tiene prioridad de carga y el resto carga en diferido.
- Los vídeos de creadores reutilizan el componente de vídeo ligero existente: póster estático, sin autoplay si el usuario prefiere menos movimiento.
- El contenido (textos de secciones, filas de la tabla, pestañas del visor, cifras de prueba social) vive en un módulo de datos tipado, separado del marcado. Así el test de contenido y la página leen la misma fuente.
- La versión, la URL del instalador y el tamaño siguen en el módulo de constantes del sitio, que ya actualiza cada release. Se le añade la URL de ClipHub Analytics, `https://app.cliphub.rechedev.cloud`, como constante única para poder cambiarla en una línea.
- La prueba social usa solo datos reales del README ("100+ usuarios", "130+ releases"). No hay testimonios ni logos de equipos inventados.
- No hay sección de precios. El mensaje es "Gratis" en el hero y en la tabla.
- No se vende "IA" en el texto. El servidor MCP para agentes puede aparecer como una línea entre las funciones, nunca como titular.
- Se actualizan los metadatos (título, descripción, imagen Open Graph) en español con el nuevo posicionamiento. Se mantiene la generación de la imagen OG existente, con el nuevo diseño.
- Cada archivo fuente queda en 1000 líneas o menos (regla `max-lines` del repo). El lint nombra carpetas concretas, como ya hace el script actual.

## Testing Decisions

- Un buen test aquí comprueba solo lo que ve el visitante en la página compilada, no cómo están hechos los componentes. No se prueban clases CSS, estructura interna ni detalles de React.
- Hay un único punto de prueba, el más alto posible: el HTML que genera `next build` para la home. Un test de contenido lee ese HTML prerenderizado y comprueba:
  - El titular y los dos CTA, con sus destinos (la URL de ClipHub Analytics y la URL del instalador del módulo de constantes).
  - Que cada sección acordada existe, con su ancla de navegación.
  - Que la tabla comparativa no nombra a ningún competidor.
  - Que el texto "ClipMind" no aparece en la página.
  - Que cada imagen tiene texto alternativo no vacío.
  - Que los metadatos (título, descripción, Open Graph) están en español y no vacíos.
- El test usa el runner nativo de Node (`node --test` sobre TypeScript), igual que los tests unitarios del paquete `web`. No añade dependencias. `landing` gana un script `test` que se ejecuta después del build.
- Antecedente en el repo: los tests `*.test.ts` del paquete `web`, que corren con `node --test`.
- Además: `typecheck`, `lint` y `build` de `landing` en verde, y una revisión manual en el navegador a 375 px y en escritorio, con capturas para el usuario. Sin suite e2e, porque `AGENTS.md` lo prohíbe.

## Out of Scope

- Una demo pública de partidos pro abierta sin cuenta en ClipHub Analytics. Hoy la versión web solo tiene rutas públicas de cuenta (login, registro, recuperar contraseña), así que hace falta trabajo de backend. Es el siguiente paso natural.
- Versión en inglés.
- Planes de pago o página de precios.
- Cambios funcionales en la app Studio o en ClipHub Analytics.
- El cambio de marca de ClipMind a ClipHub Analytics dentro de la app (UI de `web/`, emails del servidor, docs). Va en un PR propio y pequeño, solo en texto visible para el usuario, sin renombrar paquetes, rutas `/api/cm`, el binario `clipmind-server` ni variables de entorno.
- Actualizar `AGENTS.md`, que todavía dice que `web/` no es un producto web. Va en un cambio aparte.
- Deploy a producción, que se hace solo con aprobación explícita.

## Further Notes

- Contexto de la competencia (revisado el 2026-11-14): la home de cs2lens es una tabla de partidos con un botón "Replay". Sus funciones están en /about sin descripción. Se registra con email y contraseña, sin Steam ni FACEIT. Cobra el plan Org por Discord o email. Pagan con Gumroad, sin plan anual. Free corta en la ronda 12. No tiene capturas ni vídeo del producto y no funciona bien en móvil. Su única prueba social es una cita de un coach.
- Refrag y Scope.gg sí tienen landing clásica para jugadores, con cifras grandes y "Sign up for free". ClipHub no compite en ese terreno: su público es el equipo.
- La URL de ClipHub Analytics se fija en `https://app.cliphub.rechedev.cloud` (la landing sigue en `cliphub.rechedev.cloud`). Si el despliegue acaba en otro dominio, solo cambia la constante.
