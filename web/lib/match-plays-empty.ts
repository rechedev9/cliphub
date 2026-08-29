import { PLAN_READY_STATUSES } from './api/types.ts';

/** Pending parse must never share copy with a genuine zero-plays empty state. */
export const MATCH_PLAYS_ANALYZING_TITLE = 'Analizando la demo…';
export const MATCH_PLAYS_ANALYZING_DESCRIPTION =
  'Las jugadas aparecerán aquí cuando termine el parseo.';

export const MATCH_PLAYS_EMPTY_TITLE = 'Sin jugadas destacables';
export const MATCH_PLAYS_EMPTY_DESCRIPTION =
  'El análisis no encontró ninguna jugada digna de highlight en esta partida. Prueba con otra demo.';

export const MATCH_PLAYS_ERROR_TITLE = 'No se pudieron cargar las jugadas';
export const MATCH_PLAYS_ERROR_DESCRIPTION =
  'Hubo un error al leer el plan de esta partida. Vuelve a intentarlo o elige otra demo.';

/** True when the job status has a kill plan (or status is unknown — fixtures omit it). */
export function matchPlanReady(status: string | undefined): boolean {
  return status === undefined || PLAN_READY_STATUSES.has(status);
}
