// A small library must still get a reference, and a team table must read the
// library's pooled rate, not the team's own mean.
import test from 'node:test';
import assert from 'node:assert/strict';
import { RatePool, adaptiveMinRounds, bombsiteKey, categoryKey, libraryMatchesText, normsCategories, poolBombsites, poolCategories, poolMatchups, significantStats } from './library-pool.ts';
import { teamTrend } from './team-trend.ts';
import { gappedPoints, segmentsOf } from './chart.ts';
import { prepStatInputs } from './prep-stats.ts';
import type { TeamPlaybook, TeamPrep } from '../api/playbook.ts';
import type { BombsiteMap } from '../api/team-insights.ts';
import type { LibraryMatch } from '../api/library.ts';
import type { LibraryNorms } from '../api/norms.ts';

const rate = (count: number, total: number) => ({ count, total, pct: total > 0 ? (100 * count) / total : 0, reliable: true });

test('the minimum follows the median player and stays between 15 and 50', () => {
  assert.equal(adaptiveMinRounds([]), 50);
  assert.equal(adaptiveMinRounds([24, 24, 48, 30, 0]), 24);
  assert.equal(adaptiveMinRounds([5, 8, 10]), 15);
  assert.equal(adaptiveMinRounds([200, 300, 120]), 50);
});

test('a pool sums counts and totals before the percent', () => {
  const pool = new RatePool();
  pool.add('a', rate(1, 4));
  pool.add('a', rate(5, 6));
  assert.equal(pool.pct('a'), 60);
  assert.equal(pool.pct('a', 11), null);
  assert.equal(pool.pct('missing'), null);
});

test('categories, matchups and bombsites pool their rows', () => {
  const playbook = (used: number, won: number) =>
    ({ win_rate: rate(won, 10), categories: [{ category: 'execute', share: rate(used, 10), win_rate: rate(won, used) }] }) as unknown as TeamPlaybook;
  const categories = poolCategories([playbook(2, 1), playbook(6, 3)]);
  assert.equal(categories.pct(categoryKey('used', 'execute')), 40);
  assert.equal(categories.pct(categoryKey('won', 'execute')), 50);

  const prep = { matchups: [{ side: 'CT', buy: 'full', opponent_buy: 'eco', win_rate: rate(9, 10) }, { side: 'CT', buy: 'unknown', opponent_buy: 'eco', win_rate: rate(0, 5) }] } as unknown as TeamPrep;
  const matchups = poolMatchups([prep]);
  assert.equal(matchups.pct('CT|full|eco'), 90);
  assert.equal(matchups.pct('CT'), 90);
  assert.equal(matchups.pct('buy|full'), 90);

  const defense = { hit: rate(5, 10), held: rate(3, 5), plants_allowed: rate(2, 5), won: rate(3, 5), retakes_attempted: rate(2, 2), retakes_won: rate(1, 2), examples: [] };
  const attack = { attacks: rate(4, 10), plants: rate(2, 4), won: rate(1, 4), post_plant_won: rate(1, 2), examples: [] };
  const entry = { map: 'de_mirage', sites: [{ site: 'A', ct: defense, t: attack }] } as unknown as BombsiteMap;
  const sites = poolBombsites([entry, entry]);
  assert.equal(sites.pct(bombsiteKey('de_mirage', 'A', 'ct.held')), 60);
  assert.equal(sites.total(bombsiteKey('de_mirage', 'A', 't.attacks')), 20);
});

test('the library norms read through the same keys as a team', () => {
  const norms: LibraryNorms = {
    map: 'de_mirage',
    matches: 3,
    skipped: 0,
    bombsites: [],
    matchups: [{ side: 'T', buy: 'full', opponent_buy: 'full', win_rate: rate(30, 60) }],
    playbook: [
      { side: 'T', round_count: 60, win_rate: rate(30, 60), categories: [{ category: 'rush', share: rate(12, 60), win_rate: rate(3, 12) }] },
      { side: 'CT', round_count: 60, win_rate: rate(30, 60), categories: [{ category: 'stack', share: rate(6, 60), win_rate: rate(4, 6) }] },
    ],
  };
  const t = normsCategories(norms, 'T');
  assert.equal(t.pct(categoryKey('used', 'rush')), 20);
  assert.equal(t.pct(categoryKey('won', 'rush')), 25);
  assert.equal(t.pct(categoryKey('used', 'stack')), null);
  assert.equal(normsCategories(norms, 'CT').pct(categoryKey('won', 'stack')), (100 * 4) / 6);
  assert.equal(poolMatchups([norms]).pct('T|full|full'), 50);
  assert.equal(libraryMatchesText(1), 'la partida de la biblioteca');
  assert.equal(libraryMatchesText(3), 'las 3 partidas de la biblioteca');
});

test('significant stats name the gap in points, worse in its own tone, largest first', () => {
  const pct = (v: number) => `${Math.round(v)} %`;
  const stats = significantStats([
    { id: 'open', subject: 'La apertura es', value: 39, reference: 45, format: pct, percent: true, higherIsBetter: true, threshold: 5, n: 40, minSample: 10 },
    { id: 'small', subject: 'El retake es', value: 10, reference: 50, format: pct, percent: true, higherIsBetter: true, threshold: 5, n: 3, minSample: 10 },
    { id: 'even', subject: 'El ADR es', value: 80, reference: 79, format: String, percent: false, higherIsBetter: true, threshold: 4, n: 40, minSample: 10 },
    { id: 'dpr', subject: 'Las muertes por ronda son', value: 0.6, reference: 0.7, format: String, percent: false, higherIsBetter: false, threshold: 0.03, n: 40, minSample: 10 },
  ]);
  assert.deepEqual(stats.map((s) => s.id), ['dpr', 'open']);
  assert.equal(stats[1]?.text, 'La apertura es 39 %, 6 puntos peor que la media');
  assert.equal(stats[1]?.tone, 'worse');
  assert.equal(stats[0]?.tone, 'better');
});

test('a team trend gives each player a gap where they did not play', () => {
  const player = (id: string, kills: number) => ({ steamid64: id, name: id, team_key: 'team1', slot: 0, rounds: 20, kills, deaths: 10, assists: 0, damage: 1600 });
  const match = (id: string, playedAt: string, players: ReturnType<typeof player>[]) =>
    ({ id, played_at: playedAt, teams: [{ key: 'team1', team_id: 't' }, { key: 'team2', team_id: 'r' }], players }) as unknown as LibraryMatch;
  const trend = teamTrend(
    [match('m2', '2026-11-02T00:00:00Z', [player('a', 10)]), match('m1', '2026-11-01T00:00:00Z', [player('a', 20), player('b', 10)])],
    't',
    { metric: 'kpr' },
  );
  assert.deepEqual(trend.matches.map((m) => m.id), ['m1', 'm2']);
  assert.deepEqual(trend.players.find((p) => p.id === 'b')?.values, [0.5, null]);
  assert.deepEqual(trend.team, [0.75, 0.5]);
  assert.equal(teamTrend(trend.matches, 't', { metric: 'kpr', minMatches: 2 }).players.length, 1);
});

test('a gapped series breaks into runs', () => {
  const points = gappedPoints([1, null, 2, 3], { domain: [0, 4], width: 30, height: 40 });
  assert.equal(points[1], null);
  assert.deepEqual(segmentsOf(points).map((run) => run.length), [1, 2]);
});

test('a prep compares openings with the baseline and win rates with the library', () => {
  const prep = { matchups: [{ side: 'CT', buy: 'full', opponent_buy: 'full', win_rate: rate(4, 10) }] } as unknown as TeamPrep;
  const other = { matchups: [{ side: 'CT', buy: 'full', opponent_buy: 'full', win_rate: rate(16, 20) }] } as unknown as TeamPrep;
  const library = poolMatchups([prep, other]);
  const metric = (mean: number) => ({ mean, players: 10 });
  const baseline = { metrics: { opening_win_pct: metric(50), adr: metric(75), kpr: metric(0.7), dpr: metric(0.68) } } as unknown as Parameters<typeof prepStatInputs>[0]['baseline'];
  const scout = {
    tendencies: { round_count: 30, openings: { won: rate(12, 30) }, players: [{ kills: 20, deaths: 25, damage: 2100, rounds: 30 }] },
  } as unknown as Parameters<typeof prepStatInputs>[0]['scout'];
  const stats = significantStats(prepStatInputs({ prep, scout, library, baseline }));
  const texts = stats.map((s) => s.text);
  assert.ok(texts.includes('La apertura es 40 %, 10 puntos peor que la media'), texts.join(' | '));
  assert.ok(texts.includes('Gana en CT el 40 %, 27 puntos peor que la media'), texts.join(' | '));
  assert.ok(stats.every((s) => s.tone === 'worse'));
});
