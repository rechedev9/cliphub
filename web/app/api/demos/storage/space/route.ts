import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../_lib';

export const runtime = 'nodejs';

const SECONDS_RE = /^[0-9]{1,6}$/;

/** GET /api/demos/storage/space?capture_seconds=&render_seconds=: free disk against an estimate, for a warning. */
export async function GET(request: Request): Promise<Response> {
  const query = new URL(request.url).searchParams;
  const params = new URLSearchParams();
  for (const name of ['capture_seconds', 'render_seconds']) {
    const value = query.get(name) ?? '0';
    if (!SECONDS_RE.test(value)) return NextResponse.json({ error: `invalid ${name}` }, { status: 400 });
    params.set(name, value);
  }
  const response = await callOrchestrator(`${orchestratorUrl()}/api/storage/space?${params.toString()}`, { cache: 'no-store' });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const body: unknown = await response.json();
  return NextResponse.json(body, { headers: { 'Cache-Control': 'no-store' } });
}
