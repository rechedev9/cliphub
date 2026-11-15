import { NextResponse } from 'next/server';
import { jobUrl, forwardError, callOrchestrator, serviceUnavailable } from '../../_lib';

export const runtime = 'nodejs';

/** GET /api/demos/{jobId}/moments: proxy the job's scored plays unchanged. */
export async function GET(_request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = jobUrl(jobId, '/moments');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const res = await callOrchestrator(url);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  const body: unknown = await res.json();
  return NextResponse.json(body);
}
