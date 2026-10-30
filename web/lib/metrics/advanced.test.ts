import test from 'node:test';
import assert from 'node:assert/strict';
import type { MetricCount, MetricsReport, PlayerMetrics } from '../api/metrics.ts';
import { playerMetricsPath } from '../api/library.ts';
import { duelText, metricsMatchIndex, metricsSections, ppText, winChart } from './advanced.ts';

const MATCH = 'aaaaaaaa-0000-4000-8000-000000000001';
const ev = (round: number, tick: number) => ({ match_id: MATCH, round, tick });
const count = (n: number, ...examples: ReturnType<typeof ev>[]): MetricCount => ({ count: n, examples });

function player(overrides: Partial<PlayerMetrics> = {}): PlayerMetrics {
  return {
    steamid64: '76561198000000001',
    name: 'ropz',
    role: 'rifler',
    matches: 1,
    rounds: 20,
    kills: 15,
    deaths: 12,
    eco: { kills: 14.25, deaths: 12.5, examples: [{ ...ev(4, 900), value: 1.6, killer_equip: 1000, victim_equip: 4000 }] },
    duels: {
      even: { won: count(5, ev(2, 300)), lost: count(3, ev(3, 400)) },
      advantage: { won: count(8, ev(5, 500)), lost: count(1, ev(6, 600)) },
      disadvantage: { won: count(2, ev(4, 900)), lost: count(8, ev(7, 700)) },
    },
    swing: {
      wpa: 0.4567,
      per_round: 0.0228,
      best: [{ ...ev(4, 900), kind: 'kill', delta: 0.21, before: 0.3, after: 0.51 }],
      worst: [{ ...ev(7, 700), kind: 'death', delta: -0.18, before: 0.6, after: 0.42 }],
    },
    trades: { trade_kills: count(3, ev(8, 800)), traded_deaths: count(4, ev(9, 900)), untraded_deaths: count(8) },
    utility: {
      measured: true,
      flash: { thrown: 10, enemies_blinded: count(6, ev(10, 1000)), team_blinded: count(1, ev(11, 1100)), enemy_blind_seconds: 14.2, assists: count(2, ev(10, 1010)) },
      he: { thrown: 4, enemy_damage: 120, team_damage: 5, examples: [ev(12, 1200)] },
      molotov: { thrown: 3, enemy_damage: 60, team_damage: 0, examples: [ev(13, 1300)] },
    },
    positions: [{ side: 'CT', callout: 'BombsiteA', rounds: 9, won: 6, kills: 7, deaths: 4, wpa: 0.12, examples: [ev(1, 100)] }],
    match_lines: [{ match_id: MATCH, map: 'de_mirage', played_at: '2026-10-12T18:00:00Z', team_key: 'ct-start', rounds: 20, kills: 15, deaths: 12, eco_kills: 14.25, wpa: 0.4567 }],
    ...overrides,
  };
}

function report(players: PlayerMetrics[] = [player()]): MetricsReport {
  return {
    skipped: [],
    model: {
      version: 'wp1', alive_coef: 0.9, equip_coef: 1, clock_coef: 1.2, plant_coef: -1.1, bomb_coef: -1.5,
      round_seconds: 115, bomb_seconds: 40, min_equip: 200, map_priors: { de_mirage: 0 },
    },
    thresholds: { duel_even_ratio: 1.5, flash_min_seconds: 1, trade_seconds: 5, setup_seconds: 15, example_limit: 8 },
    matches: 1,
    rounds: 20,
    utility_matches: 1,
    players,
    roles: [{ role: 'rifler', players: ['76561198000000001'], rounds: 20, kills: 15, deaths: 12, eco_kills: 14.25, eco_deaths: 12.5, wpa: 0.4567, flash_assists: 2, utility_damage: 180 }],
  };
}

test('win probability added reads in signed percentage points', () => {
  assert.equal(ppText(0.4567), '+45,7 pp');
  assert.equal(ppText(-0.0228), '-2,3 pp');
  assert.equal(ppText(0), '0,0 pp');
  assert.equal(duelText({ won: count(3), lost: count(2) }), '3-2 (60 %)');
  assert.equal(duelText({ won: count(0), lost: count(0) }), 'Sin duelos');
});

test('every metric section links its counts to rounds and never uses a dash', () => {
  const sections = metricsSections(report(), { playerHref: (id) => `/jugadores/${id}`, byMatch: true, matchHref: (id) => `/partidas/${id}` });
  assert.deepEqual(
    sections.map((s) => s.id),
    ['metricas-impacto', 'metricas-economia', 'metricas-duelos', 'metricas-trades', 'metricas-utilidad', 'metricas-posiciones', 'metricas-roles', 'metricas-partidas'],
  );
  const evidence = sections.flatMap((s) => s.tables.flatMap((t) => t.rows.flatMap((row) => row.flatMap((cell) => cell.evidence ?? []))));
  for (const tick of [900, 700, 300, 800, 1000, 1010, 1200, 100]) {
    assert.ok(evidence.some((e) => e.tick === tick), `tick ${tick} has a chip`);
  }
  const impact = sections[0]?.tables[0]?.rows[0];
  assert.equal(impact?.[0]?.href, '/jugadores/76561198000000001');
  assert.equal(impact?.[3]?.text, '+45,7 pp');
  assert.equal(sections.at(-1)?.tables[0]?.rows[0]?.[0]?.href, `/partidas/${MATCH}`);
  const all = JSON.stringify(sections);
  assert.equal(/[–—]/.test(all), false);
});

test('utility from older scans says it was not measured', () => {
  const old = report([player({ utility: { ...player().utility, measured: false } })]);
  old.utility_matches = 0;
  const utility = metricsSections(old).find((s) => s.id === 'metricas-utilidad');
  assert.ok(utility);
  assert.match(utility.intro, /antes de medir la utilidad/);
  assert.equal(utility.tables[0]?.rows[0]?.[1]?.text, 'Sin medir');
  assert.equal(utility.tables[0]?.rows[0]?.[2]?.evidence, undefined);
});

test('evidence chips name the matches the metrics read', () => {
  const index = metricsMatchIndex(report());
  assert.equal(index.get(MATCH)?.map, 'de_mirage');
});

test('the win chart steps from event to event across the round', () => {
  const chart = winChart(
    {
      match_id: MATCH, round: 1, winner: 'CT',
      points: [
        { tick: 100, seconds: 0, kind: 'start', alive_ct: 5, alive_t: 5, probability_ct: 0.5 },
        { tick: 740, seconds: 10, kind: 'kill', alive_ct: 5, alive_t: 4, probability_ct: 0.75 },
        { tick: 1380, seconds: 20, kind: 'end', alive_ct: 5, alive_t: 0, probability_ct: 1 },
      ],
    },
    200,
    100,
  );
  assert.equal(chart.maxSeconds, 20);
  assert.equal(chart.path, 'M0 50 H100 V25 H200 V0');
});

test('a player metrics query carries only what is set', () => {
  assert.equal(playerMetricsPath('765', {}), '/players/765/metrics');
  assert.equal(playerMetricsPath('765', { map: 'de_nuke', side: 'T', limit: 20 }), '/players/765/metrics?map=de_nuke&side=T&limit=20');
});
