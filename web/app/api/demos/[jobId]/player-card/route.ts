import { NextResponse } from 'next/server';
import { jobUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../_lib';

export const runtime = 'nodejs';

/** GET /api/demos/{jobId}/player-card: the card catalog and the player a one-player Short shows. */
export async function GET(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = jobUrl(jobId, '/player-card');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  const response = await callOrchestrator(url, { cache: 'no-store', signal: request.signal });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const body: unknown = await response.json();
  return NextResponse.json(body, { headers: { 'Cache-Control': 'private, no-store' } });
}
