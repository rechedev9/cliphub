import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, serviceUnavailable, forwardError } from '../../demos/_lib';

export const runtime = 'nodejs';

const KINDS = new Set(['country', 'region', 'zone']);

function fields(value: unknown): Map<string, unknown> {
  return new Map(typeof value === 'object' && value !== null && !Array.isArray(value) ? Object.entries(value) : []);
}

function whitelistRankedPlayer(raw: unknown): Record<string, unknown> | null {
  const row = fields(raw);
  const rank = row.get('rank');
  const id = row.get('id');
  const nickname = row.get('nickname');
  const profileURL = row.get('profile_url');
  if (typeof rank !== 'number' || typeof id !== 'string' || typeof nickname !== 'string' || typeof profileURL !== 'string') return null;
  const player: Record<string, unknown> = { rank, id, nickname, profile_url: profileURL };
  for (const key of ['country', 'region']) {
    const value = row.get(key);
    if (typeof value === 'string') player[key] = value;
  }
  for (const key of ['position', 'elo', 'skill_level']) {
    const value = row.get(key);
    if (typeof value === 'number') player[key] = value;
  }
  return player;
}

export async function GET(request: Request): Promise<Response> {
  const params = new URL(request.url).searchParams;
  const kind = params.get('kind') ?? '';
  const code = params.get('code') ?? '';
  const offset = params.get('offset') ?? '0';
  const limit = params.get('limit') ?? '20';
  if (!KINDS.has(kind) || !/^[A-Za-z]{2,5}$/.test(code) || !/^\d{1,3}$/.test(offset) || !/^\d{1,3}$/.test(limit)) {
    return NextResponse.json({ error: 'FACEIT ranking request is invalid' }, { status: 400 });
  }
  const upstream = new URLSearchParams({ kind, code, offset, limit });
  const res = await callOrchestrator(`${orchestratorUrl()}/api/faceit/rankings?${upstream.toString()}`);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const data = fields(await res.json());
  const rows = data.get('players');
  const players = Array.isArray(rows) ? rows.map(whitelistRankedPlayer).filter((player) => player !== null) : [];
  const total = data.get('total');
  const staleCode = data.get('stale_code');
  return NextResponse.json(
    {
      kind: data.get('kind'),
      code: data.get('code'),
      total: typeof total === 'number' ? total : players.length,
      fetched_at: data.get('fetched_at'),
      stale: data.get('stale') === true,
      ...(typeof staleCode === 'string' ? { stale_code: staleCode } : {}),
      players,
    },
    { headers: { 'cache-control': 'no-store' } },
  );
}
