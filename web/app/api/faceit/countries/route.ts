import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, serviceUnavailable, forwardError } from '../../demos/_lib';

export const runtime = 'nodejs';

export async function GET(): Promise<Response> {
  const res = await callOrchestrator(`${orchestratorUrl()}/api/faceit/countries`);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const data: unknown = await res.json();
  const listed = typeof data === 'object' && data !== null && 'countries' in data ? data.countries : undefined;
  const countries = Array.isArray(listed)
    ? listed.filter((code): code is string => typeof code === 'string' && /^[a-z]{2}$/.test(code))
    : [];
  return NextResponse.json({ countries }, { headers: { 'cache-control': 'private, max-age=3600' } });
}
