import assert from 'node:assert/strict';
import { test } from 'node:test';
import { isHalfBreak } from './half-break.ts';

test('a half break is a side swap between two known rounds', () => {
  assert.equal(isHalfBreak({ half: 2 }, { half: 1 }), true);
  assert.equal(isHalfBreak({ half: 3 }, { half: 2 }), false);
  assert.equal(isHalfBreak({ half: 4 }, { half: 3 }), true);
  assert.equal(isHalfBreak({ half: 5 }, { half: 4 }), false);
  assert.equal(isHalfBreak({ half: 3 }, { half: 1 }), true);
  assert.equal(isHalfBreak({ half: 1 }, { half: 1 }), false);
  assert.equal(isHalfBreak(null, { half: 1 }), false);
  assert.equal(isHalfBreak({ half: 2 }, null), false);
});
