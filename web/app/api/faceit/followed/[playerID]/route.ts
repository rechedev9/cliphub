import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, serviceUnavailable, forwardError } from '../../../demos/_lib';
import { FACEIT_PLAYER_ID_RE } from '../../_lib';

export const runtime = 'nodejs';

export async function DELETE(
  _request: Request,
  context: { params: Promise<{ playerID: string }> },
): Promise<Response> {
  const { playerID } = await context.params;
  if (!FACEIT_PLAYER_ID_RE.test(playerID)) {
    return NextResponse.json({ error: 'FACEIT player id is invalid' }, { status: 400 });
  }
  const res = await callOrchestrator(`${orchestratorUrl()}/api/faceit/followed/${encodeURIComponent(playerID)}`, {
    method: 'DELETE',
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return new Response(null, { status: 204, headers: { 'cache-control': 'no-store' } });
}
