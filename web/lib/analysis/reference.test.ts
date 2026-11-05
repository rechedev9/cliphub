import test from 'node:test';
import assert from 'node:assert/strict';
import type { Baseline } from '../api/baseline.ts';
import type { PlayerMetrics } from '../api/metrics.ts';
import { METRIC_INFO, baselineValue, leaderboardEmptyText, playerReferenceValues, proUnavailableText } from './reference.ts';

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

const baseline = (players: number, matches: number): Baseline => ({
  scope: 'pro', map: '', min_rounds: 50, players, rounds: 0, matches, skipped: 0,
  metrics: {
    kpr: { mean: 0.7, players }, dpr: { mean: 0.6, players: 0 }, adr: { mean: 0, players }, kast: { mean: 0, players },
    opening_win_pct: { mean: 0, players }, trade_pct: { mean: 0, players }, utility_damage_per_round: { mean: 0, players },
    flash_assists_per_round: { mean: 0, players }, wpa_per_round: { mean: 0, players },
  },
});

test('reference: a metric no player has gives no reference', () => {
  assert.equal(baselineValue(baseline(3, 2), 'kpr'), 0.7);
  assert.equal(baselineValue(baseline(3, 2), 'dpr'), null);
  assert.equal(baselineValue(null, 'kpr'), null);
});

test('reference: an empty ranking says what to change', () => {
  assert.match(leaderboardEmptyText({ scope: 'pro', matches: 0, min_rounds: 50 }), /cambia a Biblioteca/);
  assert.match(leaderboardEmptyText({ scope: 'library', matches: 3, min_rounds: 200 }), /200 rondas.*Baja el mínimo/);
});

test('reference: Pro says why it is missing', () => {
  assert.equal(proUnavailableText(baseline(3, 2)), null);
  assert.match(proUnavailableText(baseline(0, 0)) ?? '', /no tiene demos de HLTV/);
  assert.match(proUnavailableText(baseline(0, 4)) ?? '', /llega a 50 rondas/);
});
