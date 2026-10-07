import { SERVICE_UNAVAILABLE_CODE } from './api/types.ts';

/**
 * Shown inline near a Partidas row when a delete fails because the local
 * analysis service is unreachable, matching the page's "offline" hint copy.
 */
export const DELETE_OFFLINE_MESSAGE = 'Servicio de análisis local desconectado. Arráncalo y reintenta.';

/** Fallback when the error carries no usable message (unexpected transport failure). */
export const DELETE_GENERIC_MESSAGE = 'No se pudo borrar. Inténtalo de nuevo.';

/** A stream project cannot be deleted while it downloads or renders (409 from DeleteStreamJob). */
export const DELETE_STREAM_BUSY_MESSAGE = 'Espera a que termine la descarga o el render para borrar este proyecto.';

/** A demo cannot be deleted while parse, scan, or capture still holds its files. */
export const DELETE_JOB_BUSY_MESSAGE = 'Espera a que termine el análisis o la captura para borrar esta partida.';

/** A demo cannot be deleted while a render or generate run is still writing its tree. */
export const DELETE_RENDER_BUSY_MESSAGE = 'Espera a que termine el render para borrar esta partida.';

const STREAM_BUSY_RE = /^stream (job|render \S+) is \w+; wait for it to settle before deleting$/;
/** DeleteJob refuses queued/scanning/parsing/recording/composing with this sentence. */
const JOB_BUSY_RE = /^job is \w+; wait for it to settle before deleting$/;
/** DeleteJob refuses while a render or generate run still writes the job tree. */
const JOB_RENDER_BUSY_RE = /^job has an active render or generate run; wait for it to settle before deleting$/;
const GENERATE_WORK_ACTIVE_CODE = 'generate_work_active';
/** The API clients' placeholder for an error response without a body. */
const BODYLESS_ERROR_RE = /^request failed \(\d+\)$/;

/**
 * Maps a delete failure to the Spanish message the row should surface. An
 * offline (service_unavailable) error gets the "start your orchestrator" hint.
 * A 409 from DeleteJob or DeleteStreamJob is English (`job is recording; …`,
 * `stream job is rendering; …`); those shapes become the Spanish wait copy.
 * A 409 that is already Spanish (or any other explanation) passes through.
 * Anything without a message falls back to a generic retry line.
 * Pure and unit-tested so the button component never branches on error shapes.
 */
export function deleteErrorMessage(err: unknown): string {
  const e = err as { code?: unknown; message?: unknown } | null;
  if (e?.code === SERVICE_UNAVAILABLE_CODE) return DELETE_OFFLINE_MESSAGE;
  if (e?.code === GENERATE_WORK_ACTIVE_CODE) return DELETE_RENDER_BUSY_MESSAGE;
  if (typeof e?.message !== 'string' || e.message.trim() === '') return DELETE_GENERIC_MESSAGE;
  if (STREAM_BUSY_RE.test(e.message)) return DELETE_STREAM_BUSY_MESSAGE;
  if (JOB_BUSY_RE.test(e.message)) return DELETE_JOB_BUSY_MESSAGE;
  if (JOB_RENDER_BUSY_RE.test(e.message)) return DELETE_RENDER_BUSY_MESSAGE;
  if (BODYLESS_ERROR_RE.test(e.message)) return DELETE_GENERIC_MESSAGE;
  return e.message;
}
