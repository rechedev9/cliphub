import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, forwardError, serviceUnavailable } from '../demos/_lib';

export const runtime = 'nodejs';

/** GET /api/songs — proxy the orchestrator's Suno catalog under ZV_MUSIC_DIR. */
export async function GET(): Promise<Response> {
  const res = await callOrchestrator(`${orchestratorUrl()}/api/songs`, { cache: 'no-store' });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json(await res.json());
}
