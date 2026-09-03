/** Produce-screen empty state when `getMatch` returns nothing (404 / gone from disk). */
export const PRODUCE_MATCH_MISSING = {
  title: 'Partida no encontrada',
  description: 'Esta partida ya no está en este PC. Puede que se haya borrado con sus artefactos.',
} as const;

export const PRODUCE_SHORT_TITLE = 'Elige los highlights';
export const PRODUCE_FULL_TITLE = 'Toda la partida, tu POV';
export const PRODUCE_SHORT_EMPTY_HINT = 'Elige al menos un highlight';
/** Round omission is not an API capability; the Full POV always records the whole plan. */
export const PRODUCE_FULL_ROUNDS_NOTE = 'El Full POV graba todas las rondas del plan';

/** A poll tick failed while the partida is already on screen: warn, keep the content. */
export const PRODUCE_POLL_ERROR = 'No se pudo actualizar esta partida. Seguimos mostrando los últimos datos cargados.';
export const PRODUCE_POLL_OFFLINE = 'Servicio local sin conexión. Seguimos mostrando los últimos datos cargados.';

/** `scanned` partida: roster read, no POV picked. Settled, so it never advances on its own. */
export const PRODUCE_MATCH_NO_POV = {
  title: 'Esta partida no tiene POV elegida',
  description: 'El roster está listo, pero no se eligió el jugador a clipear. Elige su POV y parseamos la partida.',
} as const;
export const PRODUCE_PICK_POV_CTA = 'Elegir jugador';
