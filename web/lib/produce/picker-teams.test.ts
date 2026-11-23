import test from 'node:test';
import assert from 'node:assert/strict';
import type { LibraryMatch } from '../api/library.ts';
import type { RosterMatch } from '../api/types.ts';
import { pickerScore, pickerTeams } from './picker-teams.ts';

const ROSTER: RosterMatch = { map: 'de_mirage', scoreCt: 5, scoreT: 2, rounds: 7 };

function player(steamId: string, team: 'CT' | 'T' | '') {
  return { steamId, team };
}

function library(): LibraryMatch {
  return {
    id: 'm', demo_id: 'd', map: 'de_mirage', played_at: '', source: 'faceit', tickrate: 64, duration_ticks: 1, round_count: 7,
    teams: [
      { key: 'ct-start', start_side: 'CT', clan_name: 'Vitality', team_id: null, assignment: 'auto', score: 5 },
      { key: 't-start', start_side: 'T', clan_name: '', team_id: null, team_name: 'MOUZ', assignment: 'auto', score: 2 },
    ],
    players: [
      { steamid64: 'a', name: 'apEX', team_key: 'ct-start', slot: 0, rounds: 7, kills: 5, deaths: 2, assists: 0, damage: 400 },
      { steamid64: 'b', name: 'torzsi', team_key: 't-start', slot: 0, rounds: 7, kills: 3, deaths: 5, assists: 0, damage: 300 },
    ],
    created_at: '', updated_at: '',
  };
}

test('with a library match the picker groups by team, the team that started CT first, as the match record does', () => {
  const teams = pickerTeams({ players: [player('b', 'T'), player('a', 'CT')], match: ROSTER, library: library() });
  assert.deepEqual(teams.map((team) => [team.name, team.startedAs, team.score, team.roster.map((p) => p.steamId)]), [
    ['Vitality', 'Empezó de CT', 5, ['a']],
    ['MOUZ', 'Empezó de T', 2, ['b']],
  ]);
  assert.deepEqual(pickerScore({ match: ROSTER, library: library() })?.map((side) => side.score), [5, 2]);
});

test('without a library match, or with a player it does not know, the picker groups by the side each one ended on', () => {
  const players = [player('a', 'CT'), player('b', 'T'), player('c', 'T')];
  for (const source of [null, library()]) {
    const teams = pickerTeams({ players, match: ROSTER, library: source });
    assert.deepEqual(teams.map((team) => [team.side, team.name, team.score, team.roster.length]), [['T', null, 2, 2], ['CT', null, 5, 1]]);
  }
  assert.deepEqual(pickerScore({ match: ROSTER, library: null })?.map((side) => side.score), [2, 5]);
  assert.equal(pickerScore({ match: null, library: null }), null);
});
