import assert from 'node:assert/strict';
import { test } from 'node:test';
import { scoreAfter, teamsInScoreOrder, winnerTeamName } from './strip-round.ts';

const doc = {
  teams: [
    { key: 'b', name: 'FaZe', start_side: 'T' },
    { key: 'a', name: 'NAVI', start_side: 'CT' },
  ],
};
// Only the fields the helpers read.
const docCast = doc as unknown as Parameters<typeof scoreAfter>[0];

test('teams come CT starter first', () => {
  assert.deepEqual(teamsInScoreOrder(docCast), ['NAVI', 'FaZe']);
});

test('first half: the CT win goes to the CT starter', () => {
  const round = { half: 1, score_ct_before: 3, score_t_before: 2, winner: 'CT' } as const;
  assert.deepEqual(scoreAfter(docCast, round), [4, 2]);
  assert.equal(winnerTeamName(docCast, round), 'NAVI');
});

test('second half: sides swapped, the T win goes to the CT starter', () => {
  const round = { half: 2, score_ct_before: 2, score_t_before: 3, winner: 'T' } as const;
  assert.deepEqual(scoreAfter(docCast, round), [4, 2]);
  assert.equal(winnerTeamName(docCast, round), 'NAVI');
});

test('a round without a winner keeps the score', () => {
  const round = { half: 1, score_ct_before: 1, score_t_before: 1, winner: '' } as const;
  assert.deepEqual(scoreAfter(docCast, round), [1, 1]);
  assert.equal(winnerTeamName(docCast, round), '');
});
