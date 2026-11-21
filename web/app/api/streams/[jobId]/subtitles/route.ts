import { NextResponse } from 'next/server';
import { readBoundedText } from '@/lib/api/bounded-request-body';
import { streamJobUrl, mutationHeaders, forwardError, callOrchestrator, serviceUnavailable } from '../../_lib';

export const runtime = 'nodejs';

export async function PATCH(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = streamJobUrl(jobId, '/subtitles');
  if (!url) return NextResponse.json({ error: 'Identificador de vídeo no válido.' }, { status: 400 });

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
