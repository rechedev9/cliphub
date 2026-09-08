import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../../demos/_lib';

export const runtime = 'nodejs';

export async function GET(request: Request, { params }: { params: Promise<{ assetId: string }> }): Promise<Response> {
  const { assetId } = await params;
  if (!/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(assetId)) return NextResponse.json({ error: 'Referencia de captura inválida.' }, { status: 400 });
  const response = await callOrchestrator(`${orchestratorUrl()}/api/full-demo/overlay-images/${assetId}`, { signal: request.signal });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const headers = new Headers({ 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff' });
  for (const name of ['content-type', 'content-length']) { const value = response.headers.get(name); if (value !== null) headers.set(name, value); }
  return new Response(response.body, { status: response.status, headers });
}
