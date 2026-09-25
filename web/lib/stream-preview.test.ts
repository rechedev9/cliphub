import test from 'node:test';
import assert from 'node:assert/strict';
import {
  STREAMER_BANNER_MAX_POSITION,
  STREAMER_BANNER_MIN_POSITION,
  clampStreamerBannerPosition,
  keyDropPreviewSourceSeconds,
  resolveStreamerBannerPosition,
} from './stream-preview.ts';

test('KeyDrop preview seeks into the plate window when the playhead is outside it', () => {
  const clips = [{ id: 'c1', start_seconds: 0, end_seconds: 20 }];
  // Default editor frame is mid-source; plate only shows 0–4s.
  assert.equal(keyDropPreviewSourceSeconds(clips, 10, 0, 4), 0);
  // Already inside the window: leave the playhead alone.
  assert.equal(keyDropPreviewSourceSeconds(clips, 2.5, 0, 4), 2.5);
  // Offset window on a later clip.
  const later = [{ id: 'c2', start_seconds: 30, end_seconds: 50 }];
  assert.equal(keyDropPreviewSourceSeconds(later, 40, 1, 3), 31);
});

test('explicit streamer banner position stays absolute across layouts', () => {
  for (const variant of [
    'streamer-vertical-stack-40-60',
    'streamer-vertical-stack',
    'streamer-fullframe-nocam',
  ] as const) {
    assert.equal(resolveStreamerBannerPosition(variant, 0.73), 0.73);
  }
});

test('streamer banner position clamps to keep the strip fully visible', () => {
  assert.equal(clampStreamerBannerPosition(-1), STREAMER_BANNER_MIN_POSITION);
  assert.equal(clampStreamerBannerPosition(STREAMER_BANNER_MIN_POSITION), STREAMER_BANNER_MIN_POSITION);
  assert.equal(clampStreamerBannerPosition(0.5), 0.5);
  assert.equal(clampStreamerBannerPosition(STREAMER_BANNER_MAX_POSITION), STREAMER_BANNER_MAX_POSITION);
  assert.equal(clampStreamerBannerPosition(2), STREAMER_BANNER_MAX_POSITION);
});

test('undefined streamer banner position resets to the current layout default', () => {
  assert.equal(resolveStreamerBannerPosition('streamer-vertical-stack-40-60', undefined), 0.374);
  assert.equal(resolveStreamerBannerPosition('streamer-vertical-stack', undefined), 520 / 1920);
  assert.equal(resolveStreamerBannerPosition('streamer-fullframe-nocam', undefined), 0.2);
});
