import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, serviceUnavailable, forwardError } from '../../demos/_lib';
import { INVALID_SHARE_CODE } from '@/lib/api/share-code-resolve';

export const runtime = 'nodejs';

/** Decode/resolve a share code. matchId/outcomeId stay strings (they exceed 2^53). */
export async function POST(request: Request): Promise<Response> {
  const raw = (await request.json().catch(() => null)) as { code?: unknown } | null;
  const code = typeof raw?.code === 'string' ? raw.code : '';

  const res = await callOrchestrator(`${orchestratorUrl()}/api/steam/sharecode`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ code }),
  });
  if (res === null) return serviceUnavailable();
  if (res.status === 400) {
    const body = (await res.json().catch(() => null)) as { code?: unknown; message?: unknown } | null;
    return NextResponse.json(
      {
        code: typeof body?.code === 'string' ? body.code : INVALID_SHARE_CODE,
        message: typeof body?.message === 'string' ? body.message : 'invalid share code',
      },
      { status: 400 },
    );
  }
  if (!res.ok) return forwardError(res);

  const data = (await res.json()) as {
    status?: unknown;
    matchId?: unknown;
    outcomeId?: unknown;
    tokenId?: unknown;
    demoUrl?: unknown;
  };
  const demoUrl = typeof data.demoUrl === 'string' && data.demoUrl !== '' ? data.demoUrl : undefined;
  return NextResponse.json(
    {
      status: data.status === 'resolved' ? 'resolved' : 'decoded',
      matchId: typeof data.matchId === 'string' ? data.matchId : '',
      outcomeId: typeof data.outcomeId === 'string' ? data.outcomeId : '',
      tokenId: typeof data.tokenId === 'number' && Number.isFinite(data.tokenId) ? data.tokenId : 0,
      ...(demoUrl === undefined ? {} : { demoUrl }),
    },
    { headers: { 'cache-control': 'no-store' } },
  );
}
