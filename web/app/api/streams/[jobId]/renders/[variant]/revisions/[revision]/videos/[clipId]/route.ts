import { NextResponse } from 'next/server';
import { proxyStream, streamJobUrl } from '../../../../../../../_lib';

export const runtime = 'nodejs';

const PATH_PART_RE = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;

/** Streams one immutable render revision without exposing orchestrator credentials. */
export async function GET(
  request: Request,
  { params }: { params: Promise<{ jobId: string; variant: string; revision: string; clipId: string }> },
): Promise<Response> {
  const { jobId, variant, revision, clipId } = await params;
  if (![variant, revision, clipId].every((value) => PATH_PART_RE.test(value))) {
    return NextResponse.json({ error: 'invalid path' }, { status: 400 });
  }
  const url = streamJobUrl(jobId, `/renders/${variant}/revisions/${revision}/videos/${clipId}`);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  return proxyStream(url, 'video/mp4', request);
}
