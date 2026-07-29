import { NextResponse } from 'next/server';
import { readBoundedText } from '@/lib/api/bounded-request-body';
import {
  callOrchestrator,
  forwardError,
  jobUrl,
  mutationHeaders,
  serviceUnavailable,
} from '../../../../_lib';

export const runtime = 'nodejs';

const VARIANT_RE = /^[A-Za-z0-9][A-Za-z0-9_-]*$/;

/** POST — persist a human resolution for the current render's exact QA warnings. */
export async function POST(
  request: Request,
  { params }: { params: Promise<{ jobId: string; variant: string }> },
): Promise<Response> {
  const { jobId, variant } = await params;
  if (!VARIANT_RE.test(variant)) {
    return NextResponse.json({ error: 'invalid variant' }, { status: 400 });
  }
  const url = jobUrl(jobId, `/renders/${variant}/review`);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  const incoming = await readBoundedText(request);
  if (!incoming.ok) {
    return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  }
  if (!incoming.text) {
    return NextResponse.json({ error: 'review resolution body is required' }, { status: 400 });
  }
  const res = await callOrchestrator(url, {
    method: 'POST',
    headers: { ...mutationHeaders(), 'Content-Type': 'application/json' },
    body: incoming.text,
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json((await res.json()) as unknown);
}
