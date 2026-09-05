import { NextResponse } from 'next/server';
import { proxyStream, streamJobUrl } from '../../../../../../../_lib';

export const runtime = 'nodejs';

const PATH_PART_RE = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;

/** Streams one retained revision artifact, such as its cover image. */
export async function GET(
  request: Request,
  { params }: { params: Promise<{ jobId: string; variant: string; revision: string; name: string }> },
): Promise<Response> {
  const { jobId, variant, revision, name } = await params;
  if (![variant, revision, name].every((value) => PATH_PART_RE.test(value))) {
    return NextResponse.json({ error: 'invalid path' }, { status: 400 });
  }
  const url = streamJobUrl(jobId, `/renders/${variant}/revisions/${revision}/delivery/${encodeURIComponent(name)}`);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  return proxyStream(url, 'application/octet-stream', request);
}
