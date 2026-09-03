import { NextResponse } from 'next/server';
import { localAPIRequestError } from '@/lib/api/local-request-guard';
import { editorAssetUrl, callOrchestrator, forwardError, serviceUnavailable, publicEditorAsset } from '../../_lib';

export const runtime = 'nodejs';

export async function GET(_request: Request, { params }: { params: Promise<{ id: string }> }): Promise<Response> {
  const { id } = await params;
  const url = editorAssetUrl(id);
  if (!url) return NextResponse.json({ error: 'invalid asset id' }, { status: 400 });
  const res = await callOrchestrator(url);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json(publicEditorAsset(await res.json()));
}

/** DELETE /api/editor/assets/{id} — 204; 409 while a render still owns it. */
export async function DELETE(request: Request, { params }: { params: Promise<{ id: string }> }): Promise<Response> {
  const localError = await localAPIRequestError(request.headers, request.method);
  if (localError !== undefined) return NextResponse.json({ error: localError }, { status: 403 });
  const { id } = await params;
  const url = editorAssetUrl(id);
  if (!url) return NextResponse.json({ error: 'invalid asset id' }, { status: 400 });
  const res = await callOrchestrator(url, { method: 'DELETE' });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return new Response(null, { status: 204 });
}
