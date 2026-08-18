import { SERVICE_UNAVAILABLE_CODE, STEAM_CODES } from './types.ts';

export const STEAM_IMPORT_ENDPOINT = '/api/steam/import';

export type SteamImportCredentials = {
  username: string;
  password: string;
  guard: string;
};

export type SteamImportResult =
  | { kind: 'queued'; id: string; status: string; matchId: string }
  | { kind: 'needCredentials' }
  | { kind: 'unavailable'; message: string }
  | { kind: 'offline' }
  | { kind: 'failed'; message: string; code?: string };

type ImportResponse = {
  id?: unknown;
  status?: unknown;
  matchId?: unknown;
  code?: unknown;
  error?: unknown;
  message?: unknown;
};

export async function importShareCode(
  code: string,
  credentials?: SteamImportCredentials,
): Promise<SteamImportResult> {
  const payload: Record<string, string> = { code };
  if (credentials) {
    payload.username = credentials.username;
    payload.password = credentials.password;
    payload.guard = credentials.guard;
  }
  let res: Response;
  try {
    res = await fetch(STEAM_IMPORT_ENDPOINT, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(payload),
      cache: 'no-store',
    });
  } catch (err) {
    return { kind: 'failed', message: `no se pudo contactar con el servicio local: ${String(err)}` };
  }
  const body = (await res.json().catch(() => ({}))) as ImportResponse;
  if (res.ok || res.status === 201) {
    return {
      kind: 'queued',
      id: typeof body.id === 'string' ? body.id : '',
      status: typeof body.status === 'string' ? body.status : '',
      matchId: typeof body.matchId === 'string' ? body.matchId : '',
    };
  }
  if (res.status === 503 && body.code === SERVICE_UNAVAILABLE_CODE) {
    return { kind: 'offline' };
  }
  const codeName = typeof body.code === 'string' ? body.code : undefined;
  if (res.status === 409 && codeName === STEAM_CODES.credentialsRequired) {
    return { kind: 'needCredentials' };
  }
  let message = `error del servicio (HTTP ${res.status})`;
  if (typeof body.error === 'string' && body.error !== '') message = body.error;
  else if (typeof body.message === 'string' && body.message !== '') message = body.message;
  if (res.status === 409 && codeName === STEAM_CODES.demoUnavailable) {
    return { kind: 'unavailable', message };
  }
  return { kind: 'failed', message, code: codeName };
}
