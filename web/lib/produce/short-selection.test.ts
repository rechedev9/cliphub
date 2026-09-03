import test from 'node:test';
import assert from 'node:assert/strict';
import type { Play } from '../api/types.ts';
import {
  autoPickBestPlays,
  estimatedPlaySeconds,
  estimatedSelectionSeconds,
  formatClock,
  roundsSummary,
  SHORT_TARGET_SECONDS,
} from './short-selection.ts';

function play(id: string, round: number, kills: number): Play {
  return { id, matchId: 'm', label: `R${round}`, kind: kills >= 3 ? 'highlight' : 'clean', round, kills };
}

test('estimated duration grows with kills from a fixed base', () => {
  const cases = [
    { kills: 0, want: 6 },
    { kills: 1, want: 9 },
    { kills: 3, want: 15 },
    { kills: 5, want: 21 },
  ];
  for (const { kills, want } of cases) {
    assert.equal(estimatedPlaySeconds({ kills }), want, `${kills} kills`);
  }
  assert.equal(estimatedSelectionSeconds([{ kills: 1 }, { kills: 3 }]), 24);
});

test('formatClock renders m:ss', () => {
  const cases = [
    { seconds: 0, want: '0:00' },
    { seconds: 47, want: '0:47' },
    { seconds: 60, want: '1:00' },
    { seconds: 75.4, want: '1:15' },
    { seconds: -3, want: '0:00' },
  ];
  for (const { seconds, want } of cases) {
    assert.equal(formatClock(seconds), want, String(seconds));
  }
});

test('auto pick takes the biggest frags first and never exceeds the target', () => {
  const plays = [play('a', 3, 1), play('b', 7, 5), play('c', 12, 3), play('d', 19, 4), play('e', 21, 2)];
  const picked = autoPickBestPlays(plays);
  // 5K (21s) + 4K (18s) + 3K (15s) = 54s; both the 2K (12s) and the 1K (9s) would overflow.
  assert.deepEqual([...picked].sort(), ['b', 'c', 'd']);
  const total = estimatedSelectionSeconds(plays.filter((p) => picked.has(p.id)));
  assert.ok(total <= SHORT_TARGET_SECONDS);
});

test('auto pick keeps plan order among equal kill counts', () => {
  const plays = [play('x', 1, 2), play('y', 2, 2), play('z', 3, 2)];
  const picked = autoPickBestPlays(plays, 24);
  assert.deepEqual([...picked], ['x', 'y']);
  assert.deepEqual([...autoPickBestPlays([], 60)], []);
});

test('rounds summary lists selected rounds in plan order', () => {
  assert.equal(roundsSummary([play('a', 7, 1), play('b', 12, 2), play('c', 19, 1)]), 'R7 · R12 · R19');
  assert.equal(roundsSummary([]), '');
});

test('auto pick always keeps the top-ranked play when nothing fits the target', () => {
  const cases: Array<{ name: string; plays: Play[]; target: number; want: string[] }> = [
    { name: 'single overlong play', plays: [play('a', 4, 9)], target: 10, want: ['a'] },
    {
      name: 'every play overruns: the biggest frag wins',
      plays: [play('a', 4, 5), play('b', 9, 7), play('c', 2, 6)],
      target: 12,
      want: ['b'],
    },
    { name: 'no plays at all', plays: [], target: 60, want: [] },
  ];
  for (const tc of cases) {
    assert.deepEqual([...autoPickBestPlays(tc.plays, tc.target)], tc.want, tc.name);
  }
});
