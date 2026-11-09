import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { SavedPlaylist } from '../api/playlists.ts';
import { onShelf, parsePlaylistFile, playlistFolders, playlistImportMessage } from './playlist-library.ts';

function playlist(name: string, folder: string, favorite = false): SavedPlaylist {
  return { id: name, name, folder, favorite, author: '', maps: [], rounds: [], created_at: '', updated_at: '' };
}

test('folders group spellings that differ in case and count their playlists', () => {
  const list = [playlist('a', 'Rivales'), playlist('b', 'rivales'), playlist('c', ''), playlist('d', 'Ánimo')];
  assert.deepEqual(playlistFolders(list), [{ name: 'Ánimo', count: 1 }, { name: 'Rivales', count: 2 }]);
  assert.equal(onShelf(list[1], { kind: 'folder', name: 'RIVALES' }), true);
  assert.equal(onShelf(list[2], { kind: 'none' }), true);
  assert.equal(onShelf(list[0], { kind: 'none' }), false);
  assert.equal(onShelf(playlist('e', '', true), { kind: 'favorites' }), true);
});

test('only a playlist file this Studio reads goes to the server', () => {
  assert.equal(parsePlaylistFile('{').ok, false);
  assert.equal(parsePlaylistFile('{"format":"cliphub-stratbook","version":1,"plays":[]}').ok, false);
  const newer = parsePlaylistFile('{"format":"cliphub-playlists","version":2,"playlists":[]}');
  assert.equal(newer.ok === false && newer.error.includes('Actualiza Studio'), true);
  const read = parsePlaylistFile('{"format":"cliphub-playlists","version":1,"playlists":[{"name":"x"}]}');
  assert.equal(read.ok && read.file.playlists.length === 1 && read.file.matches.length === 0, true);
});

test('an import names the demos still missing', () => {
  const message = playlistImportMessage({
    imported: 2,
    duplicates: 1,
    rejected: [],
    playlists: [],
    missing_matches: [{ match_id: 'm', map: 'de_nuke', teams: ['FUT', 'OG'], file_name: 'fut-og.dem' }],
  });
  assert.equal(message, '2 playlists importadas, 1 ya estaba en Studio. Falta 1 demo para ver todas sus rondas: fut-og.dem. Impórtalas en Partidas y las rondas aparecerán solas.');
});
