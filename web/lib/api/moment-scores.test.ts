import assert from 'node:assert/strict';
import test from 'node:test';
import type { Play } from './types.ts';
import { parseMomentScores, withMomentScores } from './moment-scores.ts';

const plays: Play[] = [
  { id: 'seg-001', matchId: 'm', label: 'a', kind: 'highlight', round: 1, kills: 2 },
  { id: 'seg-002', matchId: 'm', label: 'b', kind: 'highlight', round: 2, kills: 1 },
];

test('moments join plays by segment id and skip malformed rows', () => {
  const scores = parseMomentScores({
    moments: [
      { segment_id: 'seg-002', score: 0.7, reason_codes: ['clutch', 'clutch_won', 3], clutch_won: true, win_prob_swing: 0.35 },
      { segment_id: 'seg-001', score: 'high' },
      { score: 0.9 },
    ],
  });
  const [first, second] = withMomentScores(plays, scores);
  assert.equal(first.score, undefined);
  assert.deepEqual(
    { score: second.score, reasons: second.reasons, clutchWon: second.clutchWon, winProbSwing: second.winProbSwing },
    { score: 0.7, reasons: ['clutch', 'clutch_won'], clutchWon: true, winProbSwing: 0.35 },
  );
  assert.equal(withMomentScores(plays, parseMomentScores(null)), plays, 'no document keeps the plays as they are');
});
