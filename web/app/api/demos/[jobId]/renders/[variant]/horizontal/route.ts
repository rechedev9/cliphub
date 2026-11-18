import { NextResponse } from 'next/server';
import { jobUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../../../_lib';

export const runtime = 'nodejs';

const VARIANT_RE = /^[A-Za-z0-9][A-Za-z0-9_-]*$/;

/** GET /api/demos/{jobId}/renders/{variant}/horizontal: the 16:9 summary draft of a Short (Crear horizontal). */
export async function GET(_request: Request, { params }: { params: Promise<{ jobId: string; variant: string }> }): Promise<Response> {
  const { jobId, variant } = await params;
  if (!VARIANT_RE.test(variant)) return NextResponse.json({ error: 'invalid variant' }, { status: 400 });
  const url = jobUrl(jobId, `/renders/${variant}/horizontal`);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  const response = await callOrchestrator(url, { cache: 'no-store' });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const body: unknown = await response.json();
  return NextResponse.json(body, { headers: { 'Cache-Control': 'private, no-store' } });
}
