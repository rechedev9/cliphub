/** One round to replay, optionally starting at a tick. */
export type ReplayRef = { matchId: string; round: number; tick?: number };

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
/** Long playlists are cut so the URL stays well under browser limits. */
export const REPLAY_MAX_REFS = 200;

/** The replay page with these rounds in its strip, in order. */
export function replayHref(refs: readonly ReplayRef[], title?: string): string {
  const params = new URLSearchParams();
  for (const ref of refs.slice(0, REPLAY_MAX_REFS)) {
    params.append('r', ref.tick === undefined ? `${ref.matchId}:${ref.round}` : `${ref.matchId}:${ref.round}:${Math.round(ref.tick)}`);
  }
  if (title) params.set('t', title);
  return `/replay?${params.toString()}`;
}

/** Reads the refs back; malformed entries are dropped. */
export function parseReplayRefs(query: Pick<URLSearchParams, 'getAll'>): ReplayRef[] {
  const out: ReplayRef[] = [];
  for (const raw of query.getAll('r').slice(0, REPLAY_MAX_REFS)) {
    const [matchId = '', roundText = '', tickText] = raw.split(':');
    const round = Number(roundText);
    if (!UUID.test(matchId) || !Number.isInteger(round) || round < 1) continue;
    const tick = tickText === undefined ? undefined : Number(tickText);
    out.push(tick !== undefined && Number.isInteger(tick) && tick >= 0 ? { matchId, round, tick } : { matchId, round });
  }
  return out;
}
