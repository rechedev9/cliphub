import test from 'node:test';
import assert from 'node:assert/strict';
import { TACTICAL_EVENT_KINDS } from '../api/tactical.ts';
import type { TacticalEvent } from '../api/tactical.ts';
import type { TimelineEvent } from '../tactical-timeline.ts';
import { GIF_FPS, GIF_MAX_SIDE, GIF_TRANSPARENT, gifCaption, gifFps, gifFrames, gifPalette, gifSize, gifSpan, keepChanged, paletteSample, replayGifFileName } from './replay-gif.ts';
import { WHOLE_ROUND } from './tramo.ts';

const at = (kind: TacticalEvent['kind'], seconds: number): TimelineEvent => ({ event: { tick: seconds * 64, kind, pos: [0, 0, 0], target_pos: [0, 0, 0] }, seconds, fraction: 0 });

test('replay gif: one round records the tramo after its freeze time, superposed rounds on the shared clock', () => {
  const round = { freezeSeconds: 20, durationSeconds: 120, plantSeconds: 80 };
  assert.deepEqual(gifSpan({ ...round, tramo: { from: 55, to: 75 } }), { from: 75, to: 95 });
  assert.deepEqual(gifSpan({ ...round, tramo: WHOLE_ROUND }), { from: 20, to: 120 });
  assert.deepEqual(gifSpan({ ...round, tramo: { from: 0, to: 0, fromPlant: true } }), { from: 80, to: 120 });
  assert.deepEqual(gifSpan({ freezeSeconds: 0, durationSeconds: 90, plantSeconds: null, tramo: { from: 55, to: 115 } }), { from: 55, to: 90 });
});

test('replay gif: nothing to record from the plant of a round without one, or past its end', () => {
  assert.equal(gifSpan({ freezeSeconds: 20, durationSeconds: 120, plantSeconds: null, tramo: { from: 0, to: 0, fromPlant: true } }), null);
  assert.equal(gifSpan({ freezeSeconds: 0, durationSeconds: 40, plantSeconds: null, tramo: { from: 55, to: 75 } }), null);
});

test('replay gif: frames step through the span at the playback speed and the delays add up to its length', () => {
  const frames = gifFrames({ from: 10, to: 30 }, 1);
  assert.equal(frames.length, 20 * GIF_FPS);
  assert.equal(frames[0]?.seconds, 10);
  assert.ok(Math.abs((frames[1]?.seconds ?? 0) - (10 + 1 / GIF_FPS)) < 1e-9);
  assert.equal(frames.reduce((sum, frame) => sum + frame.delayMs, 0), 20_000);
  assert.ok(frames.every((frame) => frame.delayMs % 10 === 0 && frame.delayMs >= 60 && frame.delayMs <= 70));
  const fast = gifFrames({ from: 10, to: 30 }, 2);
  assert.equal(fast.length, 10 * GIF_FPS);
  assert.equal(fast.reduce((sum, frame) => sum + frame.delayMs, 0), 10_000);
});

test('replay gif: a long span lowers the frame rate instead of growing past the frame cap', () => {
  assert.equal(gifFps(20), GIF_FPS);
  assert.equal(gifFps(155), 11);
  assert.ok(gifFrames({ from: 0, to: 600 }, 1).length <= 1800);
});

test('replay gif: the palette keeps 255 picked colours and ends in the transparent slot', () => {
  const palette = gifPalette([[1, 2, 3], [4, 5, 6]]);
  assert.equal(palette.length, 256);
  assert.deepEqual(palette[0], [1, 2, 3]);
  assert.deepEqual(gifPalette(Array.from({ length: 300 }, () => [9, 9, 9]))[GIF_TRANSPARENT], [0, 0, 0]);
});

test('replay gif: the palette sample takes every nth pixel of every frame', () => {
  const frame = new Uint8ClampedArray([1, 1, 1, 255, 2, 2, 2, 255, 3, 3, 3, 255]);
  assert.deepEqual([...paletteSample([frame, frame], 2)], [1, 1, 1, 255, 3, 3, 3, 255, 1, 1, 1, 255, 3, 3, 3, 255]);
});

test('replay gif: only pixels that changed since the last frame are written', () => {
  assert.deepEqual([...keepChanged(new Uint8Array([4, 5, 6]), new Uint8Array([4, 0, 6]))], [GIF_TRANSPARENT, 5, GIF_TRANSPARENT]);
});

test('replay gif: the map keeps its framing and its longer side fits', () => {
  assert.deepEqual(gifSize({ width: 1440, height: 720 }), { width: GIF_MAX_SIDE, height: 360, scale: 0.5 });
  assert.deepEqual(gifSize({ width: 600, height: 400 }), { width: 600, height: 400, scale: 1 });
});

test('replay gif: the caption reads the clock since the round went live, and the bomb once planted', () => {
  const events = [at(TACTICAL_EVENT_KINDS.kill, 40), at(TACTICAL_EVENT_KINDS.plant, 90)];
  assert.equal(gifCaption({ rounds: 'R14', events, seconds: 87, freezeSeconds: 20 }), 'R14 · 1:07');
  assert.equal(gifCaption({ rounds: 'R14', events, seconds: 100, freezeSeconds: 20 }), 'R14 · Bomba 30');
});

test('replay gif: the file name names the map, the teams, the rounds and the tramo, with no reserved characters', () => {
  const span = { from: 75, to: 95 };
  assert.equal(replayGifFileName({ map: 'Mirage', teams: ['FUT', 'BetBoom'], rounds: 'R14', span, freezeSeconds: 20 }), 'Mirage FUT vs BetBoom R14 1-00 a 0-40.gif');
  assert.equal(replayGifFileName({ map: 'Nuke', teams: ['a/b', 'c:d'], rounds: '6 rondas', span: { from: 55, to: 75 }, freezeSeconds: 0 }), 'Nuke a b vs c d 6 rondas 1-00 a 0-40.gif');
  assert.equal(replayGifFileName({ map: '', teams: [], rounds: '', span, freezeSeconds: 20 }), '1-00 a 0-40.gif');
});
