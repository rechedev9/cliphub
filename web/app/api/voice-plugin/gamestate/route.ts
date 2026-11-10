import { NextResponse } from 'next/server';
import { readBoundedText } from '@/lib/api/bounded-request-body';
import { localAPIRequestError } from '@/lib/api/local-request-guard';
import { callOrchestrator, orchestratorUrl, serviceUnavailable } from '../../demos/_lib';

export const runtime = 'nodejs';

/** CS2 sends a few KiB per update; the orchestrator refuses past 64 KiB as well. */
const MAX_GAME_STATE_BYTES = 64 * 1024;

/**
 * POST /api/voice-plugin/gamestate receives CS2's game state integration. CS2 has no
 * capability cookie, so proxy.ts skips this path and the read guard runs here: loopback
 * Host and no foreign Origin. The orchestrator checks the config's token.
 */
export async function POST(request: Request): Promise<Response> {
  const localError = await localAPIRequestError(request.headers, 'GET');
  if (localError !== undefined) return NextResponse.json({ error: localError }, { status: 403 });

  const body = await readBoundedText(request, MAX_GAME_STATE_BYTES);
  if (!body.ok) return NextResponse.json({ error: body.error }, { status: body.status });

  const upstream = await callOrchestrator(`${orchestratorUrl()}/api/voice-plugin/gamestate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: body.text,
  });
  if (upstream === null) return serviceUnavailable();
  return new Response(null, { status: upstream.ok ? 204 : upstream.status });
}
