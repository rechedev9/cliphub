import { NextResponse } from 'next/server';
import { localAPIRequestError } from '@/lib/api/local-request-guard';
import { callOrchestrator, forwardError, orchestratorUrl, serviceUnavailable } from '../../demos/_lib';

export const runtime = 'nodejs';

type Params = { params: Promise<{ path: string[] }> };

const STATUS_KEYS = ['connected', 'server_url', 'login', 'workspace_id', 'workspace_name', 'last_sync_at', 'last_error', 'pending_changes', 'syncing'] as const;
const CONNECT_FIELDS = ['server_url', 'login', 'password', 'workspace_id'] as const;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function pick(value: unknown): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (!isRecord(value)) return out;
  for (const key of STATUS_KEYS) if (key in value) out[key] = value[key];
  return out;
}

/** Studio's link to a ClipMind account: the status, sign in, sync now and sign out, nothing else. */
async function handle(request: Request, { params }: Params): Promise<Response> {
  const localError = await localAPIRequestError(request.headers, request.method);
  if (localError !== undefined) return NextResponse.json({ error: localError }, { status: 403 });

  const route = (await params).path.join('/');
  const allowed =
    (route === 'account' && ['GET', 'POST', 'DELETE'].includes(request.method)) || (route === 'sync' && request.method === 'POST');
  if (!allowed) return NextResponse.json({ error: 'not found' }, { status: 404 });

  const init: RequestInit = { method: request.method };
  if (route === 'account' && request.method === 'POST') {
    const body: unknown = await request.json().catch(() => undefined);
    const forwarded: Record<string, string> = {};
    for (const field of CONNECT_FIELDS) {
      if (isRecord(body) && typeof body[field] === 'string') forwarded[field] = body[field];
    }
    init.headers = { 'content-type': 'application/json' };
    init.body = JSON.stringify(forwarded);
  }
  const upstream = await callOrchestrator(`${orchestratorUrl()}/api/clipmind/${route}`, init);
  if (upstream === null) return serviceUnavailable();
  if (!upstream.ok) return forwardError(upstream);
  return NextResponse.json(pick(await upstream.json()), { headers: { 'cache-control': 'no-store' } });
}

export const GET = handle;
export const POST = handle;
export const DELETE = handle;
