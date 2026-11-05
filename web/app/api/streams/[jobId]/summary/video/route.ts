import { NextResponse } from 'next/server';
import { streamJobUrl, proxyStream } from '../../../_lib';

export const runtime = 'nodejs';

/** GET /api/streams/{jobId}/summary/video — the summary MP4; a re-render replaces it, so it is not cached. */
export async function GET(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = streamJobUrl(jobId, '/summary/video');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  return proxyStream(url, 'video/mp4', request);
}
