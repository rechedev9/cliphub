import test from 'node:test';
import assert from 'node:assert/strict';
import type { Baseline } from '../api/baseline.ts';
import type { PlayerMetrics } from '../api/metrics.ts';
import {
  METRIC_INFO,
  baselineValue,
  leaderboardEmptyText,
  metricReliability,
  playerEvidence,
  playerReferenceValues,
  proReferenceShort,
  proSampleText,
  proSourceText,
  roundReferences,
} from './reference.ts';

function player(overrides: Partial<PlayerMetrics>): PlayerMetrics {
  const count = (n: number) => ({ count: n, examples: [] });
  const grenade = (damage: number) => ({ thrown: 0, enemy_damage: damage, team_damage: 0, examples: [] });
  return {
    steamid64: '1', name: 'p', role: '', matches: 1, rounds: 20, kills: 16, deaths: 10,
    eco: { kills: 0, deaths: 0, examples: [] },
    duels: { even: { won: count(0), lost: count(0) }, advantage: { won: count(0), lost: count(0) }, disadvantage: { won: count(0), lost: count(0) } },
    swing: { wpa: 0.4, per_round: 0.02, best: [], worst: [] },
    trades: { trade_kills: count(0), traded_deaths: count(4), untraded_deaths: count(6) },
    utility: {
      measured: true,
      flash: { thrown: 0, enemies_blinded: count(0), team_blinded: count(0), enemy_blind_seconds: 0, assists: count(2) },
      he: grenade(60), molotov: grenade(40),
    },
    positions: [], match_lines: [], buys: [],
    box: { damage: 1600, assists: 3, kast_rounds: 15, opening_kills: 3, opening_deaths: 1, utility_rounds: 10 },
    ...overrides,
  };
}

// The values mirror tacticalplan.playerTotals.values, so a card compares like with like.
test('reference: a player value uses the same denominators as the baseline', () => {
  assert.deepEqual(playerReferenceValues(player({})), {
    kpr: 0.8, dpr: 0.5, adr: 80, kast: 75, wpa_per_round: 0.02,
    opening_win_pct: 75, trade_pct: 40, utility_damage_per_round: 10, flash_assists_per_round: 0.2,
  });
});

test('reference: a metric without a denominator is left out, not zero', () => {
  const values = playerReferenceValues(
    player({ deaths: 0, box: { damage: 0, assists: 0, kast_rounds: 0, opening_kills: 0, opening_deaths: 0, utility_rounds: 0 } }),
  );
  assert.equal('trade_pct' in values, false);
  assert.equal('opening_win_pct' in values, false);
  assert.equal('utility_damage_per_round' in values, false);
  assert.deepEqual(playerReferenceValues(player({ rounds: 0 })), {});
});

test('reference: only deaths per round ranks lower first', () => {
  const lower = Object.entries(METRIC_INFO).filter(([, info]) => !info.higherIsBetter).map(([metric]) => metric);
  assert.deepEqual(lower, ['dpr']);
});

const value = (mean: number, players: number) => ({ mean, players, stddev: 0, prior_stddev: 0 });
const baseline = (players: number, matches: number): Baseline => ({
  scope: 'pro', map: '', source: 'bundled', description: '4 mapas profesionales de 2026.', min_rounds: 15, players, rounds: 0, matches, skipped: 0,
  metrics: {
    kpr: value(0.7, players), dpr: value(0.6, 0), adr: value(0, players), kast: value(0, players),
    opening_win_pct: value(0, players), trade_pct: value(0, players), utility_damage_per_round: value(0, players),
    flash_assists_per_round: value(0, players), wpa_per_round: value(0, players),
  },
});

test('reference: a metric no player has gives no reference', () => {
  assert.equal(baselineValue(baseline(3, 2), 'kpr'), 0.7);
  assert.equal(baselineValue(baseline(3, 2), 'dpr'), null);
  assert.equal(baselineValue(null, 'kpr'), null);
});

test('reference: an empty ranking says what to change', () => {
  assert.match(leaderboardEmptyText({ matches: 0, min_rounds: 50 }), /Importa demos/);
  assert.match(leaderboardEmptyText({ matches: 3, min_rounds: 200 }), /200 rondas.*Baja el mínimo/);
});

test('reference: Pro says where its numbers come from', () => {
  assert.match(proSourceText(baseline(20, 4)), /^Referencia pro incluida en Studio: 4 mapas profesionales/);
  assert.equal(proReferenceShort(baseline(20, 4)), '4 mapas pro incluidos');
  const library: Baseline = { ...baseline(12, 9), source: 'library', description: undefined };
  assert.match(proSourceText(library), /^Tus demos de HLTV: 12 jugadores/);
  assert.equal(proReferenceShort(library), '9 demos de HLTV');
  const server: Baseline = { ...baseline(180, 283), source: 'server', rounds: 6120, updated_at: '2026-11-06T09:00:00Z', description: '283 mapas profesionales.' };
  assert.match(proSourceText(server), /^Referencia pro: 283 mapas, 6120 rondas, actualizada 6 nov 2026\. 283 mapas profesionales\.$/);
  assert.equal(proReferenceShort(server), '283 mapas pro');
  assert.equal(proSampleText({ matches: 1, rounds: 20 }), 'Referencia pro: 1 mapa, 20 rondas');
});

test('reference: a comparison is called only when the interval leaves the mean out', () => {
  const kpr = { mean: 0.7, players: 20, stddev: 0.15, prior_stddev: 0.1, round_stddev: 0.85 };
  const evidence = playerEvidence(player({ rounds: 20, kills: 16 }));
  assert.deepEqual(evidence.kast, { kind: 'share', successes: 15, trials: 20 });
  const few = metricReliability(evidence.kpr, kpr, 'kpr');
  assert.ok(few !== null && !few.differs && few.weak, '0,80 over 20 rounds is noise against 0,70');
  const many = metricReliability(playerEvidence(player({ rounds: 2000, kills: 1800 })).kpr, kpr, 'kpr');
  assert.ok(many !== null && many.differs && !many.weak, '0,90 over 2000 rounds is a real difference');
  assert.match(many.detail, /90 % de confianza/);
  assert.equal(metricReliability(evidence.kpr, { ...kpr, players: 0 }, 'kpr'), null);
});

test('heat tables read the library per round, and nothing where the library has no players', () => {
  const metric = (mean: number, players = 10) => value(mean, players);
  const baseline = { metrics: { kpr: metric(0.7), dpr: metric(0.65), wpa_per_round: metric(0, 0) } } as unknown as Baseline;
  const refs = roundReferences(baseline);
  assert.equal(refs.kills, 0.7);
  assert.equal(refs.wpa, null);
  assert.ok(refs.kd !== null && Math.abs(refs.kd - 0.7 / 0.65) < 1e-9);
  assert.deepEqual(roundReferences(null), { kills: null, deaths: null, wpa: null, kd: null });
});
