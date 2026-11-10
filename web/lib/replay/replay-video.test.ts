import test from 'node:test';
import assert from 'node:assert/strict';
import { VIDEO_FPS, VIDEO_SPEEDS, replayVideoFileName, videoBlocked, videoFrameTimes, videoSeconds, videoSize, voiceCovers, voiceEndTick } from './replay-video.ts';

test('replay video: every transport speed passes, an unknown speed and a tramo over 120 s are refused before drawing', () => {
  for (const speed of VIDEO_SPEEDS) assert.equal(videoBlocked({ span: { from: 0, to: 20 }, speed }), null);
  assert.match(videoBlocked({ span: { from: 0, to: 20 }, speed: 3 }) ?? '', /velocidad/);
  assert.match(videoBlocked({ span: { from: 0, to: 40 }, speed: 0.25 }) ?? '', /120 s/);
  assert.equal(videoBlocked({ span: { from: 0, to: 240 }, speed: 2 }), null);
});

test('replay video: "Solo tiempo de compra" ends the voice at freeze end, inside the tramo', () => {
  assert.equal(voiceEndTick({ fromTick: 100, toTick: 900, freezeOnly: false, freezeEndTick: 500 }), 900);
  assert.equal(voiceEndTick({ fromTick: 100, toTick: 900, freezeOnly: true, freezeEndTick: 500 }), 500);
  assert.equal(voiceEndTick({ fromTick: 600, toTick: 900, freezeOnly: true, freezeEndTick: 500 }), 600);
  assert.equal(voiceEndTick({ fromTick: 100, toTick: 900, freezeOnly: true, freezeEndTick: null }), 900);
});

test('replay video: the voice covers a tramo when any track recorded part of it at the offset heard', () => {
  // 64 tick/s: ticks 6400 to 12800 are demo seconds 100 to 200.
  const at = { fromTick: 6400, toTick: 12800, tickrate: 64 };
  assert.equal(voiceCovers({ ...at, offsetMs: 0, durationsMs: [150_000] }), true);
  assert.equal(voiceCovers({ ...at, offsetMs: 0, durationsMs: [50_000] }), false);
  assert.equal(voiceCovers({ ...at, offsetMs: 0, durationsMs: [50_000, 120_000] }), true);
  assert.equal(voiceCovers({ ...at, offsetMs: 250_000, durationsMs: [600_000] }), false);
  assert.equal(voiceCovers({ ...at, offsetMs: 120_000, durationsMs: [30_000] }), true);
  assert.equal(voiceCovers({ ...at, toTick: at.fromTick, offsetMs: 0, durationsMs: [600_000] }), false);
});

test('replay video: a 20 s tramo takes 600 frames at 30 fps, and speed 2 halves them', () => {
  const span = { from: 35, to: 55 };
  assert.equal(videoSeconds(span, 1), 20);
  const frames = videoFrameTimes(span, 1);
  assert.equal(frames.length, 20 * VIDEO_FPS);
  assert.equal(frames[0], 35);
  assert.ok(Math.abs((frames.at(-1) ?? 0) - (55 - 1 / VIDEO_FPS)) < 1e-9);
  const fast = videoFrameTimes(span, 2);
  assert.equal(fast.length, 10 * VIDEO_FPS);
  assert.ok(Math.abs((fast[1] ?? 0) - (35 + 2 / VIDEO_FPS)) < 1e-9);
});

test('replay video: the size keeps the map canvas, with even sides and the long side capped', () => {
  assert.deepEqual(videoSize({ width: 1001, height: 777 }, { width: 500 }), { width: 1000, height: 776, scale: 2 });
  const big = videoSize({ width: 3840, height: 2160 }, { width: 1920 });
  assert.equal(big.width, 1920);
  assert.equal(big.height, 1080);
  assert.equal(big.scale, 1);
});

test('replay video: the file is named like the GIF, as an MP4', () => {
  const name = replayVideoFileName({ map: 'Mirage', teams: ['FUT', 'BetBoom'], rounds: 'R14', span: { from: 35, to: 75 }, freezeSeconds: 15 });
  assert.equal(name, 'Mirage FUT vs BetBoom R14 1-35 a 0-55.mp4');
});
