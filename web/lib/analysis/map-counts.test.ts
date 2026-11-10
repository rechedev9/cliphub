import test from 'node:test';
import assert from 'node:assert/strict';
import { DEFAULT_SCOPE, type AnalysisScope } from './scope.ts';
import { mapCountText, mapCountsOf, mapCountsQuery } from './map-counts.ts';

const NOW = Date.parse('2026-11-10T12:00:00Z');
const RIVAL = '0b7c4a1e-6f0d-4a51-9d55-3f1f3a9c2b10';
const SCOPE: AnalysisScope = {
  ...DEFAULT_SCOPE,
  map: 'de_nuke',
  side: 'CT',
  period: { kind: 'preset', preset: '30d' },
  opponent: RIVAL,
  limit: 10,
  buy: 'full',
  whose: 'own',
};

test('the counts query carries the lineup, period, rival and match filters the view reads, never map or side', () => {
  const query = mapCountsQuery({ scope: SCOPE, fields: ['lineup', 'map', 'side', 'period', 'opponent', 'limit', 'buy', 'whose'], lineup: ['1', '2'], now: NOW });
  assert.equal(query.from, '2026-10-11T12:00:00.000Z');
  assert.deepEqual(query.opponent_ids, [RIVAL]);
  assert.deepEqual(query.lineup, ['1', '2']);
  assert.deepEqual(query.criteria, [['max_matches', '10'], ['whose', 'own']]);
  assert.equal(query.maps, undefined);
  assert.equal(query.side, undefined);
});

test('a filter the view does not read does not narrow its counts', () => {
  const query = mapCountsQuery({ scope: SCOPE, fields: ['map'], lineup: ['1'], now: NOW });
  assert.equal(query.from, undefined);
  assert.deepEqual(query.opponent_ids, []);
  assert.deepEqual(query.lineup, []);
  assert.deepEqual(query.criteria, []);
});

test('counts read by map name and the text counts one match in the singular', () => {
  assert.deepEqual(mapCountsOf({ team_id: 't', team_name: 'FUT', maps: [{ map: 'de_nuke', matches: 3 }, { map: 'de_mirage', matches: 0 }] }), {
    de_nuke: 3,
    de_mirage: 0,
  });
  assert.equal(mapCountText(1), '1 partida');
  assert.equal(mapCountText(0), '0 partidas');
});
