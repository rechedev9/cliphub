import assert from 'node:assert/strict';
import { test } from 'node:test';
import { isHalfBreak } from './half-break.ts';

test('a half break needs both rounds known and a different half', () => {
  assert.equal(isHalfBreak({ half: 2 }, { half: 1 }), true);
  assert.equal(isHalfBreak({ half: 1 }, { half: 1 }), false);
  assert.equal(isHalfBreak(null, { half: 1 }), false);
  assert.equal(isHalfBreak({ half: 2 }, null), false);
});
