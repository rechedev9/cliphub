import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, serviceUnavailable, forwardError } from '../../../demos/_lib';
import { whitelistSteamAccount, type UpstreamSteamAccount } from '../../_lib';

export const runtime = 'nodejs';

export async function POST(): Promise<Response> {
  const res = await callOrchestrator(`${orchestratorUrl()}/api/steam/matches/sync`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: '{}',
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const data = (await res.json()) as UpstreamSteamAccount;
  return NextResponse.json(whitelistSteamAccount(data), { headers: { 'cache-control': 'no-store' } });
}
