/**
 * Studio tour copy. A plain module so the unit test and the e2e contract can
 * import the same strings the layer renders.
 *
 * Every nav section must own at least one chapter (enforced in
 * app-tour.test.ts): a section added to the rail without a chapter here would
 * leave the tour describing an app that no longer exists.
 */
import { CLIPS_HREF, NEW_DEMO_HREF } from './clips/routes.ts';

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
      'ClipHub Studio convierte una demo de CS2 en un Short vertical o en un vídeo largo grabado desde la vista real de un jugador. También recorta tus streams en Shorts y analiza demos.',
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
        text: 'Recorre cada sección de la barra lateral. Puedes volver a abrirla cuando quieras con el botón «Guía» de la barra superior.',
      },
    ],
  },
  {
    id: 'load',
    kicker: 'Clips y vídeos',
    label: 'Cargar una partida',
    title: 'Todo empieza con una partida',
    lead:
      'Una partida es una demo cargada en ClipHub más el jugador desde cuya vista se grabará el vídeo. Pulsa «Cargar demo» y elige primero el formato: «Vídeo largo 16:9» o «Short 9:16».',
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
    ],
    link: { href: NEW_DEMO_HREF, label: 'Cargar una demo' },
  },
  {
    id: 'short',
    kicker: 'Clips y vídeos',
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
  },
  {
    id: 'full',
    kicker: 'Clips y vídeos',
    label: 'Vídeo largo',
    title: 'La partida entera desde un jugador',
    lead:
      'El vídeo largo 16:9 graba todas las rondas desde la vista del jugador, con el audio de la partida equilibrado automáticamente y sin música de fondo.',
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
        term: 'Intro, outro y sponsor',
        text: 'Añade tus propios MP4 al principio y al final, y un vídeo de sponsor si tienes sus derechos de uso.',
      },
    ],
  },
  {
    id: 'deliver',
    kicker: 'Clips y vídeos',
    label: 'Descargar y publicar',
    title: 'Tus vídeos, listos para subir',
    lead:
      'Todo lo que creas aparece en «Clips y vídeos». Cambia entre la vista «Partidas» y la vista «Clips» para ver cada vídeo con su estado: en cola, grabando, en edición, listo o fallido.',
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
    link: { href: `${CLIPS_HREF}?vista=clips`, label: 'Ver mis vídeos' },
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
    id: 'players',
    kicker: 'Jugadores',
    label: 'Jugadores',
    title: 'Sigue jugadores de FACEIT',
    lead:
      'Busca por nick o URL de FACEIT y añádelo a «Siguiendo». Verás su nivel, victorias, K/D, ADR, headshots y su historial de partidas filtrable por mapa y resultado.',
    points: [
      {
        term: 'De la partida al clip',
        text: 'Abre la sala en FACEIT, descarga la demo y súbela a ClipHub para crear el vídeo.',
      },
      {
        term: 'Requiere FACEIT',
        text: 'Esta sección necesita que el servicio local tenga configurada la conexión con FACEIT. Si no lo está, la página te lo indica.',
      },
    ],
    link: { href: '/players', label: 'Ir a Jugadores' },
  },
  {
    id: 'tactical',
    kicker: 'Táctica',
    label: 'Táctica',
    title: 'Análisis táctico de una partida',
    lead:
      'Sobre una demo ya cargada, pulsa «Analizar». El análisis se hace en local, una sola vez, y el resultado queda guardado.',
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
    ],
    link: { href: '/tactical', label: 'Ir a Táctica' },
  },
  {
    id: 'cheaters',
    kicker: 'Anti-cheat',
    label: 'Anti-cheat',
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
    link: { href: '/cheaters', label: 'Ir a Anti-cheat' },
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
