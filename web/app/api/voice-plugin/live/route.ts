import { callOrchestrator, orchestratorUrl, serviceUnavailable } from '../../demos/_lib';

export const runtime = 'nodejs';

/** Bounds what reaches the TeamSpeak plugin's parser: a header and at most a few dozen rounds. */
const MAX_LIVE_BYTES = 16 * 1024;

/** GET /api/voice-plugin/live tells the TeamSpeak plugin whether CS2 reports a match, as plain text lines. */
export async function GET(): Promise<Response> {
  const upstream = await callOrchestrator(`${orchestratorUrl()}/api/voice-plugin/live`);
  if (upstream === null) return serviceUnavailable();
  if (!upstream.ok) return new Response('live 0\n', { status: upstream.status, headers: plainHeaders() });
  const text = await upstream.text();
  if (text.length > MAX_LIVE_BYTES) return new Response('live 0\n', { status: 502, headers: plainHeaders() });
  return new Response(text, { status: 200, headers: plainHeaders() });
}

function plainHeaders(): Record<string, string> {
  return { 'content-type': 'text/plain; charset=utf-8', 'cache-control': 'no-store', 'x-content-type-options': 'nosniff' };
}
