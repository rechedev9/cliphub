import { NextResponse } from 'next/server';
import { readBoundedText } from '@/lib/api/bounded-request-body';
import { jobUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../../_lib';

export const runtime = 'nodejs';

/** POST /api/demos/{jobId}/full-demo/capture-check: which rounds these options would record; saves nothing. */
export async function POST(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = jobUrl(jobId, '/full-demo/capture-check');
  if (!url) return NextResponse.json({ code: 'invalid_job_id', error: 'invalid job id' }, { status: 400 });
  const incoming = await readBoundedText(request);
  if (!incoming.ok) return NextResponse.json({ code: 'invalid_full_demo_options', error: incoming.error }, { status: incoming.status });
  const response = await callOrchestrator(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: incoming.text });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const body: unknown = await response.json();
  return NextResponse.json(body);
}
