import { NextResponse } from 'next/server';
import { callOrchestrator, forwardError, orchestratorUrl, proxyStream, serviceUnavailable, UUID_RE } from '../../../demos/_lib';

export const runtime = 'nodejs';

const VIDEO_NAME_RE = /^[A-Za-z0-9][A-Za-z0-9._-]{0,119}\.mp4$/;

/** Upstream URL of one imported video; null unless the id is a UUID and the name a plain mp4 name. */
function importedVideoUrl(id: string, name: string): string | null {
  if (!UUID_RE.test(id) || !VIDEO_NAME_RE.test(name)) return null;
  return `${orchestratorUrl()}/api/imported-videos/${id}/${name}`;
}

type Params = { params: Promise<{ id: string; name: string }> };

/** GET /api/imported-videos/{id}/{name}: the MP4 from this PC's disk, with Range so the player can seek. */
export async function GET(request: Request, { params }: Params): Promise<Response> {
  const { id, name } = await params;
  const url = importedVideoUrl(id, name);
  if (url === null) return NextResponse.json({ error: 'invalid path' }, { status: 400 });
  return proxyStream(url, 'video/mp4', request);
}

/** DELETE /api/imported-videos/{id}/{name}: removes the MP4 for good. */
export async function DELETE(_request: Request, { params }: Params): Promise<Response> {
  const { id, name } = await params;
  const url = importedVideoUrl(id, name);
  if (url === null) return NextResponse.json({ error: 'invalid path' }, { status: 400 });
  const res = await callOrchestrator(url, { method: 'DELETE' });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return new Response(null, { status: 204 });
}
