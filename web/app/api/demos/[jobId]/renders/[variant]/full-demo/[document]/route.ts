import { NextResponse } from 'next/server';
import { requestedRenderRevision } from '@/lib/api/render-revision';
import { jobUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../../../../_lib';

export const runtime = 'nodejs';
const DOCUMENTS = new Set(['approved', 'effective', 'audio', 'loudness', 'delivery']);

export async function GET(request: Request, { params }: { params: Promise<{ jobId: string; variant: string; document: string }> }): Promise<Response> {
  const { jobId, variant, document } = await params;
  if (!/^[a-z0-9][a-z0-9_-]{0,63}$/.test(variant) || !DOCUMENTS.has(document)) return NextResponse.json({ code: 'invalid_full_demo_document', error: 'invalid document' }, { status: 400 });
  const revision = requestedRenderRevision(request);
  if (revision === null) return NextResponse.json({ code: 'invalid_revision', error: 'invalid revision' }, { status: 400 });
  const url = jobUrl(jobId, `/renders/${variant}${revision}/full-demo/${document}`);
  if (!url) return NextResponse.json({ code: 'invalid_job_id', error: 'invalid job id' }, { status: 400 });
  const response = await callOrchestrator(url, { cache: 'no-store' });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const body: unknown = await response.json();
  return NextResponse.json(body, { headers: { 'Cache-Control': 'private, no-store' } });
}
