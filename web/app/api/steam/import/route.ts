import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, serviceUnavailable, forwardError } from '../../demos/_lib';

export const runtime = 'nodejs';

export async function POST(request: Request): Promise<Response> {
  const raw = (await request.json().catch(() => null)) as {
    code?: unknown;
    username?: unknown;
    password?: unknown;
    guard?: unknown;
  } | null;
  const payload: Record<string, string> = {
    code: typeof raw?.code === 'string' ? raw.code : '',
  };
  if (typeof raw?.username === 'string' && raw.username !== '') payload.username = raw.username;
  if (typeof raw?.password === 'string' && raw.password !== '') payload.password = raw.password;
  if (typeof raw?.guard === 'string' && raw.guard !== '') payload.guard = raw.guard;

  const res = await callOrchestrator(`${orchestratorUrl()}/api/steam/import`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const data = (await res.json()) as { id?: unknown; status?: unknown; matchId?: unknown };
  return NextResponse.json(
    {
      id: typeof data.id === 'string' ? data.id : '',
      status: typeof data.status === 'string' ? data.status : '',
      matchId: typeof data.matchId === 'string' ? data.matchId : '',
    },
    { status: 201, headers: { 'cache-control': 'no-store' } },
  );
}
