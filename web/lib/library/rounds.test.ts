import test from 'node:test';
import assert from 'node:assert/strict';
import type { TacticalDocument, TacticalRound, TacticalTeam } from '../api/tactical.ts';
import { roundEvidenceHref, roundRows, teamScoreBefore } from './rounds.ts';

const TEAMS: TacticalTeam[] = [
  { key: 'ct-start', name: 'Vitality', start_side: 'CT', slots: [], steamids: [] },
  { key: 't-start', name: 'MOUZ', start_side: 'T', slots: [], steamids: [] },
];

function round(number: number, half: number, winner: TacticalRound['winner']): TacticalRound {
  return {
    number, tick_start: number * 1000, tick_freeze_end: number * 1000 + 640, tick_end: number * 1000 + 640 + 64 * 45,
    tick_official: 0, score_ct_before: number - 1, score_t_before: 0, winner, end_reason: 'ct_eliminated_t', half,
    economy: { ct_equip_value: 0, t_equip_value: 0, ct_money: 0, t_money: 0, ct_buy: 'full', t_buy: 'eco', sample_tick: 0 },
    class: { t_side: 'execute', ct_side: 'hold', site: 'A', opening_traded: false, tags: [], reasons: [] },
    players: [], events: [],
  };
}

test('each round names the winning team across the side swap and links to its live tick', () => {
  const doc: Pick<TacticalDocument, 'teams' | 'rounds' | 'demo'> = {
    teams: TEAMS,
    rounds: [round(1, 1, 'CT'), round(13, 2, 'CT'), round(14, 2, '')],
    demo: { path: '', sha256: '', map: 'de_mirage', tickrate: 64, duration_ticks: 0, format: '', max_rounds: 24 },
  };
  const rows = roundRows(doc);
  assert.deepEqual(rows.map((row) => [row.number, row.winnerTeamKey, row.tick, row.liveSeconds, row.scoreBefore]), [
    [1, 'ct-start', 1640, 45, '0-0'],
    [13, 't-start', 13640, 45, '0-12'],
    [14, '', 14640, 45, '0-13'],
  ]);
});

test('the score keeps the team that started CT first across the side swap', () => {
  // 5kMirage: the CT starters lead 7:4 before round 12 and 8:4 before round 13, which the demo counts by side as 4:8.
  const r12 = { half: 1, score_ct_before: 7, score_t_before: 4 };
  const r13 = { half: 2, score_ct_before: 4, score_t_before: 8 };
  assert.deepEqual(teamScoreBefore({ teams: TEAMS }, r12), [7, 4]);
  assert.deepEqual(teamScoreBefore({ teams: TEAMS }, r13), [8, 4]);
  assert.deepEqual(teamScoreBefore({ teams: [] }, r13), [4, 8]);
});

test('an evidence link opens the replay tab on the round and tick', () => {
  assert.equal(roundEvidenceHref('/partidas/abc', 7, 51234), '/partidas/abc?tab=tactica&round=7&tick=51234');
});
