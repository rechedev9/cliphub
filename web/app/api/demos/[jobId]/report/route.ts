import { NextResponse } from 'next/server';
import { parseControlJSONObject, readBoundedText } from '@/lib/api/bounded-request-body';
import { isJobReportCategory } from '@/lib/api/job-report';
import { jobUrl, mutationHeaders, forwardError, callOrchestrator, serviceUnavailable } from '../../_lib';

export const runtime = 'nodejs';

const MAX_REPORT_BODY_BYTES = 1024;

/**
 * POST /api/demos/{jobId}/report — "Reportar un problema con este vídeo".
 * Only a known category crosses to the orchestrator, which records it on the
 * job's trace and answers 202, or 429 for a repeat within a minute.
 */
export async function POST(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = jobUrl(jobId, '/report');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const incoming = await readBoundedText(request, MAX_REPORT_BODY_BYTES);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  const parsed = parseControlJSONObject(incoming.text, ['category']);
  if (!parsed.ok) return NextResponse.json({ error: parsed.error }, { status: 400 });
  const { category } = parsed.value;
  if (!isJobReportCategory(category)) return NextResponse.json({ error: 'invalid category' }, { status: 400 });

  const res = await callOrchestrator(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...mutationHeaders() },
    body: JSON.stringify({ category }),
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json({ status: 'reported' }, { status: 202 });
}
