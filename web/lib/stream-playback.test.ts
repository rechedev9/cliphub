import assert from 'node:assert/strict';
import test from 'node:test';
import {
  nextStreamPlaybackIndex,
  streamAffiliateSlide,
  streamAffiliateWindow,
  streamBannerSlide,
  streamClipRange,
  streamFadeEnvelope,
  streamPlaybackIndex,
  streamPreviewFade,
  STREAM_PLAYBACK_MODE,
} from './stream-playback.ts';

const clips = [
  { id: 'later', start_seconds: 20, end_seconds: 30 },
  { id: 'first', start_seconds: 0, end_seconds: 10 },
  { id: 'overlap', start_seconds: 5, end_seconds: 15 },
];

test('clip identity selects overlapping and reordered cuts without time-based ambiguity', () => {
  assert.equal(streamPlaybackIndex(clips, 'overlap'), 2);
  assert.equal(streamPlaybackIndex(clips, 'later'), 0);
  assert.equal(nextStreamPlaybackIndex(clips, 0, STREAM_PLAYBACK_MODE.sequence, false), 1);
  assert.equal(nextStreamPlaybackIndex(clips, 2, STREAM_PLAYBACK_MODE.sequence, false), -1);
  assert.equal(nextStreamPlaybackIndex(clips, 2, STREAM_PLAYBACK_MODE.sequence, true), 0);
  assert.equal(nextStreamPlaybackIndex(clips, 2, STREAM_PLAYBACK_MODE.selected, true), 2);
});

test('invalid or removed cuts are skipped rather than replaying an excluded range', () => {
  assert.equal(streamPlaybackIndex([], null), -1);
  assert.equal(streamPlaybackIndex(clips, 'removed'), 0);
  assert.equal(streamClipRange({ id: 'bad', start_seconds: 5, end_seconds: 2 }), null);
  assert.equal(streamClipRange({ id: 'bad', start_seconds: NaN, end_seconds: 2 }), null);
});

test('visual fades follow output seconds after speed adjustment', () => {
  const clip = { id: 'x', start_seconds: 10, end_seconds: 20, edit: { speed: 2, fade_in_seconds: 1, fade_out_seconds: 1 } };
  for (const [sourceTime, opacity] of [[10, 0], [11, 0.5], [12, 1], [18, 1], [19, 0.5], [20, 0]]) {
    assert.equal(streamPreviewFade(clip, sourceTime), opacity);
  }
});

test('overlapping FFmpeg fades multiply rather than selecting the lower envelope', () => {
  assert.equal(streamFadeEnvelope(2, 4, 3, 3), 4 / 9);
  assert.equal(streamFadeEnvelope(1, 4, 3, 3), 1 / 3);
  assert.equal(streamFadeEnvelope(3, 4, 3, 3), 1 / 3);
  assert.equal(streamFadeEnvelope(-1, 4, 1, 1), 0);
  assert.equal(streamFadeEnvelope(5, 4, 1, 1), 0);
});

test('banner slide uses the render transition interval in source time', () => {
  assert.equal(streamBannerSlide(0, 10), -100);
  assert.equal(streamBannerSlide(0.175, 10), -50);
  assert.equal(streamBannerSlide(1, 10), 0);
  assert.ok(Math.abs(streamBannerSlide(9.825, 10) + 50) < 0.00001);
  assert.equal(streamBannerSlide(0.05, 0.2), -50);
});

test('affiliate visibility window uses the Go renderer clamp rules', () => {
  assert.deepEqual(streamAffiliateWindow(1.5, 5, 15), { start: 1.5, end: 5 });
  assert.deepEqual(streamAffiliateWindow(-1, 0, 15), { start: 0, end: 15 });
  assert.deepEqual(streamAffiliateWindow(20, 30, 15), { start: 0, end: 15 });
  assert.deepEqual(streamAffiliateWindow(8, 4, 15), { start: 8, end: 15 });
  assert.deepEqual(streamAffiliateWindow(Number.NaN, Number.POSITIVE_INFINITY, 15), { start: 0, end: 15 });
  assert.deepEqual(streamAffiliateWindow(0, 1, 0), { start: 0, end: 0 });
});

test('affiliate slide mirrors the centered 55-percent plate x equation', () => {
  const slide = (time: number) => streamAffiliateSlide(time, 1.5, 5, 15);
  assert.ok(Math.abs(slide(1.5) - -140.9090909090909) < 0.000001);
  assert.ok(Math.abs(slide(1.675) - -120.45454545454545) < 0.000001);
  // Go uses lt(): at the entry boundary it switches to the centered hold.
  assert.equal(slide(1.85), 0);
  assert.equal(slide(4.65), 0);
  assert.ok(Math.abs(slide(4.825) + 50) < 0.000001);
  assert.ok(Math.abs(slide(5) + 100) < 0.000001);
  // A short window clamps each phase to half its duration.
  assert.ok(Math.abs(streamAffiliateSlide(0.05, 0, 0.2, 0.2) + 120.45454545454545) < 0.000001);
  assert.equal(streamAffiliateSlide(0.1, 0, 0.2, 0.2), 0);
  assert.ok(Number.isFinite(streamAffiliateSlide(Number.NaN, 0, 5, 5)));
});
