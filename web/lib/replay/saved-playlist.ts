import { REPLAY_MAX_REFS } from '../analysis/replay-link.ts';
import type { ReplayRef } from '../analysis/replay-link.ts';
import type { PlaylistRound, PlaylistRoundInput, SavedPlaylist } from '../api/playlists.ts';

/** Where the Replay kept the rounds set aside with B before playlists were saved in the library. */
export const LEGACY_PLAYLIST_KEY = 'cliphub.replay.playlist';
/** The name the old playlist takes once it is saved, and the playlist B starts when there is none. */
export const DEFAULT_PLAYLIST_NAME = 'Mi playlist';
/** The playlist B adds to on this PC: the last one picked. */
export const TARGET_PLAYLIST_KEY = 'cliphub.replay.playlist-target';
/** The Replay query key of an open saved playlist. */
export const PLAYLIST_QUERY = 'p';
export const PLAYLISTS_HREF = '/replay/playlists';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function isPlaylistId(value: string | null | undefined): value is string {
  return typeof value === 'string' && UUID.test(value);
}

/** The Replay with a saved playlist open in its panel. */
export function savedPlaylistHref(id: string): string {
  return `/replay?${new URLSearchParams({ [PLAYLIST_QUERY]: id }).toString()}`;
}

function sameRound(a: { matchId: string; round: number }, b: { matchId: string; round: number }): boolean {
  return a.matchId === b.matchId && a.round === b.round;
}

export function holdsRound(playlist: Pick<SavedPlaylist, 'rounds'>, ref: ReplayRef): boolean {
  return playlist.rounds.some((r) => sameRound({ matchId: r.match_id, round: r.round }, ref));
}

/** The rounds the Replay can play, in order: a round whose match is gone is left out. */
export function playlistRefs(playlist: Pick<SavedPlaylist, 'rounds'>): ReplayRef[] {
  return playlist.rounds.filter((r) => r.missing !== true).map((r) => ({ matchId: r.match_id, round: r.round }));
}

export function playlistRound(playlist: Pick<SavedPlaylist, 'rounds'>, ref: ReplayRef): PlaylistRound | undefined {
  return playlist.rounds.find((r) => sameRound({ matchId: r.match_id, round: r.round }, ref));
}

/** A round as a playlist edit sends it back. */
export function roundInput(round: PlaylistRound): PlaylistRoundInput {
  return { match_id: round.match_id, round: round.round, comment: round.comment };
}

/** Reads the old stored playlist; anything that is not a round ref is dropped. */
export function parseLegacyPlaylist(raw: string | null): ReplayRef[] {
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
    if (!out.some((held) => sameRound(held, ref))) out.push(ref);
  }
  return out.slice(0, REPLAY_MAX_REFS);
}

/** The old playlist's rounds whose match is still in the library, ready to save. */
export function legacyRoundsToSave(refs: readonly ReplayRef[], roundCounts: ReadonlyMap<string, number>): PlaylistRoundInput[] {
  return refs
    .filter((ref) => ref.round <= (roundCounts.get(ref.matchId) ?? 0))
    .map((ref) => ({ match_id: ref.matchId, round: ref.round }));
}

/** The playlist B adds to: the one picked last, else the newest. */
export function targetPlaylist(list: readonly SavedPlaylist[], picked: string | null): SavedPlaylist | undefined {
  return list.find((p) => p.id === picked) ?? list[0];
}
