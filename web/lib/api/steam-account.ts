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

/* -------------------------------------------------------------------------- */
/* Client-side validation                                                      */
/* -------------------------------------------------------------------------- */

/**
 * Every rule below mirrors what the orchestrator enforces on PUT
 * /api/steam/account, so the form catches a bad value before the request and
 * never rejects one the server would have accepted:
 *
 * - SteamID64: `steamresolve.ParseSteamID` — 17 digits starting with 7656,
 *   bare or inside a `/profiles/<id>` URL.
 * - Authentication code: `validateAuthCode` — 5 to 64 characters.
 * - Web API key: `validateAPIKey` — 8 to 64 characters.
 * - Known share code: `sharecode.Decode` — optional `CSGO-` prefix, dashes
 *   ignored, 25 characters of the base-57 alphabet, at most 144 bits.
 *
 * An empty secret keeps the stored one (the server merges), so only a missing
 * SteamID64 with nothing saved is an error by omission.
 */
export type SteamAccountField = 'steamId' | 'authCode' | 'apiKey' | 'knownCode';
export type SteamAccountFieldErrors = Partial<Record<SteamAccountField, string>>;

export const STEAM_FIELD_ERROR = {
  steamIdMissing: 'Escribe tu SteamID64 o pega la URL de tu perfil de Steam.',
  steamIdInvalid: 'No es un SteamID64 válido. Son 17 cifras que empiezan por 7656, o una URL steamcommunity.com/profiles/….',
  authCode: 'El código de autenticación debe tener entre 5 y 64 caracteres, con el formato AAAA-AAAAA-AAAA.',
  apiKey: 'La clave de la Web API debe tener entre 8 y 64 caracteres. Cópiala entera desde steamcommunity.com/dev/apikey.',
  knownCode: 'El código de partida tiene la forma CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx.',
} as const;

const PROFILES_SEGMENT = '/profiles/';
const STEAM_ID64_RE = /^\d{17}$/;
const SHARE_CODE_PREFIX = 'CSGO-';
const SHARE_CODE_LENGTH = 25;
const SHARE_CODE_DICTIONARY = 'ABCDEFGHJKLMNOPQRSTUVWXYZabcdefhijkmnopqrstuvwxyz23456789';
const SHARE_CODE_MAX = 1n << 144n;

/** Byte length, as Go's `len` measures the trimmed string. */
function byteLength(value: string): number {
  return new TextEncoder().encode(value).length;
}

/** The SteamID64 the server would store for `raw`, or null when it would reject it. */
export function parseSteamId64(raw: string): string | null {
  let id = raw.trim();
  if (id === '') return null;
  const marker = id.indexOf(PROFILES_SEGMENT);
  if (marker >= 0) {
    id = id.slice(marker + PROFILES_SEGMENT.length);
    const end = id.search(/[/?#]/);
    if (end >= 0) id = id.slice(0, end);
  }
  // ParseUint(…, 64) also refuses 17-digit values above 2^64-1, which none
  // starting with 7656 can reach, so the digit check is the whole rule.
  if (!STEAM_ID64_RE.test(id) || !id.startsWith('7656')) return null;
  return id;
}

/** Whether `raw` decodes as a CS2 match share code. */
export function isValidShareCode(raw: string): boolean {
  const code = raw.trim();
  const body = code.startsWith(SHARE_CODE_PREFIX) ? code.slice(SHARE_CODE_PREFIX.length) : code;
  const stripped = body.replaceAll('-', '');
  if (stripped.length !== SHARE_CODE_LENGTH) return false;
  const base = BigInt(SHARE_CODE_DICTIONARY.length);
  let value = 0n;
  for (let i = stripped.length - 1; i >= 0; i -= 1) {
    const digit = SHARE_CODE_DICTIONARY.indexOf(stripped.charAt(i));
    if (digit < 0) return false;
    value = value * base + BigInt(digit);
  }
  return value < SHARE_CODE_MAX;
}

/** Per-field errors for a save; an empty object means the request can go out. */
export function validateSteamAccountInput(
  input: SteamAccountInput,
  stored: Pick<SteamAccount, 'steamId'> | null,
): SteamAccountFieldErrors {
  const errors: SteamAccountFieldErrors = {};
  const steamId = input.steamId.trim();
  if (steamId === '') {
    if (!stored?.steamId) errors.steamId = STEAM_FIELD_ERROR.steamIdMissing;
  } else if (parseSteamId64(steamId) === null) {
    errors.steamId = STEAM_FIELD_ERROR.steamIdInvalid;
  }
  const authCode = (input.authCode ?? '').trim();
  if (authCode !== '' && (byteLength(authCode) < 5 || byteLength(authCode) > 64)) {
    errors.authCode = STEAM_FIELD_ERROR.authCode;
  }
  const apiKey = (input.apiKey ?? '').trim();
  if (apiKey !== '' && (byteLength(apiKey) < 8 || byteLength(apiKey) > 64)) {
    errors.apiKey = STEAM_FIELD_ERROR.apiKey;
  }
  const knownCode = (input.knownCode ?? '').trim();
  if (knownCode !== '' && !isValidShareCode(knownCode)) {
    errors.knownCode = STEAM_FIELD_ERROR.knownCode;
  }
  return errors;
}

/* -------------------------------------------------------------------------- */
/* Server error copy                                                           */
/* -------------------------------------------------------------------------- */

export type SteamAccountAction = 'load' | 'save' | 'sync' | 'clear';

const STEAM_ACTION_FALLBACK: Record<SteamAccountAction, string> = {
  load: 'No se pudo leer la cuenta de Steam guardada. Vuelve a intentarlo.',
  save: 'No se pudo guardar la cuenta de Steam. Revisa los datos y vuelve a intentarlo.',
  sync: 'No se pudieron sincronizar las partidas. Vuelve a intentarlo.',
  clear: 'No se pudo desconectar la cuenta de Steam. Vuelve a intentarlo.',
};

/** Code for a request that never reached the local service. */
export const STEAM_NETWORK_ERROR_CODE = 'network_error';

/**
 * Spanish copy for a failed account request, attached to the field it is about
 * when the server names one. The orchestrator's error text is English and
 * written for logs, so it is only ever pattern-matched, never shown.
 */
export function steamAccountFailure(
  failure: { message: string; code?: string },
  action: SteamAccountAction,
): { field?: SteamAccountField; message: string } {
  const { message, code } = failure;
  if (code === STEAM_NETWORK_ERROR_CODE) {
    return { message: 'No se pudo contactar con el servicio local. Comprueba que ClipHub está abierto y vuelve a intentarlo.' };
  }
  if (code === 'steam_account_invalid') {
    if (/steam id/i.test(message)) return { field: 'steamId', message: STEAM_FIELD_ERROR.steamIdInvalid };
    if (/authentication code/i.test(message)) return { field: 'authCode', message: STEAM_FIELD_ERROR.authCode };
    if (/web api key/i.test(message)) return { field: 'apiKey', message: STEAM_FIELD_ERROR.apiKey };
    if (/share code/i.test(message)) return { field: 'knownCode', message: STEAM_FIELD_ERROR.knownCode };
  }
  if (code === 'history_not_configured') {
    return action === 'sync'
      ? { message: 'Guarda primero tu SteamID64, el código de autenticación y la clave de la Web API.' }
      : { message: 'El historial de Steam no está disponible en este PC. Reinicia ClipHub y vuelve a intentarlo.' };
  }
  if (code === 'need_known_code') {
    return { field: 'knownCode', message: 'Pega primero un código de partida conocido para arrancar el historial.' };
  }
  if (code === 'steam_history_unavailable') {
    return { message: 'Steam no respondió al pedir tu historial. Vuelve a intentarlo en unos minutos.' };
  }
  return { message: STEAM_ACTION_FALLBACK[action] };
}

export async function loadSteamAccount(): Promise<SteamAccountResult> {
  let res: Response;
  try {
    res = await fetch(STEAM_ACCOUNT_ENDPOINT, { cache: 'no-store' });
  } catch (err) {
    return { kind: 'failed', message: `no se pudo contactar con el servicio local: ${String(err)}`, code: STEAM_NETWORK_ERROR_CODE };
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
    return { kind: 'failed', message: `no se pudo contactar con el servicio local: ${String(err)}`, code: STEAM_NETWORK_ERROR_CODE };
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
    return { kind: 'failed', message: `no se pudo contactar con el servicio local: ${String(err)}`, code: STEAM_NETWORK_ERROR_CODE };
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
    return { kind: 'failed', message: `no se pudo contactar con el servicio local: ${String(err)}`, code: STEAM_NETWORK_ERROR_CODE };
  }
  const body = await read(res);
  if (res.ok) return { kind: 'ok', account: parseAccount(body) };
  return failFrom(res, body);
}

;
