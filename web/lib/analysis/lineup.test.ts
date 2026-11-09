import test from 'node:test';
import assert from 'node:assert/strict';
import type { LibraryRosterEntry } from '../api/library.ts';
import type { TeamLineups } from '../api/team-insights.ts';
import { checkedPlayers, lineupFromChecked, lineupLabel, lineupSteamIds, toggleLineupPlayer } from './lineup.ts';

const player = (steamid64: string): LibraryRosterEntry => ({
  steamid64,
  nick: steamid64,
  first_played_at: '2026-01-01T00:00:00Z',
  last_played_at: '2026-11-01T00:00:00Z',
  matches: 3,
});

const LINEUPS: TeamLineups = {
  team_id: 't',
  team_name: 'FUT',
  latest_match: { match_id: 'm', map: 'de_nuke', played_at: '2026-11-01T00:00:00Z' },
  latest: [player('3'), player('1'), player('2')],
  others: [player('9')],
};

test('the latest lineup sends its players and every player sends nothing', () => {
  assert.deepEqual(lineupSteamIds({ kind: 'latest' }, LINEUPS), ['1', '2', '3']);
  assert.deepEqual(lineupSteamIds({ kind: 'latest' }, null), []);
  assert.deepEqual(lineupSteamIds({ kind: 'all' }, LINEUPS), []);
  assert.deepEqual(lineupSteamIds({ kind: 'players', steamIds: ['1', '9'] }, LINEUPS), ['1', '9']);
});

test('checking the stand-in turns the latest lineup into every player, and back', () => {
  assert.deepEqual([...checkedPlayers({ kind: 'all' }, LINEUPS)].sort(), ['1', '2', '3', '9']);
  const all = toggleLineupPlayer({ kind: 'latest' }, LINEUPS, '9');
  assert.deepEqual(all, { kind: 'all' });
  assert.deepEqual(toggleLineupPlayer(all, LINEUPS, '9'), { kind: 'latest' });
});

test('any other set is a chosen lineup, and the last box cannot be unchecked', () => {
  assert.deepEqual(toggleLineupPlayer({ kind: 'latest' }, LINEUPS, '3'), { kind: 'players', steamIds: ['1', '2'] });
  const one = { kind: 'players', steamIds: ['2'] } as const;
  assert.equal(toggleLineupPlayer(one, LINEUPS, '2'), one);
  assert.deepEqual(lineupFromChecked(new Set(['9', '1']), LINEUPS), { kind: 'players', steamIds: ['1', '9'] });
});

test('the button names the lineup', () => {
  assert.equal(lineupLabel({ kind: 'latest' }), 'Última alineación');
  assert.equal(lineupLabel({ kind: 'all' }), 'Todos los jugadores');
  assert.equal(lineupLabel({ kind: 'players', steamIds: ['1'] }), '1 jugador');
  assert.equal(lineupLabel({ kind: 'players', steamIds: ['1', '2', '9'] }), '3 jugadores');
});
