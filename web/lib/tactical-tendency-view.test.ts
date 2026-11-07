import test from 'node:test';
import assert from 'node:assert/strict';
import { TACTICAL_SIDES } from './api/tactical.ts';
import type { TacticalRate, TacticalTendencies } from './api/tactical.ts';
import {
  matchWinnerKey,
  openingLines,
  panelFilter,
  panelTeamKey,
  percentText,
  teamSummaryLine,
  tendencyCards,
} from './tactical-tendency-view.ts';

const TEAMS = [
  { key: 'cat', start_side: TACTICAL_SIDES.t, slots: [0, 1, 2, 3, 4] },
  { key: 'miwel', start_side: TACTICAL_SIDES.ct, slots: [5, 6, 7, 8, 9] },
];

// Half 1: cat is T, miwel CT. Half 2 swaps.
const ROUNDS = [
  { half: 1, winner: TACTICAL_SIDES.ct },
  { half: 1, winner: TACTICAL_SIDES.t },
  { half: 2, winner: TACTICAL_SIDES.t },
  { half: 2, winner: TACTICAL_SIDES.ct },
  { half: 2, winner: '' as const },
  { half: 2, winner: TACTICAL_SIDES.ct },
];

function rate(count: number, total: number): TacticalRate {
  return { count, total, pct: total > 0 ? Math.round((count / total) * 1000) / 10 : 0, reliable: total >= 4 };
}

test('the match winner follows each team across the side swap', () => {
  // miwel: R1 (CT), R3 (T in half 2). cat: R2 (T), R4 and R6 (CT in half 2).
  assert.equal(matchWinnerKey(TEAMS, ROUNDS), 'cat');
  assert.equal(matchWinnerKey(TEAMS, ROUNDS.slice(0, 2)), undefined);
  assert.equal(matchWinnerKey([], ROUNDS), undefined);
});

test('the panel team is the filter team, then the pick, then the players, then the winner', () => {
  const base = { teams: TEAMS, rounds: ROUNDS, chosen: undefined };
  assert.equal(panelTeamKey({ ...base, filter: { team_key: 'miwel' }, chosen: 'cat' }), 'miwel');
  assert.equal(panelTeamKey({ ...base, filter: {}, chosen: 'miwel' }), 'miwel');
  assert.equal(panelTeamKey({ ...base, filter: {}, chosen: 'gone' }), 'cat');
  assert.equal(panelTeamKey({ ...base, filter: { slots: [6] } }), 'miwel');
  // Players of both teams name no single team, so the winner decides.
  assert.equal(panelTeamKey({ ...base, filter: { slots: [1, 6] } }), 'cat');
  assert.equal(panelTeamKey({ ...base, rounds: ROUNDS.slice(0, 2), filter: {} }), 'cat');
  assert.equal(panelTeamKey({ ...base, teams: TEAMS.slice(0, 1), filter: {} }), undefined);
});

test('the panel filter adds its team without touching the replay filter', () => {
  const replay = { side: TACTICAL_SIDES.t, buys: ['full' as const] };
  assert.deepEqual(panelFilter(replay, 'cat'), { side: 'T', buys: ['full'], team_key: 'cat' });
  assert.deepEqual(replay, { side: 'T', buys: ['full'] });
  assert.deepEqual(panelFilter({ team_key: 'miwel' }, 'cat'), { team_key: 'miwel' });
  assert.equal(panelFilter(replay, undefined), replay);
});

test('the summary line reads in Spanish with its win share', () => {
  assert.equal(
    teamSummaryLine({ rounds: 23, wins: 13, filtered: true }),
    'Sus 23 rondas de esta partida, con los filtros del replay. Ganó 13 (56,5 %).',
  );
  assert.equal(teamSummaryLine({ rounds: 1, wins: 1, filtered: false }), 'Su única ronda de esta partida. Ganó 1 (100 %).');
  assert.equal(percentText(28.6), '28,6 %');
});

function tendencies(over: Partial<TacticalTendencies>): TacticalTendencies {
  const histogram = { bucket_seconds: 5, samples: 0, median: 0, buckets: [] };
  return {
    filter: {},
    round_count: 12,
    wins: 6,
    perspective: TACTICAL_SIDES.ct,
    buys: [],
    matchups: [],
    sites: [],
    buy_sites: [],
    t_patterns: [],
    ct_patterns: [],
    openings: { rounds: 0, won: rate(0, 0), traded_on_loss: rate(0, 0), round_win_after_opening_win: rate(0, 0), round_win_after_opening_loss: rate(0, 0) },
    timings: { first_contact: histogram, plant: histogram, round_duration: histogram },
    players: [],
    ...over,
  };
}

test('cards carry rounds, win rate, outcome tone and the small-sample mark', () => {
  const cards = tendencyCards(
    tendencies({
      buys: [
        { buy: 'full', rounds: 14, share: rate(14, 23), win_rate: rate(10, 14), conversion: rate(3, 14) },
        { buy: 'force', rounds: 2, share: rate(2, 23), win_rate: rate(1, 2), conversion: rate(0, 2) },
      ],
      sites: [{ site: 'none', rounds: 3, share: rate(3, 23), win_rate: rate(0, 3) }],
      ct_patterns: [
        { pattern: 'retake', rounds: 5, share: rate(5, 11), win_rate: rate(1, 5) },
        { pattern: 'hold', rounds: 6, share: rate(6, 11), win_rate: rate(6, 6) },
      ],
    }),
  );
  // No T rounds under this filter: the "Como T" card is dropped instead of drawn empty.
  assert.deepEqual(cards.map((card) => card.title), ['Economía', 'Sitio de la ronda', 'Como CT · 11 rondas']);
  const [full, force] = cards[0]?.rows ?? [];
  assert.deepEqual(
    { label: full?.label, winText: full?.winText, tone: full?.tone, small: full?.small, shareText: full?.shareText },
    { label: 'Full', winText: '71,4 %', tone: 'win', small: false, shareText: '60,9 % de 23 rondas' },
  );
  assert.deepEqual({ tone: force?.tone, small: force?.small }, { tone: 'win', small: true });
  assert.deepEqual(cards[1]?.rows.map((row) => [row.label, row.tone]), [['Sin sitio', 'loss']]);
  assert.deepEqual(cards[2]?.rows.map((row) => row.label), ['Retoma', 'Defensa']);
});

test('opening lines sort by duels won, then fewest lost', () => {
  const player = (name: string, slot: number, kills: number, deaths: number) => ({
    slot,
    name,
    rounds: 23,
    kills: 0,
    deaths: 0,
    assists: 0,
    damage: 0,
    adr: 0,
    opening_kills: kills,
    opening_deaths: deaths,
    trade_kills: 0,
    survival_rate: rate(0, 23),
  });
  const lines = openingLines([player('miwel', 5, 3, 3), player('', 7, 4, 1), player('tilr0y', 6, 4, 0), player('gR7fc', 8, 0, 2)]);
  assert.deepEqual(
    lines.map((line) => [line.name, line.text, line.tone]),
    [
      ['tilr0y', '4-0', 'win'],
      ['Jugador 7', '4-1', 'win'],
      ['miwel', '3-3', 'even'],
      ['gR7fc', '0-2', 'loss'],
    ],
  );
});
