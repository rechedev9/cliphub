import { currentEdition, isPathAvailable, type Edition } from '../edition-routes.ts';

/** A same-origin API call that the running edition does not serve (web/lib/edition-routes.ts). */
export class EditionApiError extends Error {
  readonly path: string;
  readonly edition: Edition;

  constructor(path: string, edition: Edition) {
    super(`${path} is not served in the ${edition} edition: gate the caller on currentEdition() from web/lib/edition-routes.ts`);
    this.name = 'EditionApiError';
    this.path = path;
    this.edition = edition;
  }
}

function apiPathOf(input: string): string | null {
  if (!input.startsWith('/') || input.startsWith('//')) return null;
  const pathname = new URL(input, 'http://edition.invalid').pathname;
  return pathname === '/api' || pathname.startsWith('/api/') ? pathname : null;
}

/** Throws EditionApiError when url is a same-origin /api path the running edition does not serve. */
export function assertEditionServes(url: string): void {
  const path = apiPathOf(url);
  const edition = currentEdition();
  if (path !== null && !isPathAvailable(path, edition)) throw new EditionApiError(path, edition);
}

/** The one way UI code calls an API: fetch, refused before the network for a route the edition lacks. */
export async function apiFetch(url: string, init?: RequestInit): Promise<Response> {
  assertEditionServes(url);
  return fetch(url, init);
}
