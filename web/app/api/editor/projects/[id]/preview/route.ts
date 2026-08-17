import { NextResponse } from 'next/server';
import { editorProjectUrl, callOrchestrator, mutationHeaders, forwardError, serviceUnavailable } from '../../../_lib';

export const runtime = 'nodejs';

export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }): Promise<Response> {
  const { id } = await params;
  const url = editorProjectUrl(id, '/preview');
  if (!url) return NextResponse.json({ error: 'invalid project id' }, { status: 400 });
  const res = await callOrchestrator(url, {
    method: 'POST',
    headers: { ...mutationHeaders(), 'Content-Type': 'application/json' },
    body: await request.text(),
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json((await res.json()) as unknown);
}
