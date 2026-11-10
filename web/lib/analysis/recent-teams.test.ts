import test from 'node:test';
import assert from 'node:assert/strict';
import { parseRecentTeams, recentTeamOptions, withRecentTeam } from './recent-teams.ts';

const KNOWN = ['a', 'b', 'c', 'd', 'e', 'f'];

test('a team opened again moves to the front and the list keeps the open team and four more', () => {
  assert.deepEqual(withRecentTeam({ list: ['a', 'b', 'c', 'd'], id: 'c', known: KNOWN }), ['c', 'a', 'b', 'd']);
  assert.deepEqual(withRecentTeam({ list: ['a', 'b', 'c', 'd', 'e'], id: 'f', known: KNOWN }), ['f', 'a', 'b', 'c', 'd']);
});

test('teams that no longer exist drop out before the list is cut', () => {
  assert.deepEqual(withRecentTeam({ list: ['gone', 'a', 'b', 'c', 'd', 'e'], id: 'f', known: KNOWN }), ['f', 'a', 'b', 'c', 'd']);
});

test('stored text that is not a list of ids reads as no recents', () => {
  assert.deepEqual(parseRecentTeams(null), []);
  assert.deepEqual(parseRecentTeams('not json'), []);
  assert.deepEqual(parseRecentTeams('{"a":1}'), []);
  assert.deepEqual(parseRecentTeams('["a",3,"","a","b"]'), ['a', 'b']);
});

test('Recientes lists up to four teams other than the open one', () => {
  const options = KNOWN.map((value) => ({ value, label: value }));
  assert.deepEqual(
    recentTeamOptions(['f', 'a', 'gone', 'b', 'c', 'd'], options, 'f').map((option) => option.value),
    ['a', 'b', 'c', 'd'],
  );
});
