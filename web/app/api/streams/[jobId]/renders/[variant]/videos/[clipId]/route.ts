import { NextResponse } from 'next/server';
import { localAPIRequestError } from '@/lib/api/local-request-guard';
import { callOrchestrator, forwardError, serviceUnavailable, streamJobUrl, proxyStream } from '../../../../../_lib';

export const runtime = 'nodejs';

const VARIANT_RE = /^[A-Za-z0-9][A-Za-z0-9_-]*$/;
const CLIP_ID_RE = /^[A-Za-z0-9._-]+$/;

/** GET /api/streams/{jobId}/renders/{variant}/videos/{clipId} — stream a rendered Short mp4. */
export async function GET(
  request: Request,
  { params }: { params: Promise<{ jobId: string; variant: string; clipId: string }> },
): Promise<Response> {
  const { jobId, variant, clipId } = await params;
  if (!VARIANT_RE.test(variant) || !CLIP_ID_RE.test(clipId)) {
    return NextResponse.json({ error: 'invalid path' }, { status: 400 });
  }
  const url = streamJobUrl(jobId, `/renders/${variant}/videos/${clipId}`);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  return proxyStream(url, 'video/mp4', request);
}

/** DELETE /api/streams/{jobId}/renders/{variant}/videos/{clipId}: 204, or 409 while that variant renders. */
export async function DELETE(
  request: Request,
  { params }: { params: Promise<{ jobId: string; variant: string; clipId: string }> },
): Promise<Response> {
  const localError = await localAPIRequestError(request.headers, request.method);
  if (localError !== undefined) return NextResponse.json({ error: localError }, { status: 403 });
  const { jobId, variant, clipId } = await params;
  if (!VARIANT_RE.test(variant) || !CLIP_ID_RE.test(clipId)) {
    return NextResponse.json({ error: 'invalid path' }, { status: 400 });
  }
  const url = streamJobUrl(jobId, `/renders/${variant}/videos/${clipId}`);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  const res = await callOrchestrator(url, { method: 'DELETE' });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return new Response(null, { status: 204 });
}
