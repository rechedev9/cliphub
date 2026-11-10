import test from 'node:test';
import assert from 'node:assert/strict';
import { RECENT_TEAMS_MAX, parseRecentTeams, recentTeamOptions, withRecentTeam } from './recent-teams.ts';

test('a team opened again moves to the front and the list keeps the last four', () => {
  assert.deepEqual(withRecentTeam(['a', 'b', 'c', 'd'], 'c'), ['c', 'a', 'b', 'd']);
  assert.deepEqual(withRecentTeam(['a', 'b', 'c', 'd'], 'e'), ['e', 'a', 'b', 'c']);
  assert.equal(withRecentTeam([], 'a').length, 1);
  assert.equal(withRecentTeam(['a', 'b', 'c', 'd', 'e', 'f'], 'g').length, RECENT_TEAMS_MAX);
});

test('stored text that is not a list of ids reads as no recents', () => {
  assert.deepEqual(parseRecentTeams(null), []);
  assert.deepEqual(parseRecentTeams('not json'), []);
  assert.deepEqual(parseRecentTeams('{"a":1}'), []);
  assert.deepEqual(parseRecentTeams('["a",3,"","a","b"]'), ['a', 'b']);
});

test('recents drop the teams the library no longer has', () => {
  const options = [{ value: 'a', label: 'A' }, { value: 'b', label: 'B' }];
  assert.deepEqual(recentTeamOptions(['b', 'gone', 'a'], options), [options[1], options[0]]);
});
