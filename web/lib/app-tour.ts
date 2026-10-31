/**
 * Studio tour copy. A plain module so the unit test and the e2e contract can
 * import the same strings the layer renders.
 *
 * Every nav section must own at least one chapter (enforced in
 * app-tour.test.ts): a section added to the rail without a chapter here would
 * leave the tour describing an app that no longer exists.
 */
import { LONG_VIDEOS_HREF, RESULTS_HREF, SHORTS_HREF } from './clips/routes.ts';

export const TOUR_TITLE = 'Guía de ClipHub Studio';

export interface TourPoint {
  readonly term: string;
  readonly text: string;
}

export interface TourChapter {
  readonly id: string;
  /** Rail label of the section it explains, or a short tag outside the rail. */
  readonly kicker: string;
  /** Short name for the chapter list. */
  readonly label: string;
  readonly title: string;
  readonly lead: string;
  readonly points: readonly TourPoint[];
  /** Where the chapter's work happens; closes the tour and navigates. */
  readonly link?: { readonly href: string; readonly label: string };
}

export const TOUR_CHAPTERS: readonly TourChapter[] = [
  {
    id: 'welcome',
    kicker: 'ClipHub',
    label: 'Qué es ClipHub',
    title: 'De tus partidas de CS2 a vídeos listos para subir',
    lead:
      'ClipHub Studio convierte una demo de CS2 en un Short vertical o en una demo completa grabada desde la vista real de un jugador. También recorta tus streams en Shorts y analiza demos como un analista de equipo.',
    points: [
      {
        term: 'Todo en tu PC',
        text: 'Las demos se analizan, graban y editan en este equipo. No necesitas cuenta de ClipHub y tus archivos no se suben a ningún sitio.',
      },
      {
        term: 'Grabación real',
        text: 'ClipHub abre CS2 y graba la partida con HLAE, como un vídeo capturado a mano, pero sin que tengas que hacerlo tú.',
      },
      {
        term: 'Esta guía',
        text: 'Recorre cada sección de la barra lateral, dividida en dos áreas: Crear y Analizar. Puedes volver a abrirla cuando quieras con el botón «Guía» de la barra superior.',
      },
    ],
  },
  {
    id: 'load',
    kicker: 'Crear',
    label: 'Cargar una demo',
    title: 'Todo empieza con una demo',
    lead:
      'Cada formato tiene su sección en la barra lateral: «Demo completa» y «Short». Las dos empiezan igual: cargas una demo y eliges el jugador desde cuya vista se grabará el vídeo.',
    points: [
      {
        term: 'Archivo en mi PC',
        text: 'Arrastra una demo .dem, .dem.zst, .zip o .rar. Si juegas en FACEIT, descarga la demo desde la sala de la partida y cárgala aquí.',
      },
      {
        term: 'Importar desde Steam',
        text: 'Pega el código de partida (CSGO-…), pulsa «Comprobar» y «Descargar demo». Con Steam conectado en Ajustes verás también tus partidas recientes.',
      },
      {
        term: 'Elegir jugador',
        text: 'ClipHub escanea la demo y te muestra los diez jugadores con su rating, ADR, KAST y HS %. Elige de quién será el vídeo.',
      },
      {
        term: 'Series',
        text: 'Si cargas varias demos de un mismo bo3 o bo5, se agrupan en una serie: eliges al jugador una vez y creas un vídeo por mapa.',
      },
      {
        term: 'Demos cargadas',
        text: 'Las demos que ya analizaste quedan en la lista de cada sección, para crear otro vídeo sin volver a cargarlas.',
      },
    ],
  },
  {
    id: 'full',
    kicker: 'Demo completa',
    label: 'Demo completa',
    title: 'La partida entera desde un jugador',
    lead:
      'La demo completa 16:9 graba todas las rondas desde la vista del jugador, con el audio de la partida equilibrado automáticamente y sin música de fondo.',
    points: [
      {
        term: 'HUD de la partida',
        text: 'Elige entre once diseños de retransmisión o el HUD original de CS2, y activa el POV original 1:1 con TrueView.',
      },
      {
        term: 'Sonido',
        text: 'Puedes incluir las voces del equipo. El volumen se nivela solo para que el vídeo suene bien de principio a fin.',
      },
      {
        term: 'Entre rondas y overlays',
        text: 'Transiciones entre rondas y rótulos de inicio y final según el origen de la demo: FACEIT, Premier, Profesional o local.',
      },
      {
        term: 'Intro, sponsor y outro',
        text: 'Añade tus propios MP4 al principio y al final, y un vídeo de sponsor que se reproduce después de la ronda 2 si tienes sus derechos de uso.',
      },
    ],
    link: { href: LONG_VIDEOS_HREF, label: 'Ir a Demo completa' },
  },
  {
    id: 'short',
    kicker: 'Short',
    label: 'Crear un Short',
    title: 'Un Short vertical con las mejores jugadas',
    lead:
      'ClipHub encuentra las jugadas del jugador elegido y te guía en cuatro pasos. Al crear, graba cada jugada en CS2 y la monta en un vídeo 9:16.',
    points: [
      {
        term: 'Elige las jugadas',
        text: '«Auto: mejores 60 s» selecciona el mejor minuto por ti. Las rondas de cinco kills llevan la etiqueta ACE.',
      },
      {
        term: 'Estilo',
        text: 'Elige un estilo de montaje y compáralos antes de decidir.',
      },
      {
        term: 'Música',
        text: 'Los cortes caen al ritmo del beat. También puedes crearlo sin música.',
      },
      {
        term: 'Textos y gráficos',
        text: 'Efecto de kill, transiciones, título automático, contador de kills, texto de apertura y cierre, y portada.',
      },
    ],
    link: { href: SHORTS_HREF, label: 'Ir a Short' },
  },
  {
    id: 'streams',
    kicker: 'Clips de stream',
    label: 'Clips de stream',
    title: 'Recorta tus streams en Shorts',
    lead:
      'Pega un enlace de Twitch, YouTube o Kick, o sube un MP4. Esta sección trabaja sobre un vídeo ya grabado, así que no necesita CS2.',
    points: [
      {
        term: 'Elegir momentos',
        text: 'Marca cada corte que quieras. Cada momento se convierte en su propio Short.',
      },
      {
        term: 'Ajustar aspecto',
        text: 'Elige «Cámara + juego», «Cámara doble» o «Solo juego» y encuadra tu facecam.',
      },
      {
        term: 'Extras opcionales',
        text: 'Banner con tu nombre de streamer, música y efecto de color.',
      },
      {
        term: 'Borrador automático',
        text: 'Tu edición se guarda en este PC mientras trabajas, así que puedes dejarla y seguir después.',
      },
    ],
    link: { href: '/streams', label: 'Ir a Clips de stream' },
  },
  {
    id: 'deliver',
    kicker: 'Mis vídeos',
    label: 'Descargar y publicar',
    title: 'Tus vídeos, listos para subir',
    lead:
      'Todo lo que creas aparece en «Mis vídeos». Cada vídeo muestra su estado: en cola, grabando, en edición, listo o fallido.',
    points: [
      {
        term: 'Reproducir y descargar',
        text: 'Mira el resultado en la app y descarga el MP4. Si algo falla, puedes reintentarlo o volver a prepararlo.',
      },
      {
        term: 'Asistente de YouTube',
        text: '«Publicar» te propone título, descripción, etiquetas y buenas horas para subirlo. Lo copias y lo subes tú desde YouTube Studio: ClipHub no publica por ti.',
      },
      {
        term: 'Cambiar la música',
        text: 'En un Short puedes añadir o cambiar la canción sin volver a grabar.',
      },
    ],
    link: { href: RESULTS_HREF, label: 'Ver mis vídeos' },
  },
  {
    id: 'ideas',
    kicker: 'Ideas',
    label: 'Ideas',
    title: 'Qué hacer para llegar a más gente en YouTube',
    lead: 'Instrucciones cortas para tus Shorts y demos completas de CS2.',
    points: [
      {
        term: 'Por formato',
        text: 'Cómo empezar un Short, qué poner en título y miniatura, y cuándo publicar.',
      },
      {
        term: 'Qué evitar',
        text: 'Música que bloquea un Short, clips ajenos y enlaces a apuestas de skins.',
      },
    ],
    link: { href: '/ideas', label: 'Ir a Ideas' },
  },
  {
    id: 'matches',
    kicker: 'Partidas',
    label: 'Partidas',
    title: 'Tus partidas, analizadas ronda a ronda',
    lead:
      'Importa varias demos a la vez o usa las que cargas en Crear: todas forman tu biblioteca. Cada demo se analiza en local una sola vez y queda guardada con sus equipos, jugadores y rondas.',
    points: [
      {
        term: 'Rondas clasificadas',
        text: 'Cada ronda por tipo de compra (pistola, eco, semi, force, full) y por patrón de ataque o defensa, con etiquetas como ace o post-plant.',
      },
      {
        term: 'Repetición 2D',
        text: 'Revive cada ronda en un radar con línea de tiempo y lista de eventos.',
      },
      {
        term: 'Tendencias',
        text: 'Cómo juega cada equipo. Con pocas rondas ClipHub te avisa de que aún no es una tendencia.',
      },
      {
        term: 'Del análisis al vídeo',
        text: 'Desde una ronda, una jugada o una fila de un informe: graba el POV de un jugador, monta un vídeo de review con varias rondas o envíalas a un Short. El vídeo sale en Mis vídeos y en la ficha de la partida.',
      },
      {
        term: 'Series',
        text: 'Junta los mapas de un BO3 o un BO5 y apunta su veto, si lo conoces.',
      },
    ],
    link: { href: '/partidas', label: 'Ir a Partidas' },
  },
  {
    id: 'teams',
    kicker: 'Equipos',
    label: 'Equipos',
    title: 'La ficha de cada equipo',
    lead:
      'Cada equipo tiene su ficha con su plantilla por fecha, sus partidas y su rendimiento por mapa, la base del scouting de un rival.',
    points: [
      {
        term: 'Desde tus demos',
        text: 'Los equipos salen del nombre de clan de cada demo. Si uno sale mal, corrígelo en la partida y Studio lo recuerda.',
      },
      {
        term: 'Alias y fusiones',
        text: 'Un mismo equipo con dos nombres se fusiona en uno, y sus alias resuelven las demos que lleguen después.',
      },
      {
        term: 'Mapas y veto',
        text: 'El historial por mapa cuenta resultados, rondas por lado, pistolas, picks y bans. El simulador de veto recomienda cada paso de un BO1, BO3 o BO5 contra un rival y explica por qué.',
      },
    ],
    link: { href: '/equipos', label: 'Ir a Equipos' },
  },
  {
    id: 'players',
    kicker: 'Jugadores',
    label: 'Jugadores',
    title: 'La ficha de cada jugador',
    lead:
      'Cada jugador de tus demos tiene su ficha por SteamID64: sus nicks, sus equipos por fecha, sus partidas y el rol que se lee de sus rondas.',
    points: [
      {
        term: 'Rol con evidencia',
        text: 'AWPer, entry o soporte se deciden con umbrales a la vista, y cada cifra enlaza a las rondas de las que sale.',
      },
      {
        term: 'Métricas avanzadas',
        text: 'WPA, bajas ajustadas por economía, duelos justos e injustos, trades, utilidad y rendimiento por posición. También en la ficha de cada partida y en cada informe.',
      },
      {
        term: 'FACEIT',
        text: 'Vincula su cuenta de FACEIT o sigue jugadores desde Seguimiento FACEIT. Necesita la conexión con FACEIT del servicio local.',
      },
    ],
    link: { href: '/jugadores', label: 'Ir a Jugadores' },
  },
  {
    id: 'stratbook',
    kicker: 'Stratbook',
    label: 'Stratbook',
    title: 'Las jugadas que quieres recordar',
    lead:
      'Guarda una ronda, propia o del rival, como jugada: mapa, lado, granadas, posiciones y notas.',
    points: [
      {
        term: 'Guardar como jugada',
        text: 'En la repetición 2D de una partida, el botón Guardar como jugada toma las granadas y las posiciones del momento en que estás.',
      },
      {
        term: 'Jugadas repetidas',
        text: 'Detectar repetidas agrupa las rondas de un equipo con la misma utilidad y las mismas posiciones, con los umbrales a la vista.',
      },
      {
        term: 'Con su evidencia',
        text: 'Cada jugada enlaza a las rondas de ejemplo de las que sale, y sus humos se comparan con el catálogo de lineups.',
      },
      {
        term: 'Compartir',
        text: 'Exporta el stratbook como fichero JSON e impórtalo en otro PC con Studio.',
      },
    ],
    link: { href: '/stratbook', label: 'Ir a Stratbook' },
  },
  {
    id: 'reports',
    kicker: 'Informes',
    label: 'Informes',
    title: 'Informes de scouting para el coach',
    lead:
      'Elige un rival y las partidas que quieres leer: el informe junta sus pistolas, defaults, setups, ejecuciones, economía, AWP y aperturas.',
    points: [
      {
        term: 'Con evidencia',
        text: 'Cada fila enlaza a sus rondas de ejemplo, que abren el replay en la ronda y el tick exactos.',
      },
      {
        term: 'Para compartir',
        text: 'Exporta el informe como HTML o guárdalo en PDF, con las notas del coach al principio.',
      },
      {
        term: 'Sin nube',
        text: 'El informe se genera en este PC a partir de tus demos.',
      },
    ],
    link: { href: '/informes', label: 'Ir a Informes' },
  },
  {
    id: 'integrity',
    kicker: 'Integridad',
    label: 'Integridad',
    title: 'Detecta comportamientos anómalos',
    lead:
      'Suelta una demo y ClipHub puntúa a los diez jugadores. Es un detector de anomalías para decidir qué revisar a mano, no un veredicto.',
    points: [
      {
        term: 'Niveles',
        text: 'Muy anómalo, anómalo, no concluyente, sin anomalías o datos insuficientes, con los momentos que merece la pena revisar.',
      },
      {
        term: 'Expediente',
        text: 'En los casos anómalos puedes preparar un expediente con la evidencia para denunciarlo tú. ClipHub no envía nada.',
      },
      {
        term: 'Sin tocar tus partidas',
        text: 'El análisis no abre CS2 ni modifica la partida que tengas cargada.',
      },
    ],
    link: { href: '/integridad', label: 'Ir a Integridad' },
  },
  {
    id: 'settings',
    kicker: 'Ajustes',
    label: 'Ajustes',
    title: 'Prepara este PC y tus conexiones',
    lead: 'En Ajustes compruebas que todo está listo para grabar y gestionas las conexiones opcionales.',
    points: [
      {
        term: 'Grabación de demos',
        text: 'Comprueba CS2, HLAE y el grabador de ClipHub, y ajusta rutas personalizadas si las tienes en otro sitio.',
      },
      {
        term: 'Steam',
        text: 'Conecta tu historial para ver tus partidas recientes al cargar una demo. Tu contraseña nunca se guarda.',
      },
      {
        term: 'Motor de vídeo y diagnósticos',
        text: 'Revisa la aceleración por hardware, la versión instalada y decide si compartes diagnósticos de errores.',
      },
    ],
    link: { href: '/settings', label: 'Ir a Ajustes' },
  },
  {
    id: 'capture',
    kicker: 'Captura',
    label: 'Grabación y progreso',
    title: 'Mientras ClipHub graba',
    lead:
      'Para grabar una demo necesitas CS2 instalado en este PC; ClipHub Studio trae su propia copia de HLAE. Puedes cargar demos y preparar vídeos antes de tenerlo todo listo.',
    points: [
      {
        term: 'Estado de grabación',
        text: 'El indicador al pie de la barra lateral dice si CS2 y HLAE están listos. Púlsalo para ver qué falta.',
      },
      {
        term: 'Progreso en la barra superior',
        text: 'Ahí ves cada trabajo en marcha: descarga, parseo, REC, edición y cola.',
      },
      {
        term: 'No toques el juego',
        text: 'Durante el REC, CS2 está grabando: deja el juego a ClipHub hasta que termine. Los siguientes vídeos esperan en cola.',
      },
      {
        term: 'Actualizaciones',
        text: 'Cuando hay una versión nueva aparece «Actualizar». La instalación espera a que terminen la grabación y la edición.',
      },
    ],
    link: { href: '/settings#capture', label: 'Revisar requisitos de grabación' },
  },
];
