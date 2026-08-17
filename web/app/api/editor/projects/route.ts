import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, mutationHeaders, forwardError, serviceUnavailable } from '../_lib';

export const runtime = 'nodejs';

export async function GET(): Promise<Response> {
  const res = await callOrchestrator(`${orchestratorUrl()}/api/editor/projects`);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json((await res.json()) as unknown);
}

export async function POST(request: Request): Promise<Response> {
  const body = await request.text();
  const res = await callOrchestrator(`${orchestratorUrl()}/api/editor/projects`, {
    method: 'POST',
    headers: { ...mutationHeaders(), 'Content-Type': 'application/json' },
    body: body === '' ? '{}' : body,
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json((await res.json()) as unknown, { status: res.status });
}
