import assert from 'node:assert/strict';
import { test } from 'node:test';
import { ZONE_MARKS_MAX, zonesFromQuery } from './zone-marks.ts';

test('a link marks its zones once each, without control characters or overlong tokens', () => {
  const params = new URLSearchParams();
  for (const zone of ['BombsiteA', 'BombsiteA', 'Bad\u0007', 'x'.repeat(65), 'Middle']) params.append('zona', zone);
  assert.deepEqual(zonesFromQuery(params), ['BombsiteA', 'Middle']);
  const many = new URLSearchParams(Array.from({ length: 20 }, (_, i) => ['zona', `Z${i}`]));
  assert.equal(zonesFromQuery(many).length, ZONE_MARKS_MAX);
});
