import { orchestratorUrl, callOrchestrator, serviceUnavailable } from '../../../../demos/_lib';
import { FACEIT_PLAYER_ID_RE } from '../../../_lib';

export const runtime = 'nodejs';

export async function GET(
  _request: Request,
  context: { params: Promise<{ playerID: string }> },
): Promise<Response> {
  const { playerID } = await context.params;
  if (!FACEIT_PLAYER_ID_RE.test(playerID)) {
    return new Response(null, { status: 404 });
  }
  const res = await callOrchestrator(
    `${orchestratorUrl()}/api/faceit/players/${encodeURIComponent(playerID)}/avatar`,
  );
  if (res === null) return serviceUnavailable();
  // No cached avatar (or FACEIT's CDN refused it) is an expected state, not an
  // error: a 404 here logged one console error per player row. An empty 204
  // still fails the <img> load, so the caller falls back to the initial.
  if (!res.ok) return new Response(null, { status: 204, headers: { 'cache-control': 'no-store' } });
  const ct = res.headers.get('content-type') ?? 'image/jpeg';
  return new Response(res.body, {
    status: 200,
    headers: {
      'content-type': ct,
      'cache-control': 'public, max-age=3600',
    },
  });
}
