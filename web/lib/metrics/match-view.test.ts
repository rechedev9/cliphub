import test from 'node:test';
import assert from 'node:assert/strict';
import type { LibraryMatchPlayer } from '../api/library.ts';
import type { MetricsReport, PlayerMetrics, RoundTimeline, WinPoint } from '../api/metrics.ts';
import {
  bestValues,
  ctTeamByRound,
  favouredTeam,
  generalText,
  isBest,
  playerRows,
  roundBars,
  teamGroups,
  topSwings,
  winPointText,
} from './match-view.ts';

const MATCH = 'aaaaaaaa-0000-4000-8000-000000000001';

// Slots 0 to 4 started CT, 5 to 9 started T.
const PLAYERS: Pick<LibraryMatchPlayer, 'slot' | 'team_key' | 'steamid64'>[] = Array.from({ length: 10 }, (_, slot) => ({
  slot,
  team_key: slot < 5 ? 'ct-start' : 't-start',
  steamid64: `7656119800000000${slot}`,
}));

const start = (probability: number): WinPoint => ({ tick: 100, seconds: 0, kind: 'start', alive_ct: 5, alive_t: 5, probability_ct: probability });
const end = (probability: number): WinPoint => ({ tick: 9000, seconds: 90, kind: 'end', alive_ct: 1, alive_t: 1, probability_ct: probability });
function kill(options: { tick: number; target: number; victim: 'CT' | 'T'; probability: number; actor?: number }): WinPoint {
  return {
    tick: options.tick, seconds: options.tick / 64, kind: 'kill', actor_slot: options.actor, target_slot: options.target,
    alive_ct: options.victim === 'CT' ? 4 : 5, alive_t: options.victim === 'T' ? 4 : 5, probability_ct: options.probability,
  };
}
function round(number: number, winner: RoundTimeline['winner'], points: WinPoint[]): RoundTimeline {
  return { match_id: MATCH, round: number, winner, points };
}

const TIMELINES: RoundTimeline[] = [
  round(1, 'CT', [start(0.5), kill({ tick: 1280, target: 5, victim: 'T', probability: 0.68, actor: 0 }), end(1)]),
  round(13, 'CT', [start(0.6), kill({ tick: 640, target: 5, victim: 'CT', probability: 0.41 }), end(1)]),
  round(14, 'T', [start(0.55), end(0)]),
  round(25, 'T', [start(0.5), kill({ tick: 2560, target: 0, victim: 'T', probability: 0.9 }), end(0)]),
  round(26, 'T', [start(0.5), end(0)]),
  round(28, 'CT', [start(0.45), kill({ tick: 960, target: 0, victim: 'CT', probability: 0.25 }), end(1)]),
  round(29, 'CT', [start(0.5), end(1)]),
];

test('the CT team of each round comes from who died on which side, across the half and overtime', () => {
  const ct = ctTeamByRound(TIMELINES, PLAYERS);
  assert.deepEqual(
    [...ct],
    [[1, 'ct-start'], [13, 't-start'], [14, 't-start'], [25, 't-start'], [26, 't-start'], [28, 'ct-start'], [29, 'ct-start']],
  );
  // Without any kill to read, the CS2 format decides.
  assert.deepEqual([...ctTeamByRound([round(2, 'CT', [start(0.5)]), round(20, 'T', [start(0.5)])], [])], [[2, 'ct-start'], [20, 't-start']]);
});

test('each bar is the team chance at the start, its outcome and a gap where the side changes', () => {
  const bars = roundBars({ timelines: TIMELINES, ctTeams: ctTeamByRound(TIMELINES, PLAYERS), team: 'ct-start' });
  assert.deepEqual(
    bars.map((bar) => [bar.round, bar.side, Math.round(bar.chance * 100), bar.outcome, bar.halfStart]),
    [
      [1, 'CT', 50, 'win', false],
      [13, 'T', 40, 'loss', true],
      [14, 'T', 45, 'win', false],
      [25, 'T', 50, 'win', false],
      [26, 'T', 50, 'win', false],
      [28, 'CT', 45, 'win', true],
      [29, 'CT', 50, 'win', false],
    ],
  );
  assert.equal(bars[0]?.swing?.tick, 1280);
  assert.equal(bars[2]?.swing, undefined);
});

test('the biggest kills of the match come first and name the team they favoured', () => {
  const ctTeams = ctTeamByRound(TIMELINES, PLAYERS);
  const swings = topSwings(TIMELINES, 3);
  assert.deepEqual(swings.map((s) => [s.round, Math.round(s.delta * 100)]), [[25, 40], [28, -20], [13, -19]]);
  assert.deepEqual(swings.map((s) => favouredTeam(ctTeams, s)), ['t-start', 't-start', 'ct-start']);
  const names = new Map([[0, 'ropz']]);
  assert.equal(winPointText(TIMELINES[0]?.points[1] ?? start(0), new Map([...names, [5, 'jL']])), 'ropz mata a jL');
  assert.equal(winPointText(TIMELINES[1]?.points[1] ?? start(0), names), 'Jugador 5 muere');
});

const REPORT_BASE: MetricsReport = {
  skipped: [],
  model: {
    version: 'wp1', alive_coef: 0.9, equip_coef: 1, clock_coef: 1.2, plant_coef: -1.1, bomb_coef: -1.5,
    round_seconds: 115, bomb_seconds: 40, min_equip: 200, map_priors: {},
  },
  thresholds: { duel_even_ratio: 1.5, flash_min_seconds: 1, trade_seconds: 5, setup_seconds: 15, example_limit: 8 },
  matches: 1,
  rounds: 20,
  utility_matches: 1,
  players: [],
  roles: [],
};

function player(overrides: Partial<PlayerMetrics>): PlayerMetrics {
  const none = { count: 0, examples: [] };
  return {
    steamid64: '76561198000000000', name: 'ropz', role: 'rifler', matches: 1, rounds: 20, kills: 15, deaths: 12,
    eco: { kills: 0, deaths: 0, examples: [] },
    duels: { even: { won: none, lost: none }, advantage: { won: none, lost: none }, disadvantage: { won: none, lost: none } },
    swing: { wpa: 0.2, per_round: 0.01, best: [], worst: [] },
    trades: { trade_kills: { count: 3, examples: [] }, traded_deaths: none, untraded_deaths: none },
    utility: {
      measured: true,
      flash: { thrown: 0, enemies_blinded: none, team_blinded: none, enemy_blind_seconds: 0, assists: none },
      he: { thrown: 2, enemy_damage: 90, team_damage: 0, examples: [] },
      molotov: { thrown: 1, enemy_damage: 30, team_damage: 0, examples: [] },
    },
    positions: [], match_lines: [], buys: [],
    box: { damage: 1600, assists: 2, kast_rounds: 15, opening_kills: 3, opening_deaths: 1, utility_rounds: 20 },
    ...overrides,
  };
}

test('the General table groups players by team and marks the best figure of each column', () => {
  const players = [
    player({}),
    player({ steamid64: '76561198000000005', name: 'jL', kills: 20, swing: { wpa: 0.5, per_round: 0.025, best: [], worst: [] }, trades: { ...player({}).trades, trade_kills: { count: 0, examples: [] } } }),
    player({
      steamid64: '76561198000000099', name: 'sub', utility: { ...player({}).utility, measured: false },
      box: { ...player({}).box, opening_kills: 0, opening_deaths: 0 },
      match_lines: [{ match_id: MATCH, map: 'de_inferno', played_at: '', team_key: 't-start', rounds: 20, kills: 0, deaths: 0, eco_kills: 0, wpa: 0 }],
    }),
  ];
  const rows = playerRows({ ...REPORT_BASE, players }, PLAYERS);
  assert.deepEqual(teamGroups(rows).map((g) => [g.teamKey, g.rows.map((r) => r.player.name)]), [['ct-start', ['ropz']], ['t-start', ['jL', 'sub']]]);
  const best = bestValues(rows);
  assert.equal(isBest(rows[0]?.values.kpr ?? null, best.kpr), true);
  assert.equal(isBest(rows[1]?.values.kpr ?? null, best.kpr), false);
  assert.deepEqual(
    (['kast', 'adr', 'kpr', 'opening', 'trades', 'utility'] as const).map((c) => generalText(c, rows[1]?.values[c] ?? null)),
    ['75,0 %', '80,0', '0,75', '75 %', '3', '6,0'],
  );
  assert.equal(generalText('opening', rows[2]?.values.opening ?? null), 'Sin duelos');
  assert.equal(generalText('utility', rows[2]?.values.utility ?? null), 'Sin medir');
});
