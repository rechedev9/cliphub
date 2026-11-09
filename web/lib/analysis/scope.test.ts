// The URL is the scope's storage: a reload or a pasted link must restore the
// same filter, and a hand-edited value must never break the rest of it.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  DEFAULT_SCOPE,
  inScopePeriod,
  lineupOf,
  parseAnalysisScope,
  resetScopeFields,
  scopeCountText,
  scopeDateRange,
  scopeBuyTypes,
  scopeFieldActive,
  writeAnalysisScope,
  type AnalysisScope,
} from './scope.ts';
import { teamScopeEntries } from './team-scope.ts';

const TEAM = '0b7c4a1e-6f0d-4a51-9d55-3f1f3a9c2b10';
const NOW = Date.parse('2026-11-05T12:00:00Z');
const ONE = '76561198000000001';
const TWO = '76561198000000002';

test('an empty query is the default scope', () => {
  assert.deepEqual(parseAnalysisScope(new URLSearchParams('')), DEFAULT_SCOPE);
});

test('a full scope survives a round trip through the URL', () => {
  const scope: AnalysisScope = {
    map: 'de_mirage',
    side: 'CT',
    period: { kind: 'range', from: '2026-09-01', to: '2026-10-31' },
    limit: 20,
    buy: 'full',
    opponentBuy: 'eco',
    opponent: TEAM,
    reference: 'pro',
    lineup: { kind: 'players', steamIds: [ONE, TWO] },
    awp: 'with',
    opponentAwp: 'without',
    advantage: '4v5',
    rounds: { from: 13, to: 24 },
    opponentTop: 30,
    whose: 'others',
  };
  assert.deepEqual(parseAnalysisScope(new URLSearchParams(writeAnalysisScope(scope))), scope);
  const half: AnalysisScope = { ...DEFAULT_SCOPE, buy: 'half', period: { kind: 'preset', preset: '14d' }, limit: 3 };
  assert.deepEqual(parseAnalysisScope(new URLSearchParams(writeAnalysisScope(half))), half);
  const preset: AnalysisScope = { ...DEFAULT_SCOPE, period: { kind: 'preset', preset: '90d' } };
  assert.deepEqual(parseAnalysisScope(new URLSearchParams(writeAnalysisScope(preset))), preset);
});

test('writing keeps parameters the scope does not own and drops defaults', () => {
  const query = writeAnalysisScope({ ...DEFAULT_SCOPE, side: 'T' }, 'tab=radar&map=de_nuke&period=30d');
  assert.equal(query, 'tab=radar&side=T');
});

test('values the URL cannot hold are dropped one by one', () => {
  const scope = parseAnalysisScope(
    new URLSearchParams('map=Mirage!&side=ct&limit=500&buy=unknown&opponent_buy=force&opponent=nope&from=2026-02-30'),
  );
  assert.deepEqual(scope, { ...DEFAULT_SCOPE, side: 'CT', opponentBuy: 'force' });
});

test('the reference is the library unless the URL says pro', () => {
  assert.equal(parseAnalysisScope(new URLSearchParams('ref=pro')).reference, 'pro');
  assert.equal(parseAnalysisScope(new URLSearchParams('ref=hltv')).reference, 'library');
  assert.equal(writeAnalysisScope({ ...DEFAULT_SCOPE, reference: 'library' }, 'ref=pro'), '');
});

test('the lineup is the latest one unless the URL says all or names valid players', () => {
  const read = (query: string): AnalysisScope['lineup'] => parseAnalysisScope(new URLSearchParams(query)).lineup;
  assert.deepEqual(read(''), { kind: 'latest' });
  assert.deepEqual(read('lineup=all'), { kind: 'all' });
  assert.deepEqual(read(`lineup=${TWO},${ONE},${TWO}`), { kind: 'players', steamIds: [ONE, TWO] });
  assert.deepEqual(read(`lineup=${ONE},zywoo`), { kind: 'latest' });
  assert.deepEqual(read(`lineup=${Array.from({ length: 51 }, () => ONE).join(',')}`), { kind: 'latest' });
  assert.equal(writeAnalysisScope({ ...DEFAULT_SCOPE, lineup: { kind: 'all' } }, 'tab=preparar'), 'tab=preparar&lineup=all');
  assert.equal(writeAnalysisScope(DEFAULT_SCOPE, 'lineup=all'), '');
  assert.deepEqual(lineupOf([TWO, ONE, ONE]), { kind: 'players', steamIds: [ONE, TWO] });
});

test('a lineup is active until it is back at the latest one', () => {
  const scope: AnalysisScope = { ...DEFAULT_SCOPE, lineup: { kind: 'all' } };
  assert.equal(scopeFieldActive(scope, 'lineup'), true);
  assert.equal(scopeFieldActive(resetScopeFields(scope, ['lineup']), 'lineup'), false);
});

test('a reversed date range is read in order', () => {
  const scope = parseAnalysisScope(new URLSearchParams('from=2026-10-31&to=2026-09-01'));
  assert.deepEqual(scope.period, { kind: 'range', from: '2026-09-01', to: '2026-10-31' });
});

test('a preset wins over exact days', () => {
  const scope = parseAnalysisScope(new URLSearchParams('period=30d&from=2026-01-01'));
  assert.deepEqual(scope.period, { kind: 'preset', preset: '30d' });
});

test('the period becomes inclusive RFC 3339 bounds', () => {
  assert.deepEqual(scopeDateRange({ kind: 'all' }, NOW), {});
  assert.deepEqual(scopeDateRange({ kind: 'preset', preset: '30d' }, NOW), { from: '2026-10-06T12:00:00.000Z' });
  assert.deepEqual(scopeDateRange({ kind: 'range', from: '', to: '2026-10-31' }, NOW), { to: '2026-10-31T23:59:59Z' });
});

test('a match is in the period when its date falls between the bounds', () => {
  const october: AnalysisScope['period'] = { kind: 'range', from: '2026-10-01', to: '2026-10-31' };
  assert.equal(inScopePeriod('2026-10-31T22:00:00Z', october, NOW), true);
  assert.equal(inScopePeriod('2026-11-01T00:00:00Z', october, NOW), false);
  assert.equal(inScopePeriod('not a date', october, NOW), false);
  assert.equal(inScopePeriod('not a date', { kind: 'all' }, NOW), true);
});

test('reset puts only the named fields back', () => {
  const scope: AnalysisScope = { ...DEFAULT_SCOPE, map: 'de_inferno', side: 'T', period: { kind: 'preset', preset: '30d' } };
  const next = resetScopeFields(scope, ['side', 'period']);
  assert.deepEqual(next, { ...DEFAULT_SCOPE, map: 'de_inferno' });
  assert.equal(scopeFieldActive(next, 'map'), true);
  assert.equal(scopeFieldActive(next, 'period'), false);
});

test('the counter names its units in the singular and the plural', () => {
  assert.equal(scopeCountText(1, 1), '1 mapa · 1 ronda');
  assert.equal(scopeCountText(12, 286), '12 mapas · 286 rondas');
});

test('the round filters drop what the server would refuse', () => {
  const scope = parseAnalysisScope(
    new URLSearchParams('awp=yes&opponent_awp=with&advantage=6v5&round_from=0&round_to=100&opponent_top=10&whose=mine'),
  );
  assert.deepEqual(scope, { ...DEFAULT_SCOPE, opponentAwp: 'with' });
  assert.deepEqual(parseAnalysisScope(new URLSearchParams('round_from=24&round_to=13')).rounds, { from: 13, to: 24 });
  assert.deepEqual(parseAnalysisScope(new URLSearchParams('round_to=12')).rounds, { from: null, to: 12 });
});

test('a round range is active while either end is set', () => {
  const scope: AnalysisScope = { ...DEFAULT_SCOPE, rounds: { from: null, to: 12 } };
  assert.equal(scopeFieldActive(scope, 'rounds'), true);
  assert.equal(scopeFieldActive(resetScopeFields(scope, ['rounds']), 'rounds'), false);
});

test('a half buy stands for semi and force', () => {
  assert.deepEqual(scopeBuyTypes('half'), ['semi', 'force']);
  assert.deepEqual(scopeBuyTypes('full'), ['full']);
  assert.deepEqual(scopeBuyTypes(''), []);
});

test('a team view sends only the scope fields it reads, with the limit as max_matches', () => {
  const scope: AnalysisScope = {
    ...DEFAULT_SCOPE,
    limit: 5,
    buy: 'half',
    opponentBuy: 'full',
    awp: 'with',
    advantage: '5v4',
    rounds: { from: 1, to: 12 },
    opponentTop: 15,
    whose: 'own',
  };
  assert.deepEqual(teamScopeEntries(scope, ['limit', 'buy', 'opponentBuy', 'awp', 'advantage', 'rounds', 'opponentTop', 'whose']), [
    ['max_matches', '5'],
    ['buy', 'semi'],
    ['buy', 'force'],
    ['opponent_buy', 'full'],
    ['awp', 'with'],
    ['advantage', '5v4'],
    ['round_from', '1'],
    ['round_to', '12'],
    ['opponent_top', '15'],
    ['whose', 'own'],
  ]);
  assert.deepEqual(teamScopeEntries(scope, ['side', 'awp']), [['awp', 'with']]);
});
