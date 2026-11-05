import { NextResponse } from 'next/server';
import { streamJobUrl, mutationHeaders, forwardError, callOrchestrator, serviceUnavailable } from '../../_lib';

export const runtime = 'nodejs';

/** GET /api/streams/{jobId}/plays — the state and result of the play scan. */
export async function GET(_request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = streamJobUrl(jobId, '/plays');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const res = await callOrchestrator(url);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json((await res.json()) as unknown);
}

/** POST /api/streams/{jobId}/plays — start scanning the source for plays. */
export async function POST(_request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = streamJobUrl(jobId, '/plays');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const res = await callOrchestrator(url, { method: 'POST', headers: mutationHeaders() });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json((await res.json()) as unknown, { status: res.status });
}
