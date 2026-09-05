/** Produce-screen empty state when `getMatch` returns nothing (404 / gone from disk). */
export const PRODUCE_MATCH_MISSING = {
  title: 'Partida no encontrada',
  description: 'Esta partida ya no está en este PC. Puede que se haya borrado con sus artefactos.',
} as const;

export const PRODUCE_SHORT_TITLE = 'Prepara tu Short';
export const PRODUCE_FULL_TITLE = 'Prepara tu vídeo largo';
export const PRODUCE_SHORT_EMPTY_HINT = 'Elige al menos una jugada';
/** Round omission is not an API capability; the Full POV always records the whole plan. */
export const PRODUCE_FULL_ROUNDS_NOTE = 'El vídeo largo incluye todas las rondas del plan, en orden';

/** A poll tick failed while the partida is already on screen: warn, keep the content. */
export const PRODUCE_POLL_ERROR = 'No se pudo actualizar esta partida. Seguimos mostrando los últimos datos cargados.';
export const PRODUCE_POLL_OFFLINE = 'Servicio local sin conexión. Seguimos mostrando los últimos datos cargados.';

/** `scanned` partida: roster read, no POV picked. Settled, so it never advances on its own. */
export const PRODUCE_MATCH_NO_POV = {
  title: 'Elige de quién será el vídeo',
  description: 'La demo ya está cargada. Elige un jugador para analizar sus jugadas y preparar el vídeo.',
} as const;
export const PRODUCE_PICK_POV_CTA = 'Elegir jugador';
