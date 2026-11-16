import { NextResponse } from 'next/server';
import { readBoundedText } from '@/lib/api/bounded-request-body';
import { orchestratorUrl, callOrchestrator, forwardError, serviceUnavailable, UUID_RE } from '../../../demos/_lib';

export const runtime = 'nodejs';
type Context = { params: Promise<{ themeId: string }> };

async function themeUrl(context: Context): Promise<string | null> {
  const { themeId } = await context.params;
  return UUID_RE.test(themeId) ? `${orchestratorUrl()}/api/full-demo/hud-themes/${themeId.toLowerCase()}` : null;
}

export async function PUT(request: Request, context: Context): Promise<Response> {
  const url = await themeUrl(context);
  if (!url) return NextResponse.json({ error: 'Tema de HUD inválido.' }, { status: 400 });
  const incoming = await readBoundedText(request);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  const response = await callOrchestrator(url, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: incoming.text });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const body: unknown = await response.json();
  return NextResponse.json(body);
}

export async function DELETE(_request: Request, context: Context): Promise<Response> {
  const url = await themeUrl(context);
  if (!url) return NextResponse.json({ error: 'Tema de HUD inválido.' }, { status: 400 });
  const response = await callOrchestrator(url, { method: 'DELETE' });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  return new Response(null, { status: 204 });
}
