import { NextResponse } from 'next/server';
import { readBoundedText } from '@/lib/api/bounded-request-body';
import { streamJobUrl, mutationHeaders, forwardError, callOrchestrator, serviceUnavailable } from '../../_lib';

export const runtime = 'nodejs';

export async function GET(_request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = streamJobUrl(jobId, '/transcript');
  if (!url) return NextResponse.json({ error: 'Identificador de vídeo no válido.' }, { status: 400 });

  const res = await callOrchestrator(url);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const body: unknown = await res.json();
  return NextResponse.json(body);
}

export async function POST(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = streamJobUrl(jobId, '/transcript');
  if (!url) return NextResponse.json({ error: 'Identificador de vídeo no válido.' }, { status: 400 });

  const incoming = await readBoundedText(request);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  const res = await callOrchestrator(url, {
    method: 'POST',
    headers: { ...mutationHeaders(), 'Content-Type': 'application/json' },
    body: incoming.text,
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const body: unknown = await res.json();
  return NextResponse.json(body, { status: res.status });
}
