import { NextResponse } from 'next/server';
import { jobUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../../../../_lib';

export const runtime = 'nodejs';

const VARIANT_RE = /^[A-Za-z0-9][A-Za-z0-9_-]*$/;
const REVISION_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

/** DELETE /api/demos/{jobId}/renders/{variant}/versions/{revision}: an older version; 409 for the current one. */
export async function DELETE(_request: Request, { params }: { params: Promise<{ jobId: string; variant: string; revision: string }> }): Promise<Response> {
  const { jobId, variant, revision } = await params;
  if (!VARIANT_RE.test(variant) || !REVISION_RE.test(revision)) return NextResponse.json({ error: 'invalid path' }, { status: 400 });
  const url = jobUrl(jobId, `/renders/${variant}/versions/${revision}`);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  const response = await callOrchestrator(url, { method: 'DELETE' });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  return NextResponse.json({ deleted: revision });
}
