import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamPlay } from '../api/streams.ts';
import { clipsWithPlays, rankPlays } from './plays.ts';

function play(id: string, start: number, end: number, score: number): StreamPlay {
  return { id, label: id, start_seconds: start, end_seconds: end, kill_seconds: [start + 6], score };
}

test('rankPlays puts the best play first and breaks ties by time', () => {
  const ranked = rankPlays([play('a', 10, 20, 20), play('b', 50, 60, 65), play('c', 5, 9, 20)]);
  assert.deepEqual(
    ranked.map((p) => p.id),
    ['b', 'c', 'a'],
  );
});

test('clipsWithPlays skips plays that overlap a moment and keeps source order', () => {
  let n = 0;
  const clips = clipsWithPlays(
    [{ id: 'mine', start_seconds: 30, end_seconds: 40 }],
    [play('late', 100, 112, 45), play('overlap', 35, 45, 65), play('early', 0, 9, 20)],
    () => `new-${++n}`,
  );
  assert.deepEqual(
    clips.map((c) => [c.id, c.start_seconds, c.title]),
    [
      ['new-2', 0, 'early'],
      ['mine', 30, undefined],
      ['new-1', 100, 'late'],
    ],
  );
});
