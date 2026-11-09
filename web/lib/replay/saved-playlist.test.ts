import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { SavedPlaylist } from '../api/playlists.ts';
import { holdsRound, legacyRoundsToSave, parseLegacyPlaylist, playlistRefs, savedPlaylistHref, targetPlaylist } from './saved-playlist.ts';

const MATCH = '82b2365b-3c63-48d4-8d9c-00342b71c5a1';
const GONE = '0b9c1f9e-0f4d-4c7a-9a59-2f4f6c1d2e3a';

function playlist(id: string, rounds: SavedPlaylist['rounds']): SavedPlaylist {
  return { id, name: id, folder: '', favorite: false, author: '', maps: [], rounds, created_at: '', updated_at: '' };
}

test('the old stored playlist keeps only real round refs, once each', () => {
  const raw = JSON.stringify([{ matchId: MATCH, round: 2 }, { matchId: 'x', round: 1 }, { matchId: MATCH, round: 2 }, { matchId: MATCH, round: 0 }, 7]);
  assert.deepEqual(parseLegacyPlaylist(raw), [{ matchId: MATCH, round: 2 }]);
  assert.deepEqual(parseLegacyPlaylist('{'), []);
  assert.deepEqual(parseLegacyPlaylist(null), []);
});

test('saving the old playlist drops rounds whose match left the library or never had them', () => {
  const refs = [{ matchId: MATCH, round: 2 }, { matchId: GONE, round: 1 }, { matchId: MATCH, round: 31 }];
  assert.deepEqual(legacyRoundsToSave(refs, new Map([[MATCH, 30]])), [{ match_id: MATCH, round: 2 }]);
});

test('the Replay plays the rounds it can find, and B knows which it holds', () => {
  const p = playlist('a', [
    { match_id: MATCH, round: 4, map: 'de_mirage', comment: 'humo' },
    { match_id: GONE, round: 1, map: 'de_nuke', comment: '', missing: true },
  ]);
  assert.deepEqual(playlistRefs(p), [{ matchId: MATCH, round: 4 }]);
  assert.equal(holdsRound(p, { matchId: MATCH, round: 4, tick: 9 }), true);
  assert.equal(holdsRound(p, { matchId: MATCH, round: 5 }), false);
});

test('B adds to the playlist picked last, else the newest', () => {
  const list = [playlist('new', []), playlist('old', [])];
  assert.equal(targetPlaylist(list, 'old')?.id, 'old');
  assert.equal(targetPlaylist(list, 'deleted')?.id, 'new');
  assert.equal(targetPlaylist([], null), undefined);
  assert.equal(savedPlaylistHref(MATCH), `/replay?p=${MATCH}`);
});
