import { PLAYLIST_FILE_FORMAT, PLAYLIST_FILE_VERSION } from '../api/playlists.ts';
import type { PlaylistFile, PlaylistImport, SavedPlaylist } from '../api/playlists.ts';
import { countOf } from '../records/selection.ts';
import type { CountNoun } from '../records/selection.ts';

/** What the Playlists page lists: everything, the starred ones, the ones outside every folder, or one folder. */
export type PlaylistShelf = { kind: 'all' } | { kind: 'favorites' } | { kind: 'none' } | { kind: 'folder'; name: string };

export type PlaylistFolder = { name: string; count: number };

function folderKey(name: string): string {
  return name.toLocaleLowerCase('es');
}

/** The folders in use, by name; two spellings that differ only in case are one folder. */
export function playlistFolders(list: readonly SavedPlaylist[]): PlaylistFolder[] {
  const byKey = new Map<string, PlaylistFolder>();
  for (const p of list) {
    if (p.folder === '') continue;
    const held = byKey.get(folderKey(p.folder));
    if (held === undefined) byKey.set(folderKey(p.folder), { name: p.folder, count: 1 });
    else held.count++;
  }
  return [...byKey.values()].sort((a, b) => a.name.localeCompare(b.name, 'es', { sensitivity: 'base' }));
}

export function onShelf(p: SavedPlaylist, shelf: PlaylistShelf): boolean {
  switch (shelf.kind) {
    case 'all':
      return true;
    case 'favorites':
      return p.favorite;
    case 'none':
      return p.folder === '';
    case 'folder':
      return p.folder !== '' && folderKey(p.folder) === folderKey(shelf.name);
  }
}

export function playlistSearchText(p: SavedPlaylist): string {
  return [p.name, p.author, p.folder].join(' ');
}

export function missingRounds(p: SavedPlaylist): number {
  return p.rounds.filter((r) => r.missing === true).length;
}

export function playlistFileName(date: Date): string {
  return `playlists-${date.toISOString().slice(0, 10)}.json`;
}

/** A chosen file as it goes to the server, which checks every playlist in it. */
export type PlaylistUpload = Omit<PlaylistFile, 'playlists' | 'matches'> & { playlists: unknown[]; matches: unknown[] };

/** Reads a chosen file far enough to know it is a playlist file this Studio reads. */
export function parsePlaylistFile(text: string): { ok: true; file: PlaylistUpload } | { ok: false; error: string } {
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    return { ok: false, error: 'El fichero no es JSON. Elige un .json que exportó Studio.' };
  }
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    return { ok: false, error: 'El fichero no es una exportación de playlists de Studio.' };
  }
  const playlists: unknown = Reflect.get(value, 'playlists');
  const version: unknown = Reflect.get(value, 'version');
  if (Reflect.get(value, 'format') !== PLAYLIST_FILE_FORMAT || !Array.isArray(playlists)) {
    return { ok: false, error: 'El fichero no es una exportación de playlists de Studio.' };
  }
  if (version !== PLAYLIST_FILE_VERSION) {
    return { ok: false, error: `Este fichero es de la versión ${String(version)} y Studio lee la ${PLAYLIST_FILE_VERSION}. Actualiza Studio.` };
  }
  const exportedAt: unknown = Reflect.get(value, 'exported_at');
  const matches: unknown = Reflect.get(value, 'matches');
  return {
    ok: true,
    file: {
      format: PLAYLIST_FILE_FORMAT,
      version: PLAYLIST_FILE_VERSION,
      exported_at: typeof exportedAt === 'string' ? exportedAt : new Date(0).toISOString(),
      playlists,
      matches: Array.isArray(matches) ? matches : [],
    },
  };
}

const IMPORTED: CountNoun = { one: 'playlist importada', many: 'playlists importadas', feminine: true };
const HELD: CountNoun = { one: 'ya estaba', many: 'ya estaban', feminine: true };

/** What an import did, in one or two sentences, naming the demos still to import. */
export function playlistImportMessage(result: PlaylistImport): string {
  const parts: string[] = [];
  if (result.imported > 0) parts.push(countOf(result.imported, IMPORTED));
  if (result.duplicates > 0) parts.push(`${countOf(result.duplicates, HELD)} en Studio`);
  if (result.rejected.length > 0) parts.push(`${result.rejected.length} no se ${result.rejected.length === 1 ? 'pudo' : 'pudieron'} leer`);
  const head = parts.length === 0 ? 'El fichero no traía playlists.' : `${parts.join(', ')}.`;
  const missing = result.missing_matches;
  if (missing.length === 0) return head;
  const names = missing.slice(0, 3).map((m) => m.file_name || m.teams.join(' vs ') || m.map);
  const more = missing.length > names.length ? ` y ${missing.length - names.length} más` : '';
  const lacking = missing.length === 1 ? 'Falta 1 demo' : `Faltan ${missing.length} demos`;
  return `${head} ${lacking} para ver todas sus rondas: ${names.join(', ')}${more}. Impórtalas en Partidas y las rondas aparecerán solas.`;
}
