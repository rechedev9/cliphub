import { NextResponse } from 'next/server';
import { editorProjectUrl, proxyStream } from '../../../../_lib';

export const runtime = 'nodejs';

export async function GET(request: Request, { params }: { params: Promise<{ id: string }> }): Promise<Response> {
  const { id } = await params;
  const url = editorProjectUrl(id, '/render/video');
  if (!url) return NextResponse.json({ error: 'invalid project id' }, { status: 400 });
  return proxyStream(url, 'video/mp4', request);
}
