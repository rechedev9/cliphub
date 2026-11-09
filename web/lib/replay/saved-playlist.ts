import { REPLAY_MAX_REFS } from '../analysis/replay-link.ts';
import type { ReplayRef } from '../analysis/replay-link.ts';

/** The rounds set aside with B in the Replay, kept on this PC until they are opened as a playlist. */
export const PLAYLIST_KEY = 'cliphub.replay.playlist';
export const PLAYLIST_TITLE = 'Mi playlist';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function sameRound(a: ReplayRef, b: ReplayRef): boolean {
  return a.matchId === b.matchId && a.round === b.round;
}

export function inPlaylist(list: readonly ReplayRef[], ref: ReplayRef): boolean {
  return list.some((entry) => sameRound(entry, ref));
}

/** Adds the round at the end, or takes it out when it is already there. */
export function togglePlaylistRound(list: readonly ReplayRef[], ref: ReplayRef): ReplayRef[] {
  if (inPlaylist(list, ref)) return list.filter((entry) => !sameRound(entry, ref));
  return [...list, { matchId: ref.matchId, round: ref.round }].slice(-REPLAY_MAX_REFS);
}

/** Reads the stored playlist; anything that is not a round ref is dropped. */
export function parsePlaylist(raw: string | null): ReplayRef[] {
  if (raw === null) return [];
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return [];
  }
  if (!Array.isArray(parsed)) return [];
  const out: ReplayRef[] = [];
  for (const item of parsed) {
    if (typeof item !== 'object' || item === null) continue;
    const entry: { matchId?: unknown; round?: unknown } = item;
    if (typeof entry.matchId !== 'string' || !UUID.test(entry.matchId)) continue;
    if (typeof entry.round !== 'number' || !Number.isInteger(entry.round) || entry.round < 1) continue;
    const ref = { matchId: entry.matchId, round: entry.round };
    if (!inPlaylist(out, ref)) out.push(ref);
  }
  return out.slice(0, REPLAY_MAX_REFS);
}
