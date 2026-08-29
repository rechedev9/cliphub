import { SERVICE_UNAVAILABLE_CODE } from './types.ts';

export const STEAM_ACCOUNT_ENDPOINT = '/api/steam/account';
const STEAM_SYNC_ENDPOINT = '/api/steam/matches/sync';

export type SteamStoredMatch = {
  shareCode: string;
  matchId: string;
  discoveredAt?: string;
};

export type SteamAccount = {
  steamId: string;
  authCodeSet: boolean;
  apiKeySet: boolean;
  knownCode: string;
  historyConfigured: boolean;
  gcConfigured: boolean;
  matches: SteamStoredMatch[];
};

export type SteamAccountResult =
  | { kind: 'ok'; account: SteamAccount }
  | { kind: 'offline' }
  | { kind: 'failed'; message: string; code?: string };

export type SteamAccountInput = {
  steamId: string;
  authCode?: string;
  apiKey?: string;
  knownCode?: string;
};

type AccountResponse = {
  steamId?: unknown;
  authCodeSet?: unknown;
  apiKeySet?: unknown;
  knownCode?: unknown;
  historyConfigured?: unknown;
  gcConfigured?: unknown;
  matches?: unknown;
  code?: unknown;
  error?: unknown;
  message?: unknown;
};

function parseAccount(body: AccountResponse): SteamAccount {
  const matches: SteamStoredMatch[] = [];
  if (Array.isArray(body.matches)) {
    for (const item of body.matches) {
      if (typeof item !== 'object' || item === null) continue;
      const row = item as { shareCode?: unknown; matchId?: unknown; discoveredAt?: unknown };
      if (typeof row.shareCode !== 'string' || typeof row.matchId !== 'string') continue;
      matches.push({
        shareCode: row.shareCode,
        matchId: row.matchId,
        discoveredAt: typeof row.discoveredAt === 'string' ? row.discoveredAt : undefined,
      });
    }
  }
  return {
    steamId: typeof body.steamId === 'string' ? body.steamId : '',
    authCodeSet: body.authCodeSet === true,
    apiKeySet: body.apiKeySet === true,
    knownCode: typeof body.knownCode === 'string' ? body.knownCode : '',
    historyConfigured: body.historyConfigured === true,
    gcConfigured: body.gcConfigured === true,
    matches,
  };
}

function failFrom(res: Response, body: AccountResponse): SteamAccountResult {
  if (res.status === 503 && body.code === SERVICE_UNAVAILABLE_CODE) {
    return { kind: 'offline' };
  }
  let message = `error del servicio (HTTP ${res.status})`;
  if (typeof body.error === 'string' && body.error !== '') message = body.error;
  else if (typeof body.message === 'string' && body.message !== '') message = body.message;
  const code = typeof body.code === 'string' ? body.code : undefined;
  return { kind: 'failed', message, code };
}

async function read(res: Response): Promise<AccountResponse> {
  return (await res.json().catch(() => ({}))) as AccountResponse;
}

export async function loadSteamAccount(): Promise<SteamAccountResult> {
  let res: Response;
  try {
    res = await fetch(STEAM_ACCOUNT_ENDPOINT, { cache: 'no-store' });
  } catch (err) {
    return { kind: 'failed', message: `no se pudo contactar con el servicio local: ${String(err)}` };
  }
  const body = await read(res);
  if (res.ok) return { kind: 'ok', account: parseAccount(body) };
  return failFrom(res, body);
}

export async function saveSteamAccount(input: SteamAccountInput): Promise<SteamAccountResult> {
  let res: Response;
  try {
    res = await fetch(STEAM_ACCOUNT_ENDPOINT, {
      method: 'PUT',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(input),
      cache: 'no-store',
    });
  } catch (err) {
    return { kind: 'failed', message: `no se pudo contactar con el servicio local: ${String(err)}` };
  }
  const body = await read(res);
  if (res.ok) return { kind: 'ok', account: parseAccount(body) };
  return failFrom(res, body);
}

export async function clearSteamAccount(): Promise<SteamAccountResult> {
  let res: Response;
  try {
    res = await fetch(STEAM_ACCOUNT_ENDPOINT, { method: 'DELETE', cache: 'no-store' });
  } catch (err) {
    return { kind: 'failed', message: `no se pudo contactar con el servicio local: ${String(err)}` };
  }
  const body = await read(res);
  if (res.ok) return { kind: 'ok', account: parseAccount(body) };
  return failFrom(res, body);
}

export async function syncSteamMatches(): Promise<SteamAccountResult> {
  let res: Response;
  try {
    res = await fetch(STEAM_SYNC_ENDPOINT, { method: 'POST', cache: 'no-store' });
  } catch (err) {
    return { kind: 'failed', message: `no se pudo contactar con el servicio local: ${String(err)}` };
  }
  const body = await read(res);
  if (res.ok) return { kind: 'ok', account: parseAccount(body) };
  return failFrom(res, body);
}

;
