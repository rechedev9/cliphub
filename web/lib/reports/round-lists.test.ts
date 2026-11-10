import test from 'node:test';
import assert from 'node:assert/strict';
import { parseReplayRefs } from '../analysis/replay-link.ts';
import { roundListHref } from './round-lists.ts';

const A = '0b6c1d2e-3f40-4a5b-8c6d-7e8f9a0b1c2d';
const B = '1c7d2e3f-4051-4b6c-9d7e-8f9a0b1c2d3e';

test('a round list opens its rounds in order under its title', () => {
  const href = roundListHref({ title: 'Split B en full buy', rounds: [{ match_id: B, round: 4 }, { match_id: A, round: 12 }] });
  const [path, query = ''] = href.split('?');
  const params = new URLSearchParams(query);
  assert.equal(path, '/replay');
  assert.equal(params.get('t'), 'Split B en full buy');
  assert.deepEqual(parseReplayRefs(params), [{ matchId: B, round: 4 }, { matchId: A, round: 12 }]);
});
