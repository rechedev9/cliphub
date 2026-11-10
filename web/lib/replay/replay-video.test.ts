import test from 'node:test';
import assert from 'node:assert/strict';
import { VIDEO_FPS, replayVideoFileName, videoFrameTimes, videoSeconds, videoSize } from './replay-video.ts';

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
