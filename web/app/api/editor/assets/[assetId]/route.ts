import { NextResponse } from 'next/server.js';
import { publicEditorAsset } from '../../../../../lib/api/public-projections.ts';
import { orchestratorUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../../demos/_lib.ts';

export const runtime = 'nodejs';

/** Restore a local clip's display name without exposing its storage location. */
export async function GET(request: Request, { params }: { params: Promise<{ assetId: string }> }): Promise<Response> {
  const { assetId } = await params;
  if (!/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(assetId)) return NextResponse.json({ code: 'invalid_asset', error: 'invalid asset id' }, { status: 400 });
  const response = await callOrchestrator(`${orchestratorUrl()}/api/editor/assets/${assetId}`, { signal: request.signal });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  return NextResponse.json(publicEditorAsset(await response.json()), { headers: { 'Cache-Control': 'private, no-store' } });
}
