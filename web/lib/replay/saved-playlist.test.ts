import assert from 'node:assert/strict';
import { test } from 'node:test';
import { inPlaylist, parsePlaylist, togglePlaylistRound } from './saved-playlist.ts';

const MATCH = '82b2365b-3c63-48d4-8d9c-00342b71c5a1';

test('B adds the round once and a second B takes it out', () => {
  const added = togglePlaylistRound([], { matchId: MATCH, round: 4, tick: 900 });
  assert.deepEqual(added, [{ matchId: MATCH, round: 4 }]);
  assert.equal(inPlaylist(added, { matchId: MATCH, round: 4 }), true);
  assert.deepEqual(togglePlaylistRound(added, { matchId: MATCH, round: 4 }), []);
});

test('a stored playlist keeps only real round refs, once each', () => {
  const raw = JSON.stringify([{ matchId: MATCH, round: 2 }, { matchId: 'x', round: 1 }, { matchId: MATCH, round: 2 }, { matchId: MATCH, round: 0 }, 7]);
  assert.deepEqual(parsePlaylist(raw), [{ matchId: MATCH, round: 2 }]);
  assert.deepEqual(parsePlaylist('{'), []);
  assert.deepEqual(parsePlaylist(null), []);
});
