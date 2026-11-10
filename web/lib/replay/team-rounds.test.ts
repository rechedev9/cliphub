import test from 'node:test';
import assert from 'node:assert/strict';
import { DEFAULT_SCOPE } from '../analysis/scope.ts';
import type { TeamRoundsMatch } from '../api/team-insights.ts';
import { teamRoundRefs, teamRoundsQuery } from './team-rounds.ts';

const NOW = Date.parse('2026-11-10T12:00:00Z');

function match(id: string, rounds: number[]): TeamRoundsMatch {
  return {
    match_id: id,
    map: 'de_nuke',
    played_at: '2026-11-01T00:00:00Z',
    team_key: 'ct-start',
    rounds: rounds.map((round) => ({ round, side: 'T', winner: 'T', buy: 'full', opponent_buy: 'full', site: 'A' })),
    total: rounds.length,
  };
}

test('team rounds: the query reads the scope the way the team Replay tab does', () => {
  const scope = { ...DEFAULT_SCOPE, map: 'de_nuke', side: 'T' as const, limit: 5, buy: 'full' as const, opponent: 'rival-1' };
  const query = teamRoundsQuery({ scope, lineup: ['7656'], now: NOW });
  assert.deepEqual(query.maps, ['de_nuke']);
  assert.equal(query.side, 'T');
  assert.deepEqual(query.opponent_ids, ['rival-1']);
  assert.deepEqual(query.lineup, ['7656']);
  assert.deepEqual(query.criteria?.filter(([key]) => key === 'max_matches'), [['max_matches', '5']]);
  assert.ok(query.criteria?.some(([, value]) => value === 'full'));
});

test('team rounds: with no map the query asks for every map', () => {
  assert.deepEqual(teamRoundsQuery({ scope: DEFAULT_SCOPE, lineup: [], now: NOW }).maps, []);
});

test('team rounds: refs keep the newest match first and each match in round order', () => {
  const answer = { matches: [match('m-new', [3, 1, 2]), match('m-empty', []), match('m-old', [5])] };
  const { refs, total, matchIds } = teamRoundRefs(answer);
  assert.deepEqual(refs.map((ref) => `${ref.matchId}:${ref.round}`), ['m-new:1', 'm-new:2', 'm-new:3', 'm-old:5']);
  assert.equal(total, 4);
  assert.deepEqual(matchIds, ['m-new', 'm-old']);
});

test('team rounds: the cap cuts the list but the total still counts every round', () => {
  const { refs, total } = teamRoundRefs({ matches: [match('m-new', [1, 2, 3]), match('m-old', [1, 2])] }, 4);
  assert.equal(refs.length, 4);
  assert.equal(total, 5);
});
