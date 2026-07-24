// Unit tests for the replay clock. The transport is driven by the wall clock
// rather than by counting animation frames, so these tests pin the property that
// matters on a busy machine: a long or a dropped frame advances the playhead by
// the time that actually passed, and never desynchronises the canvas from the
// timeline.
// Run: node --test "lib/**/*.test.ts"
import test from 'node:test';
import assert from 'node:assert/strict';
import { TACTICAL_EVENT_KINDS, TACTICAL_SIDES } from './api/tactical.ts';
import type { TacticalEvent } from './api/tactical.ts';
import {
  DEFAULT_TICKRATE,
  advanceTransport,
  clampToTimeline,
  roundClockSeconds,
  roundTimeline,
  seekEventSeconds,
  timelineEvents,
  timelineFraction,
  timelineSeconds,
  timelineTick,
} from './tactical-timeline.ts';

const ROUND = {
  tick_start: 1000,
  tick_freeze_end: 1000 + 64 * 20,
  tick_end: 1000 + 64 * 80,
  tick_official: 1000 + 64 * 85,
};

function event(tick: number, kind: TacticalEvent['kind']): TacticalEvent {
  return { tick, kind, pos: [0, 0, 0], target_pos: [0, 0, 0], side: TACTICAL_SIDES.t };
}

test('roundTimeline: the freeze band leads the bar and the round fills the rest', () => {
  const timeline = roundTimeline(ROUND, 64);
  assert.equal(timeline.tickrate, 64);
  assert.equal(timeline.freezeSeconds, 20);
  assert.equal(timeline.durationSeconds, 80);
});

test('roundTimeline: an unusable tick rate falls back to the CS2 default', () => {
  assert.equal(roundTimeline(ROUND, 0).tickrate, DEFAULT_TICKRATE);
  assert.equal(roundTimeline(ROUND, Number.NaN).tickrate, DEFAULT_TICKRATE);
});

test('roundTimeline: a missing round end falls back to the official end', () => {
  const timeline = roundTimeline({ ...ROUND, tick_end: 0 }, 64);
  assert.equal(timeline.endTick, ROUND.tick_official);
  assert.equal(timeline.durationSeconds, 85);
});

test('roundTimeline: a round with no usable end still spans a real second', () => {
  const timeline = roundTimeline({ ...ROUND, tick_end: 0, tick_official: 0 }, 64);
  assert.equal(timeline.durationSeconds, 1);
});

test('roundTimeline: freeze end is clamped inside the bar', () => {
  const timeline = roundTimeline({ ...ROUND, tick_freeze_end: 10 }, 64);
  assert.equal(timeline.freezeSeconds, 0);
});

test('timeline: ticks and seconds convert both ways', () => {
  const timeline = roundTimeline(ROUND, 64);
  assert.equal(timelineSeconds(timeline, ROUND.tick_freeze_end), 20);
  assert.equal(timelineTick(timeline, 20), ROUND.tick_freeze_end);
  assert.equal(timelineFraction(timeline, 40), 0.5);
  assert.equal(timelineFraction(timeline, 1000), 1);
});

test('timeline: the round clock is negative through freeze time', () => {
  const timeline = roundTimeline(ROUND, 64);
  assert.equal(roundClockSeconds(timeline, 0), -20);
  assert.equal(roundClockSeconds(timeline, 20), 0);
  assert.equal(roundClockSeconds(timeline, 35), 15);
});

test('clampToTimeline: keeps a seek inside the bar', () => {
  const timeline = roundTimeline(ROUND, 64);
  assert.equal(clampToTimeline(timeline, -5), 0);
  assert.equal(clampToTimeline(timeline, 500), 80);
  assert.equal(clampToTimeline(timeline, Number.NaN), 0);
});

test('transport: a paused playhead never moves', () => {
  const next = advanceTransport(
    { position: 12, playing: false },
    { elapsedMs: 500, speed: 1, durationSeconds: 80, loop: false },
  );
  assert.deepEqual(next, { position: 12, playing: false });
});

test('transport: advancing follows the wall clock, scaled by speed', () => {
  const next = advanceTransport(
    { position: 10, playing: true },
    { elapsedMs: 250, speed: 2, durationSeconds: 80, loop: false },
  );
  assert.deepEqual(next, { position: 10.5, playing: true });
});

test('transport: a dropped frame advances by the time that actually passed', () => {
  const smooth = Array.from({ length: 10 }).reduce<{ position: number; playing: boolean }>(
    (state) => advanceTransport(state, { elapsedMs: 16, speed: 1, durationSeconds: 80, loop: false }),
    { position: 0, playing: true },
  );
  const stalled = advanceTransport(
    { position: 0, playing: true },
    { elapsedMs: 160, speed: 1, durationSeconds: 80, loop: false },
  );
  assert.ok(Math.abs(smooth.position - stalled.position) < 1e-9);
});

test('transport: reaching the end stops on the last frame without looping', () => {
  const next = advanceTransport(
    { position: 79.9, playing: true },
    { elapsedMs: 1000, speed: 1, durationSeconds: 80, loop: false },
  );
  assert.deepEqual(next, { position: 80, playing: false });
});

test('transport: looping wraps by the whole span after a long stall', () => {
  const next = advanceTransport(
    { position: 70, playing: true },
    { elapsedMs: 15_000, speed: 1, durationSeconds: 80, loop: true },
  );
  assert.equal(next.playing, true);
  assert.ok(Math.abs(next.position - 5) < 1e-9);
});

test('transport: a non-finite or negative step is no time at all', () => {
  const base = { position: 4, playing: true };
  const step = { speed: 1, durationSeconds: 80, loop: false };
  assert.equal(advanceTransport(base, { ...step, elapsedMs: Number.NaN }).position, 4);
  assert.equal(advanceTransport(base, { ...step, elapsedMs: -50 }).position, 4);
});

test('transport: a zero-length round parks the playhead at the start', () => {
  assert.deepEqual(
    advanceTransport({ position: 3, playing: true }, { elapsedMs: 16, speed: 1, durationSeconds: 0, loop: true }),
    { position: 0, playing: false },
  );
});

test('timelineEvents: ordered, placed, and clipped to the bar', () => {
  const timeline = roundTimeline(ROUND, 64);
  const placed = timelineEvents(timeline, [
    event(ROUND.tick_start + 64 * 60, TACTICAL_EVENT_KINDS.kill),
    event(ROUND.tick_start + 64 * 30, TACTICAL_EVENT_KINDS.flash),
    event(ROUND.tick_start + 64 * 200, TACTICAL_EVENT_KINDS.kill),
    event(ROUND.tick_start - 64 * 5, TACTICAL_EVENT_KINDS.smoke),
  ]);
  assert.deepEqual(placed.map((entry) => entry.seconds), [30, 60]);
  assert.equal(placed[0].fraction, 30 / 80);
});

test('seekEventSeconds: steps one event at a time and stops at the ends', () => {
  const timeline = roundTimeline(ROUND, 64);
  const placed = timelineEvents(timeline, [
    event(ROUND.tick_start + 64 * 30, TACTICAL_EVENT_KINDS.kill),
    event(ROUND.tick_start + 64 * 60, TACTICAL_EVENT_KINDS.kill),
  ]);
  assert.equal(seekEventSeconds(placed, 0, 1), 30);
  assert.equal(seekEventSeconds(placed, 30, 1), 60);
  assert.equal(seekEventSeconds(placed, 60, 1), undefined);
  assert.equal(seekEventSeconds(placed, 60, -1), 30);
  assert.equal(seekEventSeconds(placed, 30, -1), undefined);
});
