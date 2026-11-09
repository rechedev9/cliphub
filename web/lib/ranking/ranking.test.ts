import test from 'node:test';
import assert from 'node:assert/strict';
import type { LeaderboardRow } from '../api/baseline.ts';
import {
  DEFAULT_MIN_ROUNDS,
  RANKING_FETCH_LIMIT,
  leaderboardRequest,
  parseRanking,
  parsePins,
  patchRanking,
  playerRoundsHref,
  rankingEntries,
  sortRankingRows,
  togglePin,
  writeRanking,
  type RankingState,
} from './ranking.ts';

const NOW = Date.parse('2026-11-09T12:00:00Z');
const MATCH = '0b7c4a1e-6f0d-4a51-9d55-3f1f3a9c2b10';

function row(steamid64: string, rating: number, rounds: number, dpr = 0.7): LeaderboardRow {
  return { rank: 0, steamid64, name: `p${steamid64}`, rounds, matches: 3, value: rating, metrics: { rating, dpr } };
}

const parse = (query: string): RankingState => parseRanking(new URLSearchParams(query));

test('an empty query is the default view', () => {
  const state = parse('');
  assert.equal(state.sort, 'rating');
  assert.equal(state.minRounds, DEFAULT_MIN_ROUNDS);
  assert.equal(state.role, '');
  assert.equal(writeRanking(state), '');
});

test('the view survives a round trip through the URL', () => {
  const query = 'map=de_anubis&period=90d&role=b_lurk&nivel=hltv&min=50&orden=adr&sentido=asc&other=1';
  const state = parse(query);
  assert.equal(state.scope.side, 'T');
  assert.deepEqual(parse(writeRanking(state, 'other=1')), state);
  assert.match(writeRanking(state, 'other=1'), /other=1/);
});

test('a role needs a map and fixes the side', () => {
  assert.equal(parse('role=b_lurk').role, '');
  assert.equal(parse('map=de_anubis&role=b_anchor&side=T').scope.side, 'CT');
  assert.equal(parse('map=de_anubis&role=nope').role, '');
});

test('choosing the other side or clearing the map drops the role', () => {
  const state = parse('map=de_anubis&role=b_lurk');
  assert.equal(patchRanking(state, { scope: { side: 'CT' } }).role, '');
  assert.equal(patchRanking(state, { scope: { side: 'T' } }).role, 'b_lurk');
  assert.equal(patchRanking(state, { scope: { map: '' } }).role, '');
  assert.equal(patchRanking(state, { role: 'rotator' }).scope.side, 'CT');
});

test('the library request carries every filter and the pro request only the map', () => {
  const state = parse('map=de_anubis&period=90d&role=b_lurk&nivel=hltv&min=50&orden=adr');
  const library = leaderboardRequest(state, NOW);
  assert.deepEqual(
    { metric: library.metric, map: library.map, role: library.role, side: library.side, source: library.source, min: library.min_rounds, limit: library.limit },
    { metric: 'adr', map: 'de_anubis', role: 'b_lurk', side: 'T', source: 'hltv', min: 50, limit: RANKING_FETCH_LIMIT },
  );
  assert.equal(library.from, new Date(NOW - 90 * 24 * 3600 * 1000).toISOString());
});

test('the pro request names a period and a rival level, and never the library filters', () => {
  const pro = leaderboardRequest(parse('map=de_anubis&ref=pro&role=b_lurk&nivel=hltv&period=30d&rival=30'), NOW);
  assert.deepEqual(
    [pro.scope, pro.map, pro.role, pro.side, pro.days, pro.top, pro.source, pro.from],
    ['pro', 'de_anubis', 'b_lurk', 'T', 30, 30, undefined, undefined],
  );
  assert.equal(leaderboardRequest(parse('ref=pro'), NOW).days, 365);
  assert.equal(leaderboardRequest(parse('ref=pro&rival=7'), NOW).top, undefined);
  assert.equal(parse('rival=30').top, null);
  assert.equal(parse('ref=pro&rival=30').top, 30);
  assert.equal(parse(writeRanking(parse('ref=pro&rival=15&map=de_anubis'))).top, 15);
});

test('sorting puts the best first, deaths per round lowest first, and rows without the value last', () => {
  const rows = [row('1', 1.0, 100, 0.8), row('2', 1.3, 60, 0.6), { ...row('3', 1.1, 80), metrics: { rating: 1.1 } }];
  assert.deepEqual(sortRankingRows(rows, { sort: 'rating', dir: null }, 'rating').map((r) => r.steamid64), ['2', '3', '1']);
  assert.deepEqual(sortRankingRows(rows, { sort: 'rating', dir: 'asc' }, 'rating').map((r) => r.steamid64), ['1', '3', '2']);
  assert.deepEqual(sortRankingRows(rows, { sort: 'dpr', dir: null }, 'rating').map((r) => r.steamid64), ['2', '1', '3']);
  assert.deepEqual(sortRankingRows(rows, { sort: 'rounds', dir: null }, 'rating').map((r) => r.steamid64), ['1', '3', '2']);
});

test('pinned players stay on top in table order, even when the filter leaves them out', () => {
  const sorted = [row('1', 1.4, 90), row('2', 1.2, 90), row('3', 1.0, 90)];
  const pins = [
    { steamid64: '3', name: 'three' },
    { steamid64: '9', name: 'gone' },
  ];
  const entries = rankingEntries(sorted, pins);
  assert.deepEqual(
    entries.map((e) => [e.row?.steamid64 ?? null, e.pinned?.steamid64 ?? null, e.place]),
    [['3', '3', 3], [null, '9', null], ['1', null, 1], ['2', null, 2]],
  );
});

test('pins toggle and a corrupt store reads as none', () => {
  const pin = { steamid64: '5', name: 'five' };
  assert.deepEqual(togglePin(togglePin([], pin), pin), []);
  assert.deepEqual(parsePins('not json'), []);
  assert.deepEqual(parsePins(JSON.stringify([{ steamid64: '5', name: 'five' }, { steamid64: 'x', name: 'bad' }, 3])), [pin]);
});

test('a row opens its rounds in the Replay', () => {
  const withRounds = { ...row('1', 1, 10), examples: [{ match_id: MATCH, round: 4, tick: 900 }] };
  assert.equal(playerRoundsHref(withRounds, 'p1')?.startsWith(`/replay?r=${MATCH}%3A4%3A900`), true);
  assert.equal(playerRoundsHref(row('2', 1, 10), 'p2'), null);
});
