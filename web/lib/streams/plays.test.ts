import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamPlay } from '../api/streams.ts';
import { clipsWithPlays, isVoiceOnly, playMoment, rankPlays, sameIds, summaryPlayIds } from './plays.ts';

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

test('rankPlays puts a marked voice-only play below a kill with a shout, not above every ace', () => {
  const marked: StreamPlay = { ...play('shout', 90, 100, 30), kind: 'voice_peak', kill_seconds: [], voice_seconds: [92], marked: true };
  const ranked = rankPlays([play('ace', 10, 20, 100), marked, play('plain', 30, 40, 20), play('loud', 50, 60, 35)]);
  assert.deepEqual(
    ranked.map((p) => p.id),
    ['ace', 'loud', 'shout', 'plain'],
  );
});

test('playMoment points at the first kill, or at the voice peak of a play with no kill', () => {
  const shout: StreamPlay = { ...play('shout', 90, 100, 40), kind: 'voice_peak', kill_seconds: [], voice_seconds: [92] };
  assert.equal(playMoment(play('kill', 10, 20, 20)), 16);
  assert.equal(playMoment(shout), 92);
  assert.equal(isVoiceOnly(shout), true);
  assert.equal(isVoiceOnly(play('kill', 10, 20, 20)), false);
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

test('summaryPlayIds reads the chosen plays and treats an old full summary as every play', () => {
  const plays = [play('a', 10, 20, 20), play('b', 50, 60, 65)];
  assert.deepEqual(summaryPlayIds({ status: 'done', progress: 1, play_count: 1, play_ids: ['b'] }, plays), ['b']);
  assert.deepEqual(summaryPlayIds({ status: 'done', progress: 1, play_count: 2 }, plays), ['a', 'b']);
  assert.equal(summaryPlayIds({ status: 'done', progress: 1, play_count: 3 }, plays), null);
  assert.equal(summaryPlayIds({ status: 'idle', progress: 0 }, plays), null);
});

test('sameIds ignores order but not membership', () => {
  assert.equal(sameIds(['a', 'b'], ['b', 'a']), true);
  assert.equal(sameIds(['a', 'b'], ['a']), false);
  assert.equal(sameIds(['a'], ['b']), false);
});
