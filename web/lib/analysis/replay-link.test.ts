import assert from 'node:assert/strict';
import test from 'node:test';
import { parseReplayRefs, replayHref } from './replay-link.ts';

const A = '11111111-1111-4111-8111-111111111111';

test('replay links round-trip their rounds and drop malformed ones', () => {
  const href = replayHref([{ matchId: A, round: 3 }, { matchId: A, round: 9, tick: 4100.4 }], 'Clutches');
  const query = new URLSearchParams(href.split('?')[1]);
  query.append('r', 'nope:1');
  query.append('r', `${A}:0`);
  assert.deepEqual(parseReplayRefs(query), [{ matchId: A, round: 3 }, { matchId: A, round: 9, tick: 4100 }]);
  assert.equal(query.get('t'), 'Clutches');
});
