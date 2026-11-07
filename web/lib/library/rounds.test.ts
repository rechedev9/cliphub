import test from 'node:test';
import assert from 'node:assert/strict';
import type { RoundTimeline, WinPoint } from '../api/metrics.ts';
import type { TacticalBuyType, TacticalDocument, TacticalRound, TacticalTeam } from '../api/tactical.ts';
import {
  EVERY_ROUND,
  halfScoreText,
  keepsRound,
  roundEvidenceHref,
  roundHalves,
  roundRows,
  roundWinChance,
  teamScoreBefore,
  visibleHalves,
  winnerCounts,
} from './rounds.ts';

const TEAMS: TacticalTeam[] = [
  { key: 'ct-start', name: 'Vitality', start_side: 'CT', slots: [], steamids: [] },
  { key: 't-start', name: 'MOUZ', start_side: 'T', slots: [], steamids: [] },
];

type RoundSpec = {
  number: number;
  half: number;
  winner: TacticalRound['winner'];
  /** Score before the round as the demo counts it, by side. */
  ct: number;
  t: number;
  overtime?: number;
  buys?: [TacticalBuyType, TacticalBuyType];
  tags?: string[];
};

function round(spec: RoundSpec): TacticalRound {
  const [ctBuy, tBuy] = spec.buys ?? ['full', 'full'];
  return {
    number: spec.number, tick_start: spec.number * 10000, tick_freeze_end: spec.number * 10000 + 640,
    tick_end: spec.number * 10000 + 640 + 64 * 45, tick_official: 0, score_ct_before: spec.ct, score_t_before: spec.t,
    winner: spec.winner, end_reason: 'ct_eliminated_t', half: spec.half, overtime: spec.overtime,
    economy: { ct_equip_value: 0, t_equip_value: 0, ct_money: 0, t_money: 0, ct_buy: ctBuy, t_buy: tBuy, sample_tick: 0 },
    class: { t_side: 'execute', ct_side: 'hold', site: 'A', opening_traded: false, tags: spec.tags ?? [], reasons: [] },
    players: [], events: [],
  };
}

// Vitality starts CT. Regulation ends 3-3, the first overtime 6-4 for Vitality.
// The overtime starts on the second-half sides, so the scan keeps half 2 for its first half.
const ROUNDS: TacticalRound[] = [
  round({ number: 1, half: 1, winner: 'CT', ct: 0, t: 0, buys: ['pistol', 'pistol'], tags: ['pistol'] }),
  round({ number: 2, half: 1, winner: 'T', ct: 1, t: 0, buys: ['full', 'eco'] }),
  round({ number: 3, half: 1, winner: 'CT', ct: 1, t: 1 }),
  round({ number: 4, half: 2, winner: 'T', ct: 1, t: 2, buys: ['pistol', 'pistol'], tags: ['pistol'] }),
  round({ number: 5, half: 2, winner: 'CT', ct: 1, t: 3, buys: ['full', 'force'] }),
  round({ number: 6, half: 2, winner: 'CT', ct: 2, t: 3 }),
  round({ number: 7, half: 2, overtime: 1, winner: 'T', ct: 3, t: 3 }),
  round({ number: 8, half: 2, overtime: 1, winner: 'T', ct: 3, t: 4 }),
  round({ number: 9, half: 3, overtime: 1, winner: 'T', ct: 5, t: 3 }),
  round({ number: 10, half: 3, overtime: 1, winner: 'CT', ct: 5, t: 4 }),
];

const DOC: Pick<TacticalDocument, 'teams' | 'rounds' | 'demo'> = {
  teams: TEAMS,
  rounds: ROUNDS,
  demo: { path: '', sha256: '', map: 'de_mirage', tickrate: 64, duration_ticks: 0, format: '', max_rounds: 6 },
};

function point(kind: string, probabilityCT: number, slots: Partial<Pick<WinPoint, 'actor_slot' | 'target_slot'>> = {}): WinPoint {
  return { tick: Math.round(probabilityCT * 1000), seconds: probabilityCT * 100, kind, alive_ct: 5, alive_t: 5, probability_ct: probabilityCT, ...slots };
}

const TIMELINES: RoundTimeline[] = [
  { match_id: 'm', round: 1, winner: 'CT', points: [point('start', 0.5), point('end', 1)] },
  {
    match_id: 'm', round: 2, winner: 'T', points: [
      point('start', 0.75),
      point('kill', 0.875, { actor_slot: 1, target_slot: 6 }),
      point('plant', 0.5),
      point('kill', 0.25, { actor_slot: 7, target_slot: 2 }),
      point('end', 0),
    ],
  },
];

test('each round names its winner and buys by team across the side swap, with the score after it', () => {
  const rows = roundRows(DOC);
  assert.deepEqual(rows.map((row) => [row.number, row.winnerTeamKey, row.scoreAfter['ct-start'], row.scoreAfter['t-start']]), [
    [1, 'ct-start', 1, 0], [2, 't-start', 1, 1], [3, 'ct-start', 2, 1], [4, 'ct-start', 3, 1], [5, 't-start', 3, 2],
    [6, 't-start', 3, 3], [7, 'ct-start', 4, 3], [8, 'ct-start', 5, 3], [9, 't-start', 5, 4], [10, 'ct-start', 6, 4],
  ]);
  const fifth = rows[4];
  assert.deepEqual([fifth?.sides, fifth?.buys], [{ 'ct-start': 'T', 't-start': 'CT' }, { 'ct-start': 'force', 't-start': 'full' }]);
  assert.deepEqual([rows[0]?.tick, rows[0]?.liveSeconds], [10640, 45]);
});

test('a round without a result has no winning team and keeps the score', () => {
  const [row] = roundRows({ ...DOC, rounds: [round({ number: 13, half: 2, winner: '', ct: 0, t: 12 })] });
  assert.deepEqual([row?.winnerTeamKey, row?.scoreAfter], ['', { 'ct-start': 12, 't-start': 0 }]);
});

test('without a CT starter a round names no winning team', () => {
  const [row] = roundRows({ ...DOC, teams: [], rounds: [round({ number: 1, half: 1, winner: 'CT', ct: 0, t: 0 })] });
  assert.deepEqual([row?.winnerTeamKey, row?.scoreAfter], ['', { 'ct-start': 0, 't-start': 0 }]);
});

test('the score keeps the team that started CT first across the side swap', () => {
  // 5kMirage: the CT starters lead 7:4 before round 12 and 8:4 before round 13, which the demo counts by side as 4:8.
  const r12 = { half: 1, score_ct_before: 7, score_t_before: 4 };
  const r13 = { half: 2, score_ct_before: 4, score_t_before: 8 };
  assert.deepEqual(teamScoreBefore({ teams: TEAMS }, r12), [7, 4]);
  assert.deepEqual(teamScoreBefore({ teams: TEAMS }, r13), [8, 4]);
  assert.deepEqual(teamScoreBefore({ teams: [] }, r13), [4, 8]);
});

test('halves carry their sides and score, overtime halves included', () => {
  const halves = roundHalves(roundRows(DOC));
  assert.deepEqual(
    halves.map((half) => [half.title, half.sides['ct-start'], half.rows.map((row) => row.number), halfScoreText(half)]),
    [
      ['Primera mitad', 'CT', [1, 2, 3], '2-1'],
      ['Segunda mitad', 'T', [4, 5, 6], '1-2, total 3-3'],
      ['Prórroga 1, primera mitad', 'T', [7, 8], '2-0, total 5-3'],
      ['Prórroga 1, segunda mitad', 'CT', [9, 10], '1-1, final 6-4'],
    ],
  );
});

test('the win chance names an upset winner and the kill that moved the chance most', () => {
  const rows = roundRows(DOC, TIMELINES);
  assert.deepEqual(rows[0]?.chance, { winnerStart: 0.5, upset: false, swing: null });
  // The plant moves the chance more, but only kills count; the T kill beats the earlier CT one.
  assert.deepEqual(rows[1]?.chance, {
    winnerStart: 0.25,
    upset: true,
    swing: { tick: 250, seconds: 25, actorSlot: 7, targetSlot: 2, side: 'T', gain: 0.25 },
  });
  assert.equal(rows[2]?.chance, null);
  assert.equal(roundWinChance([point('start', 0.2)], '')?.upset, false);
  assert.equal(roundWinChance([], 'CT'), null);
});

test('filters keep rounds by winning team and round type, either team counting for a buy', () => {
  const rows = roundRows(DOC, TIMELINES);
  const kept = (filter: typeof EVERY_ROUND): number[] => rows.filter((row) => keepsRound(filter, row)).map((row) => row.number);
  assert.deepEqual(kept({ winner: 't-start', kind: 'any' }), [2, 5, 6, 9]);
  assert.deepEqual(kept({ winner: 'all', kind: 'pistol' }), [1, 4]);
  assert.deepEqual(kept({ winner: 'all', kind: 'eco' }), [2]);
  assert.deepEqual(kept({ winner: 'all', kind: 'force' }), [5]);
  assert.deepEqual(kept({ winner: 'all', kind: 'upset' }), [2]);
  assert.deepEqual(kept({ winner: 't-start', kind: 'pistol' }), []);
  assert.deepEqual(winnerCounts(rows), { all: 10, 'ct-start': 6, 't-start': 4 });
});

test('a filtered half keeps the score of the whole half and an empty half is dropped', () => {
  const halves = visibleHalves(roundHalves(roundRows(DOC)), { winner: 'all', kind: 'force' });
  assert.deepEqual(halves.map((half) => [half.title, half.rows.map((row) => row.number), halfScoreText(half)]), [
    ['Segunda mitad', [5], '1-2, total 3-3'],
  ]);
});

test('an evidence link opens the replay tab on the round and tick', () => {
  assert.equal(roundEvidenceHref('/partidas/abc', 7, 51234), '/partidas/abc?tab=tactica&round=7&tick=51234');
});
