import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../../demos/_lib';

export const runtime = 'nodejs';

const PREVIEW_KEYS = ['base', 'ct', 't', 'background', 'radius', 'compact_header', 'hide_loadout', 'round_win_banner', 'win_probability', 'ct_name', 't_name', 'vertical'];

export async function GET(request: Request): Promise<Response> {
  const incoming = new URL(request.url).searchParams;
  const query = new URLSearchParams();
  for (const key of PREVIEW_KEYS) {
    const value = incoming.get(key);
    if (value !== null) query.set(key, value.slice(0, 64));
  }
  if (!query.get('base')) return NextResponse.json({ error: 'Falta el diseño de HUD.' }, { status: 400 });
  const response = await callOrchestrator(`${orchestratorUrl()}/api/full-demo/hud-themes/preview?${query.toString()}`, { signal: request.signal });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const headers = new Headers({ 'Content-Type': 'image/svg+xml', 'Cache-Control': 'private, max-age=3600', 'X-Content-Type-Options': 'nosniff' });
  const policy = response.headers.get('content-security-policy');
  if (policy !== null) headers.set('Content-Security-Policy', policy);
  return new Response(response.body, { status: response.status, headers });
}
