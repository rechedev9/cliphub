import { NextResponse } from 'next/server';
import { jobUrl, forwardError, callOrchestrator, serviceUnavailable } from '../../_lib';

export const runtime = 'nodejs';

const NUMBER_RE = /^\d+(?:\.\d+)?$/;

/** GET /api/demos/{jobId}/auto-pov: the Auto-POV pick of a summary; only numeric target_seconds, extra_seconds and pre_roll_seconds travel. */
export async function GET(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const incoming = new URL(request.url).searchParams;
  const query = new URLSearchParams();
  for (const key of ['target_seconds', 'extra_seconds', 'pre_roll_seconds']) {
    const value = incoming.get(key);
    if (value === null) continue;
    if (!NUMBER_RE.test(value)) return NextResponse.json({ error: `invalid ${key}` }, { status: 400 });
    query.set(key, value);
  }
  const suffix = query.size > 0 ? `/auto-pov?${query.toString()}` : '/auto-pov';
  const url = jobUrl(jobId, suffix);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const res = await callOrchestrator(url, { cache: 'no-store' });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  const body: unknown = await res.json();
  return NextResponse.json(body, { headers: { 'cache-control': 'no-store' } });
}
