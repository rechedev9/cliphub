import { NextResponse } from 'next/server';
import { readBoundedText } from '@/lib/api/bounded-request-body';
import { orchestratorUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../demos/_lib';

export const runtime = 'nodejs';

export async function GET(request: Request): Promise<Response> {
  const response = await callOrchestrator(`${orchestratorUrl()}/api/full-demo/hud-themes`, { cache: 'no-store', signal: request.signal });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const body: unknown = await response.json();
  return NextResponse.json(body);
}

export async function POST(request: Request): Promise<Response> {
  const incoming = await readBoundedText(request);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  const response = await callOrchestrator(`${orchestratorUrl()}/api/full-demo/hud-themes`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: incoming.text });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const body: unknown = await response.json();
  return NextResponse.json(body, { status: response.status });
}
