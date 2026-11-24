import { SERVICE_UNAVAILABLE_CODE } from './api/types.ts';
import { currentEdition, type Edition } from './edition-routes.ts';

export type LoadErrorKind = 'offline' | 'not_found' | 'error';

const OFFLINE_STATUSES = new Set([502, 503, 504]);

function field(error: unknown, key: string): unknown {
  return typeof error === 'object' && error !== null ? Reflect.get(error, key) : undefined;
}

export function loadErrorKind(error: unknown): LoadErrorKind {
  if (error instanceof TypeError) return 'offline';
  if (field(error, 'code') === SERVICE_UNAVAILABLE_CODE) return 'offline';
  const status = field(error, 'status');
  if (typeof status !== 'number') return 'error';
  if (status === 404) return 'not_found';
  return OFFLINE_STATUSES.has(status) ? 'offline' : 'error';
}

export function offlineTitle(edition: Edition = currentEdition()): string {
  return edition === 'web' ? 'No se puede conectar con ClipMind' : 'El servicio local se ha detenido';
}

export function offlineHint(edition: Edition = currentEdition()): string {
  return edition === 'web' ? 'Comprueba tu conexión y vuelve a intentarlo en un momento.' : 'Cierra ClipHub Studio y vuelve a abrirlo.';
}

export function offlineMessage(edition: Edition = currentEdition()): string {
  return edition === 'web'
    ? 'No se puede conectar con ClipMind. Vuelve a intentarlo en un momento.'
    : 'El servicio local se ha detenido: cierra ClipHub Studio y vuelve a abrirlo.';
}

export const NOT_FOUND_HINT = 'Ya no está en la biblioteca. Puede que se haya borrado.';
export const RETRY_HINT = 'Vuelve a intentarlo.';

export type LoadErrorMessageOptions = {
  error: unknown;
  lead: string;
  edition?: Edition;
};

export function loadErrorMessage({ error, lead, edition = currentEdition() }: LoadErrorMessageOptions): string {
  const kind = loadErrorKind(error);
  if (kind === 'offline') return `${lead} ${offlineMessage(edition)}`;
  if (kind === 'not_found') return `${lead} ${NOT_FOUND_HINT}`;
  return `${lead} ${RETRY_HINT}`;
}
