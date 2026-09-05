import { NextResponse } from 'next/server';
import { jobUrl, proxyStream } from '../../../../../../../_lib';

export const runtime = 'nodejs';

const VARIANT_RE = /^[A-Za-z0-9][A-Za-z0-9_-]*$/;
const REVISION_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const NAME_RE = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;

/** Streams one cover from an immutable demo render revision. */
export async function GET(
  request: Request,
  { params }: { params: Promise<{ jobId: string; variant: string; revision: string; name: string }> },
): Promise<Response> {
  const { jobId, variant, revision, name } = await params;
  if (!VARIANT_RE.test(variant) || !REVISION_RE.test(revision) || !NAME_RE.test(name)) {
    return NextResponse.json({ error: 'invalid path' }, { status: 400 });
  }
  const url = jobUrl(jobId, `/renders/${variant}/revisions/${revision}/covers/${name}`);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  return proxyStream(url, 'image/jpeg', request);
}
