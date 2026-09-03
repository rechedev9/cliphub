import { NextResponse } from 'next/server';
import { localAPIRequestError } from '@/lib/api/local-request-guard';
import { streamJobUrl, callOrchestrator, forwardError, serviceUnavailable } from '../_lib';
import { publicStreamJob } from '@/lib/api/public-projections';

export const runtime = 'nodejs';

/** GET /api/streams/{jobId} — proxy a single stream-clip job. */
export async function GET(_request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = streamJobUrl(jobId);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const res = await callOrchestrator(url);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  return NextResponse.json(publicStreamJob(await res.json()));
}

/** DELETE /api/streams/{jobId} — 204, or 409 while the job is still acquiring or rendering. */
export async function DELETE(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const localError = await localAPIRequestError(request.headers, request.method);
  if (localError !== undefined) return NextResponse.json({ error: localError }, { status: 403 });
  const { jobId } = await params;
  const url = streamJobUrl(jobId);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const res = await callOrchestrator(url, { method: 'DELETE' });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return new Response(null, { status: 204 });
}
