import { NextResponse } from 'next/server';
import { editorAssetUrl, proxyStream } from '../../../_lib';

export const runtime = 'nodejs';

export async function GET(request: Request, { params }: { params: Promise<{ id: string }> }): Promise<Response> {
  const { id } = await params;
  const url = editorAssetUrl(id, '/media');
  if (!url) return NextResponse.json({ error: 'invalid asset id' }, { status: 400 });
  return proxyStream(url, 'video/mp4', request);
}
