/** Produce-screen empty state when `getMatch` returns nothing (404 / gone from disk). */
export const PRODUCE_MATCH_MISSING = {
  title: 'Partida no encontrada',
  description: 'Esta partida ya no está en este PC. Puede que se haya borrado con sus artefactos.',
} as const;

/** Browser titles of the producer route, one per format. */
export const PRODUCE_DOCUMENT_TITLE = { short: 'Nuevo Short', full: 'Nuevo vídeo largo' } as const;

/** Each format is its own flow: the section name, its home copy and its steps. */
export const FLOW_TITLE = { short: 'Shorts', full: 'Vídeos largos' } as const;
export const FLOW_HOME_TITLE = { short: 'Crea un Short', full: 'Crea un vídeo largo' } as const;
export const FLOW_HOME_DESCRIPTION = {
  short: 'Vídeo vertical 9:16 con las mejores jugadas de un jugador. Carga una demo de CS2, elige el jugador y revisa las jugadas antes de grabar.',
  full: 'Vídeo horizontal 16:9 con todas las rondas desde la vista de un jugador. Carga una demo de CS2, elige el jugador y revisa los ajustes antes de grabar.',
} as const;
export const FLOW_STEPS = {
  short: ['Cargar demo', 'Elegir jugador', 'Preparar Short', 'Grabar y descargar'],
  full: ['Cargar demo', 'Elegir jugador', 'Preparar vídeo largo', 'Grabar y descargar'],
} as const;
export const FLOW_PRODUCE_CTA = {
  short: { first: 'Crear Short', again: 'Crear otro Short' },
  full: { first: 'Preparar vídeo largo', again: 'Preparar otro vídeo largo' },
} as const;

export const PRODUCE_DRAFT_RESET = 'Empezar de cero';

export const PRODUCE_SHORT_TITLE = 'Prepara tu Short';
export const PRODUCE_SHORT_EMPTY_HINT = 'Elige al menos una jugada';
/** The Short draft lives in sessionStorage; the long-video one in localStorage, so it outlives the session. */
export const PRODUCE_SHORT_DRAFT_RESTORED = 'Configuración recuperada de tu última visita en esta sesión.';
export const PRODUCE_SHORT_DRAFT_RESET = PRODUCE_DRAFT_RESET;
export const PRODUCE_SHORT_CTA = 'Crear Short';

/** Footer line once a producer can create: where it records and where the result lands. */
export const PRODUCE_READY_HINT = 'Todo preparado. Al crear, ClipHub grabará en este PC; encontrarás el vídeo en Resultados.';

/** The long-form flow is "vídeo largo" everywhere the user reads it; "Full Demo" stays internal. */
export const PRODUCE_FULL_TITLE = 'Prepara tu vídeo largo';
export const PRODUCE_FULL_DRAFT_RESTORED = 'Configuración recuperada de tu última visita.';
export const PRODUCE_FULL_CTA = 'Crear vídeo largo';
export const PRODUCE_FULL_QUEUE_CTA = 'Poner vídeo largo en cola';

/** A poll tick failed while the partida is already on screen: warn, keep the content. */
export const PRODUCE_POLL_ERROR = 'No se pudo actualizar esta partida. Seguimos mostrando los últimos datos cargados.';
export const PRODUCE_POLL_OFFLINE = 'Servicio local sin conexión. Seguimos mostrando los últimos datos cargados.';

/** `scanned` partida: roster read, no POV picked. Settled, so it never advances on its own. */
export const PRODUCE_MATCH_NO_POV = {
  title: 'Elige de quién será el vídeo',
  description: 'La demo ya está cargada. Elige un jugador para analizar sus jugadas y preparar el vídeo.',
} as const;
export const PRODUCE_PICK_POV_CTA = 'Elegir jugador';

/** Creating a Short failed in transport or on the server; domain errors keep their own text. */
export const PRODUCE_SHORT_CREATE_OFFLINE = 'El servicio local no responde. Comprueba que ClipHub sigue abierto y vuelve a intentarlo.';
export const PRODUCE_SHORT_CREATE_ERROR = 'No se pudo crear el Short. Vuelve a intentarlo.';
