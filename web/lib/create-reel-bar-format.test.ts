import test from 'node:test';
import assert from 'node:assert/strict';
import { lockedFormatLabel } from './reel-format.ts';

test('locked format label names the locked delivery', () => {
  const cases: Array<{ format: 'short-9x16' | 'landscape-16x9'; want: string }> = [
    { format: 'landscape-16x9', want: '16:9' },
    { format: 'short-9x16', want: '9:16' },
  ];
  for (const tc of cases) {
    assert.equal(lockedFormatLabel(tc.format), tc.want);
  }
});
