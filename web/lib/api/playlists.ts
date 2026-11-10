import { jsonInit, readAll, request } from './library.ts';
import type { Stroke } from './spatial.ts';

/**
 * Browser-side contract of the saved round playlists (`library.Playlist` in
 * Go): rounds of any matches in play order, each with a comment.
 */

/** The server's caps (`library.MaxPlaylist*`). */
export const MAX_PLAYLIST_NAME = 120;
export const MAX_PLAYLIST_FOLDER = 60;
export const MAX_PLAYLIST_AUTHOR = 60;
export const MAX_PLAYLIST_COMMENT = 1000;
export const MAX_PLAYLIST_ROUNDS = 200;

/** A note and drawing on a playlist round, anchored to a tick as a match's note is; a new one has no id yet. */
export type PlaylistNote = { id?: string; tick: number; text: string; strokes: Stroke[] };

export type PlaylistRound = {
  match_id: string;
  round: number;
  map: string;
  demo_sha256?: string;
  comment: string;
  /** In tick order; an older server may leave it out. */
  notes?: PlaylistNote[];
  /** The match is not in this library; the round keeps its map and its demo's hash. */
  missing?: boolean;
};

export type SavedPlaylist = {
  id: string;
  name: string;
  folder: string;
  favorite: boolean;
  author: string;
  /** The rounds' maps, in round order. */
  maps: string[];
  rounds: PlaylistRound[];
  created_at: string;
  updated_at: string;
};

/** A round as the browser names it; the server reads its map and demo. */
export type PlaylistRoundInput = { match_id: string; round: number; comment?: string };

export type PlaylistInput = {
  name: string;
  folder?: string;
  author?: string;
  favorite?: boolean;
  rounds?: PlaylistRoundInput[];
};

/** `rounds` replaces the list in order; a star alone keeps the edit time. */
export type PlaylistPatch = Partial<PlaylistInput>;

/** One round of a playlist, as the round routes name it. */
export type PlaylistRoundTarget = { playlistId: string; matchId: string; round: number };

export const PLAYLIST_KEYS = [
  'id', 'name', 'folder', 'favorite', 'author', 'maps', 'rounds', 'created_at', 'updated_at',
] as const satisfies readonly (keyof SavedPlaylist)[];
export const PLAYLIST_FIELDS = ['name', 'folder', 'author', 'favorite', 'rounds'] as const satisfies readonly (keyof PlaylistInput)[];
export const PLAYLIST_LIST_QUERY = ['q', 'map', 'folder', 'no_folder', 'favorite'] as const;
export const PLAYLIST_ADD_FIELDS = ['rounds', 'position'] as const;
export const PLAYLIST_ROUND_FIELDS = ['comment', 'notes', 'position'] as const;

export const PLAYLIST_FILE_FORMAT = 'cliphub-playlists';
export const PLAYLIST_FILE_VERSION = 1;
export const PLAYLIST_FILE_KEYS = ['format', 'version', 'exported_at', 'playlists', 'matches'] as const;
export const PLAYLIST_IMPORT_KEYS = ['imported', 'duplicates', 'rejected', 'playlists', 'missing_matches'] as const;

/** A match a playlist file points at, so a library without it knows which demo to import. */
export type PlaylistFileMatch = {
  match_id: string;
  demo_sha256?: string;
  map: string;
  played_at?: string;
  teams: string[];
  file_name?: string;
};

export type PlaylistFile = {
  format: typeof PLAYLIST_FILE_FORMAT;
  version: number;
  exported_at: string;
  playlists: SavedPlaylist[];
  matches: PlaylistFileMatch[];
};

export type PlaylistImport = {
  imported: number;
  duplicates: number;
  rejected: { index: number; name: string; error: string }[];
  playlists: SavedPlaylist[];
  missing_matches: PlaylistFileMatch[];
};

function playlistPath(id: string): string {
  return `/playlists/${encodeURIComponent(id)}`;
}

function roundPath(target: PlaylistRoundTarget): string {
  return `${playlistPath(target.playlistId)}/rounds/${encodeURIComponent(target.matchId)}/${target.round}`;
}

/** Every saved playlist, by most recent edit. */
export function listPlaylists(): Promise<SavedPlaylist[]> {
  return readAll<SavedPlaylist>('/playlists', 'playlists');
}

export function getPlaylist(id: string): Promise<SavedPlaylist> {
  return request<SavedPlaylist>(playlistPath(id));
}

export function createPlaylist(input: PlaylistInput): Promise<SavedPlaylist> {
  return request<SavedPlaylist>('/playlists', jsonInit('POST', input));
}

export function patchPlaylist(id: string, patch: PlaylistPatch): Promise<SavedPlaylist> {
  return request<SavedPlaylist>(playlistPath(id), jsonInit('PATCH', patch));
}

export function deletePlaylist(id: string): Promise<void> {
  return request<void>(playlistPath(id), { method: 'DELETE' });
}

/** Adds rounds at `position` (0 is the start, the end without it); held rounds stay where they are. */
export function addPlaylistRounds(id: string, add: { rounds: PlaylistRoundInput[]; position?: number }): Promise<SavedPlaylist> {
  return request<SavedPlaylist>(`${playlistPath(id)}/rounds`, jsonInit('POST', add));
}

/** `notes` replaces the round's whole list. */
export function patchPlaylistRound(target: PlaylistRoundTarget, patch: { comment?: string; notes?: PlaylistNote[]; position?: number }): Promise<SavedPlaylist> {
  return request<SavedPlaylist>(roundPath(target), jsonInit('PATCH', patch));
}

/** Takes the round out and answers with the playlist left. */
export function removePlaylistRound(target: PlaylistRoundTarget): Promise<SavedPlaylist> {
  return request<SavedPlaylist>(roundPath(target), { method: 'DELETE' });
}

/** Ids one export names in its URL; a longer choice reads every playlist and keeps the chosen ones. */
const EXPORT_IDS_IN_URL = 100;

/** The playlist file of these playlists (every one when `ids` is empty), with the matches their rounds point at. */
export async function exportPlaylists(ids: readonly string[]): Promise<PlaylistFile> {
  const params = new URLSearchParams();
  if (ids.length <= EXPORT_IDS_IN_URL) for (const id of ids) params.append('id', id);
  const file = await request<PlaylistFile>(`/playlists/export?${params.toString()}`);
  if (ids.length <= EXPORT_IDS_IN_URL) return file;
  const chosen = new Set(ids);
  const playlists = file.playlists.filter((p) => chosen.has(p.id));
  const used = new Set(playlists.flatMap((p) => p.rounds.map((r) => r.match_id)));
  return { ...file, playlists, matches: file.matches.filter((m) => used.has(m.match_id)) };
}

/** Stores each playlist of a parsed file on its own; the server checks every field. */
export function importPlaylists(file: unknown): Promise<PlaylistImport> {
  return request<PlaylistImport>('/playlists/import', jsonInit('POST', file));
}
