import { NextResponse } from 'next/server';
import { callOrchestrator, forwardError, orchestratorUrl, serviceUnavailable } from '../demos/_lib';

export const runtime = 'nodejs';

/** GET /api/imported-videos: Shorts an older Studio downloaded from the retired ClipHub cloud. */
export async function GET(): Promise<Response> {
  const res = await callOrchestrator(`${orchestratorUrl()}/api/imported-videos`, { cache: 'no-store' });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json(await res.json(), { headers: { 'cache-control': 'no-store' } });
}
