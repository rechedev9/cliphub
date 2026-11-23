import { SERVICE_UNAVAILABLE_CODE } from './api/types.ts';
import { currentEdition, type Edition } from './edition-routes.ts';
import { loadErrorMessage, RETRY_HINT } from './load-error.ts';

export type HumanErrorOptions = {
  error: unknown;
  lead: string;
  codes?: Readonly<Record<string, string>>;
  edition?: Edition;
};

function field(error: unknown, key: string): unknown {
  return typeof error === 'object' && error !== null ? Reflect.get(error, key) : undefined;
}

export function serverError(message: string, response: { readonly status: number }, code?: unknown): Error {
  return Object.assign(new Error(message), { status: response.status }, typeof code === 'string' ? { code } : {});
}

export function humanErrorText({ error, lead, codes = {}, edition = currentEdition() }: HumanErrorOptions): string {
  const code = field(error, 'code');
  const known = typeof code === 'string' ? codes[code] : undefined;
  if (known !== undefined) return known;
  const fromServer = typeof field(error, 'status') === 'number' || code === SERVICE_UNAVAILABLE_CODE;
  if (fromServer || error instanceof TypeError) return loadErrorMessage({ error, lead, edition });
  if (error instanceof Error && error.message.trim() !== '') return error.message;
  return `${lead} ${RETRY_HINT}`;
}
