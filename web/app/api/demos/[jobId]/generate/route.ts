import { NextResponse } from 'next/server';
import { MAX_RENDER_CONTROL_BODY_BYTES, readBoundedText } from '@/lib/api/bounded-request-body';
import { jobUrl, mutationHeaders, forwardError, callOrchestrator, serviceUnavailable } from '../../_lib';

export const runtime = 'nodejs';

/** POST /api/demos/{jobId}/generate — reuse or capture from the cached demo, then render. */
export async function POST(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = jobUrl(jobId, '/generate');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const incoming = await readBoundedText(request, MAX_RENDER_CONTROL_BODY_BYTES);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  const init: RequestInit = { method: 'POST', headers: { ...mutationHeaders() } };
  if (incoming.text) {
    init.headers = { ...init.headers, 'Content-Type': 'application/json' };
    init.body = incoming.text;
  }
  const res = await callOrchestrator(url, init);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  return NextResponse.json((await res.json()) as unknown);
}
