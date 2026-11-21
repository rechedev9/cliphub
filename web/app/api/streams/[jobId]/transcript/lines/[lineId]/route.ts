import { NextResponse } from 'next/server';
import { readBoundedText } from '@/lib/api/bounded-request-body';
import { streamJobUrl, mutationHeaders, forwardError, callOrchestrator, serviceUnavailable } from '../../../../_lib';

export const runtime = 'nodejs';

const UUID_RE = /^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i;

export async function PATCH(request: Request, { params }: { params: Promise<{ jobId: string; lineId: string }> }): Promise<Response> {
  const { jobId, lineId } = await params;
  const url = UUID_RE.test(lineId) ? streamJobUrl(jobId, `/transcript/lines/${lineId}`) : null;
  if (!url) return NextResponse.json({ error: 'Identificador no válido.' }, { status: 400 });

  const incoming = await readBoundedText(request);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  const res = await callOrchestrator(url, {
    method: 'PATCH',
    headers: { ...mutationHeaders(), 'Content-Type': 'application/json' },
    body: incoming.text,
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const body: unknown = await res.json();
  return NextResponse.json(body);
}
