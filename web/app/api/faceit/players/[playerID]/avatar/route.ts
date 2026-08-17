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
  if (!res.ok) return new Response(null, { status: 404 });
  const ct = res.headers.get('content-type') ?? 'image/jpeg';
  return new Response(res.body, {
    status: 200,
    headers: {
      'content-type': ct,
      'cache-control': 'public, max-age=3600',
    },
  });
}
