import { NextResponse } from 'next/server';
import { editorProjectUrl, callOrchestrator, mutationHeaders, forwardError, serviceUnavailable } from '../../../_lib';

export const runtime = 'nodejs';

export async function GET(_request: Request, { params }: { params: Promise<{ id: string }> }): Promise<Response> {
  const { id } = await params;
  const url = editorProjectUrl(id, '/plan');
  if (!url) return NextResponse.json({ error: 'invalid project id' }, { status: 400 });
  const res = await callOrchestrator(url);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json((await res.json()) as unknown);
}

export async function PUT(request: Request, { params }: { params: Promise<{ id: string }> }): Promise<Response> {
  const { id } = await params;
  const url = editorProjectUrl(id, '/plan');
  if (!url) return NextResponse.json({ error: 'invalid project id' }, { status: 400 });
  const res = await callOrchestrator(url, {
    method: 'PUT',
    headers: { ...mutationHeaders(), 'Content-Type': 'application/json' },
    body: await request.text(),
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json((await res.json()) as unknown);
}
