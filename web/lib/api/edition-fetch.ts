import { currentEdition, isPathAvailable, type Edition } from '../edition-routes.ts';

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

export function assertEditionServes(url: string): void {
  const path = apiPathOf(url);
  const edition = currentEdition();
  if (path !== null && !isPathAvailable(path, edition)) throw new EditionApiError(path, edition);
}

const READ_METHODS: ReadonlySet<string> = new Set(['GET', 'HEAD', 'OPTIONS']);
const mutationListeners = new Set<() => void>();

export function subscribeToApiMutations(listener: () => void): () => void {
  mutationListeners.add(listener);
  return () => {
    mutationListeners.delete(listener);
  };
}

export function notifyApiMutation(request: { method: string | undefined; ok: boolean }): void {
  if (!request.ok || READ_METHODS.has((request.method ?? 'GET').toUpperCase())) return;
  for (const listener of mutationListeners) listener();
}

export async function apiFetch(url: string, init?: RequestInit): Promise<Response> {
  assertEditionServes(url);
  const response = await fetch(url, init);
  notifyApiMutation({ method: init?.method, ok: response.ok });
  return response;
}
