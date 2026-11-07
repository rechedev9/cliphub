import { NextResponse } from 'next/server';
import { parseControlJSONObject, readBoundedText } from '@/lib/api/bounded-request-body';
import { jobUrl, mutationHeaders, forwardError, callOrchestrator, parseTargetBody, serviceUnavailable } from '../../_lib';

export const runtime = 'nodejs';

/**
 * POST /api/demos/{jobId}/parse — start a parse for the chosen player. Maps the
 * client's { steamId } to the orchestrator's { target_steamid }, or { steamIds }
 * to { target_steamids } for a multi-POV job, and forwards the mutation token.
 */
export async function POST(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = jobUrl(jobId, '/parse');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const incoming = await readBoundedText(request);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  const parsed = parseControlJSONObject(incoming.text, ['steamId', 'steamIds']);
  if (!parsed.ok) return NextResponse.json({ error: parsed.error }, { status: 400 });
  const target = parseTargetBody(parsed.value);
  if (target === null) {
    return NextResponse.json({ error: 'steamId or a steamIds list required' }, { status: 400 });
  }

  const res = await callOrchestrator(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...mutationHeaders() },
    body: JSON.stringify(target),
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  const body = (await res.json()) as { id: string; status: string };
  return NextResponse.json(body);
}
