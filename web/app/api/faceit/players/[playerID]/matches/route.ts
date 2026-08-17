import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, serviceUnavailable, forwardError } from '../../../../demos/_lib';
import { FACEIT_PLAYER_ID_RE } from '../../../_lib';

export const runtime = 'nodejs';

export async function GET(
  request: Request,
  context: { params: Promise<{ playerID: string }> },
): Promise<Response> {
  const { playerID } = await context.params;
  if (!FACEIT_PLAYER_ID_RE.test(playerID)) {
    return NextResponse.json({ error: 'FACEIT player id is invalid' }, { status: 400 });
  }
  const limit = new URL(request.url).searchParams.get('limit') ?? '10';
  if (!/^\d{1,2}$/.test(limit)) {
    return NextResponse.json({ error: 'limit must be an integer' }, { status: 400 });
  }
  const res = await callOrchestrator(
    `${orchestratorUrl()}/api/faceit/players/${encodeURIComponent(playerID)}/matches?limit=${limit}`,
  );
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const data = (await res.json()) as { matches?: unknown };
  if (!Array.isArray(data.matches)) {
    return NextResponse.json({ error: 'FACEIT match list is invalid' }, { status: 502 });
  }
  return NextResponse.json(
    { matches: data.matches.map(whitelistMatch).filter((match) => match !== null) },
    { headers: { 'cache-control': 'no-store' } },
  );
}

type UpstreamMatch = {
  id?: unknown;
  room_url?: unknown;
  started_at?: unknown;
  finished_at?: unknown;
  competition?: unknown;
  score?: unknown;
  stats?: unknown;
};

function whitelistMatch(raw: unknown): Record<string, unknown> | null {
  const match = raw as UpstreamMatch;
  if (typeof match.id !== 'string' || typeof match.room_url !== 'string') return null;
  const out: Record<string, unknown> = { id: match.id, room_url: match.room_url };
  if (typeof match.started_at === 'string') out.started_at = match.started_at;
  if (typeof match.finished_at === 'string') out.finished_at = match.finished_at;
  if (typeof match.competition === 'string') out.competition = match.competition;
  if (match.score !== null && typeof match.score === 'object' && !Array.isArray(match.score)) {
    const score = match.score as Record<string, unknown>;
    out.score = {
      player_team: typeof score.player_team === 'string' ? score.player_team : undefined,
      winner_team: typeof score.winner_team === 'string' ? score.winner_team : undefined,
      for: typeof score.for === 'number' ? score.for : undefined,
      against: typeof score.against === 'number' ? score.against : undefined,
    };
  }
  if (match.stats !== null && typeof match.stats === 'object' && !Array.isArray(match.stats)) {
    const stats = match.stats as Record<string, unknown>;
    out.stats = {
      map: typeof stats.map === 'string' ? stats.map : undefined,
      result: stats.result === 'win' || stats.result === 'loss' ? stats.result : 'unknown',
      kills: typeof stats.kills === 'number' ? stats.kills : 0,
      deaths: typeof stats.deaths === 'number' ? stats.deaths : 0,
      assists: typeof stats.assists === 'number' ? stats.assists : 0,
      adr: typeof stats.adr === 'number' ? stats.adr : undefined,
      kd_ratio: typeof stats.kd_ratio === 'number' ? stats.kd_ratio : undefined,
      headshots_percent: typeof stats.headshots_percent === 'number' ? stats.headshots_percent : undefined,
    };
  }
  return out;
}
