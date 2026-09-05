import { NextResponse } from 'next/server.js';
import { orchestratorUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../../../demos/_lib.ts';

export const runtime = 'nodejs';

/** Same-origin preview of an already imported local asset; no capture or render. */
export async function GET(request: Request, { params }: { params: Promise<{ assetId: string }> }): Promise<Response> {
  const { assetId } = await params;
  if (!/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(assetId)) return NextResponse.json({ code: 'invalid_asset', error: 'invalid asset id' }, { status: 400 });
  const range = request.headers.get('range');
  const response = await callOrchestrator(`${orchestratorUrl()}/api/editor/assets/${assetId}/media`, {
    headers: range === null ? {} : { Range: range }, signal: request.signal,
  });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const headers = new Headers({ 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff' });
  for (const name of ['content-type', 'content-length', 'accept-ranges', 'content-range']) {
    const value = response.headers.get(name); if (value !== null) headers.set(name, value);
  }
  return new Response(response.body, { status: response.status, headers });
}
