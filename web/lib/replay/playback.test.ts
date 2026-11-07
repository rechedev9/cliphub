import test from 'node:test';
import assert from 'node:assert/strict';
import { SCRUB_STEP_SECONDS, playStartSeconds, playbackStep } from './playback.ts';

const frame = { position: 10, elapsedMs: 16, speed: 1, durationSeconds: 112.7, loop: false, scrubbing: false, seeked: false, hasNext: true };

test('playback: plays on inside the round', () => {
  const step = playbackStep(frame);
  assert.equal(step.action, 'play');
  assert.ok(Math.abs(step.position - 10.016) < 1e-9);
});

test('playback: a round played into its end moves on, even after a long stall', () => {
  assert.deepEqual(playbackStep({ ...frame, position: 112.69 }), { position: 112.7, action: 'next' });
  assert.deepEqual(playbackStep({ ...frame, elapsedMs: 600_000 }), { position: 112.7, action: 'next' });
  assert.equal(playbackStep({ ...frame, position: 112.69, speed: 4 }).action, 'next');
});

test('playback: a held scrubber owns the playhead, even at the very end', () => {
  assert.deepEqual(playbackStep({ ...frame, scrubbing: true, position: 112.7 }), { position: 112.7, action: 'hold' });
  assert.deepEqual(playbackStep({ ...frame, scrubbing: true, seeked: true, position: 50 }), { position: 50, action: 'hold' });
});

test('playback: a seek to the end stops there, also one scrubber step short of it', () => {
  assert.equal(playbackStep({ ...frame, seeked: true, position: 112.7 }).action, 'stop');
  assert.equal(playbackStep({ ...frame, seeked: true, position: 112.7 - SCRUB_STEP_SECONDS / 2 }).action, 'stop');
});

test('playback: a seek inside the round keeps playing', () => {
  assert.equal(playbackStep({ ...frame, seeked: true, position: 60 }).action, 'play');
});

test('playback: the last round, multi round and an empty round stop', () => {
  assert.equal(playbackStep({ ...frame, hasNext: false, position: 112.7 }).action, 'stop');
  assert.deepEqual(playbackStep({ ...frame, durationSeconds: 0, hasNext: false }), { position: 0, action: 'stop' });
});

test('playback: the loop wraps the round instead of moving on', () => {
  const step = playbackStep({ ...frame, loop: true, seeked: true, position: 112.69, elapsedMs: 20 });
  assert.equal(step.action, 'play');
  assert.ok(step.position < 1);
});

test('playback: play restarts a round parked at its end', () => {
  assert.equal(playStartSeconds(112.7, 112.7), 0);
  assert.equal(playStartSeconds(112.7 - SCRUB_STEP_SECONDS / 2, 112.7), 0);
  assert.equal(playStartSeconds(100, 112.7), 100);
});
