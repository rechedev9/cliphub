import { NextResponse } from 'next/server';
import { jobUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../../_lib';

export const runtime = 'nodejs';

const TEMPLATE_RE = /^[a-z][a-z0-9-]{0,31}$/;

/** GET /api/demos/{jobId}/player-card/preview?template=: the job's card in one template, as an SVG. */
export async function GET(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const template = new URL(request.url).searchParams.get('template') ?? '';
  if (!TEMPLATE_RE.test(template)) return NextResponse.json({ error: 'Plantilla de carta inválida.' }, { status: 400 });
  const url = jobUrl(jobId, `/player-card/preview?template=${template}`);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  const response = await callOrchestrator(url, { signal: request.signal });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const headers = new Headers({ 'Content-Type': 'image/svg+xml', 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff' });
  const policy = response.headers.get('content-security-policy');
  if (policy !== null) headers.set('Content-Security-Policy', policy);
  return new Response(response.body, { status: response.status, headers });
}
