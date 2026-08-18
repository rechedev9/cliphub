import { SERVICE_UNAVAILABLE_CODE } from './types.ts';

/** Proxy/orchestrator code for a share code that fails to decode. */
export const INVALID_SHARE_CODE = 'invalid_share_code';

export const SHARE_CODE_ENDPOINT = '/api/steam/sharecode';

/** 64-bit ids stay strings: they exceed JS Number precision. */
export type ShareCodeResolution =
  | { kind: 'decoded' | 'resolved'; matchId: string; outcomeId: string; tokenId: number; demoUrl?: string }
  | { kind: 'invalid'; message: string }
  | { kind: 'offline' }
  | { kind: 'failed'; message: string };

type ShareCodeResponse = {
  status?: unknown;
  matchId?: unknown;
  outcomeId?: unknown;
  tokenId?: unknown;
  demoUrl?: unknown;
  code?: unknown;
  message?: unknown;
  error?: unknown;
};

export async function resolveShareCode(code: string): Promise<ShareCodeResolution> {
  let res: Response;
  try {
    res = await fetch(SHARE_CODE_ENDPOINT, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ code }),
      cache: 'no-store',
    });
  } catch (err) {
    return { kind: 'failed', message: `no se pudo contactar con el servicio local: ${String(err)}` };
  }

  const body = (await res.json().catch(() => ({}))) as ShareCodeResponse;
  if (res.ok) {
    return {
      kind: body.status === 'resolved' ? 'resolved' : 'decoded',
      matchId: typeof body.matchId === 'string' ? body.matchId : '',
      outcomeId: typeof body.outcomeId === 'string' ? body.outcomeId : '',
      tokenId: typeof body.tokenId === 'number' && Number.isFinite(body.tokenId) ? body.tokenId : 0,
      demoUrl: typeof body.demoUrl === 'string' && body.demoUrl !== '' ? body.demoUrl : undefined,
    };
  }
  if (res.status === 503 && body.code === SERVICE_UNAVAILABLE_CODE) {
    return { kind: 'offline' };
  }
  let message = `error del servicio (HTTP ${res.status})`;
  if (typeof body.message === 'string' && body.message !== '') {
    message = body.message;
  } else if (typeof body.error === 'string' && body.error !== '') {
    message = body.error;
  }
  if (res.status === 400) {
    return { kind: 'invalid', message };
  }
  return { kind: 'failed', message };
}
