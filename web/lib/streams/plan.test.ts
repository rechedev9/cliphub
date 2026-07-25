import test from 'node:test';
import assert from 'node:assert/strict';
import {
  blankPlan,
  clipOutputDuration,
  clipTimelineGeometry,
  clipsAreValid,
  errorMessage,
  fitPlanToSourceDuration,
  formatStreamTimestamp,
  initialStreamClipEnd,
  isServiceUnavailable,
  nextClipId,
  nonVideoExtension,
  overlayMarkerGeometry,
  planFingerprint,
  pruneClipEdit,
  streamSourceLabel,
  STREAM_OFFLINE_MESSAGE,
} from './plan.ts';
import type { StreamClipRange, StreamEditPlan } from '../api/streams.ts';

function clip(overrides: Partial<StreamClipRange> = {}): StreamClipRange {
  return { id: 'clip-1', start_seconds: 10, end_seconds: 20, ...overrides };
}

test('non-video URLs are rejected by extension, videos and junk are left to the server', () => {
  assert.equal(nonVideoExtension('https://i.imgur.com/a.PNG'), 'png');
  assert.equal(nonVideoExtension('https://example.com/clip.mp4'), null);
  assert.equal(nonVideoExtension('https://clips.twitch.tv/SomeClipSlug'), null);
  assert.equal(nonVideoExtension('not a url at all'), null);
});

test('the source label names the Twitch channel when the clip URL carries one', () => {
  assert.equal(streamSourceLabel('https://www.twitch.tv/zacketizor/clip/AbcDef'), 'Twitch · zacketizor');
  assert.equal(streamSourceLabel('https://clips.twitch.tv/AbcDef'), 'Twitch');
  assert.equal(streamSourceLabel('https://youtu.be/abc'), 'YouTube');
  assert.equal(streamSourceLabel('https://vod.example.com/a.mp4'), 'vod.example.com');
  assert.equal(streamSourceLabel(undefined), null);
});

test('an offline code wins over the generic fallback message', () => {
  const offline = { code: 'service_unavailable' };
  assert.equal(isServiceUnavailable(offline), true);
  assert.equal(errorMessage(offline, 'fallback'), STREAM_OFFLINE_MESSAGE);
  assert.equal(errorMessage(new Error('boom'), 'fallback'), 'boom');
  assert.equal(errorMessage('weird', 'fallback'), 'fallback');
});

test('clip ids are unique so a new range never collides with an existing one', () => {
  assert.notEqual(nextClipId(), nextClipId());
});

test('a blank plan starts with one valid range awaiting facecam review', () => {
  const plan = blankPlan(120);
  assert.equal(plan.clips.length, 1);
  assert.equal(plan.face_crop_reviewed, false);
  assert.equal(clipsAreValid(plan.clips), true);
  assert.equal(clipsAreValid([clip({ end_seconds: 10 })]), false);
  assert.equal(clipsAreValid([]), false);
});

test('a fresh range keeps the 20-second default while respecting short sources', () => {
  assert.equal(initialStreamClipEnd(15.15), 15.15);
  assert.equal(initialStreamClipEnd(120), 20);
  assert.equal(initialStreamClipEnd(0), 20);
  assert.equal(initialStreamClipEnd(Number.NaN), 20);
});

test('fitting to the source clamps legacy endpoints and upgrades the schema version', () => {
  const plan: StreamEditPlan = {
    schema_version: '1.0',
    variant: 'streamer-vertical-stack-40-60',
    clips: [
      { id: 'legacy', start_seconds: 0, end_seconds: 20 },
      { id: 'beyond-eof', start_seconds: 30, end_seconds: 20 },
    ],
  };
  const fitted = fitPlanToSourceDuration(plan, 15.15);
  assert.equal(fitted.schema_version, '1.1');
  assert.deepEqual(fitted.clips.map((c) => [c.id, c.end_seconds]), [['legacy', 15.15]]);
});

test('fitting preserves custom overruns for strict backend validation', () => {
  const plan: StreamEditPlan = {
    schema_version: '1.1',
    variant: 'streamer-vertical-stack-40-60',
    clips: [{ id: 'custom', start_seconds: 0, end_seconds: 42 }],
  };
  assert.equal(fitPlanToSourceDuration(plan, 15.15).clips[0].end_seconds, 42);
  assert.equal(fitPlanToSourceDuration(plan, 0).clips[0].end_seconds, 42);
});

test('timestamps render as m:ss.hh with a padded seconds field', () => {
  assert.equal(formatStreamTimestamp(0), '0:00.00');
  assert.equal(formatStreamTimestamp(65.5), '1:05.50');
  assert.equal(formatStreamTimestamp(Number.NaN), '0:00.00');
  assert.equal(formatStreamTimestamp(-4), '0:00.00');
});

test('an all-defaults edit prunes to undefined so the fingerprint does not move', () => {
  assert.equal(pruneClipEdit({ speed: 1, source_volume: 1, fade_in_seconds: 0 }), undefined);
  assert.deepEqual(pruneClipEdit({ speed: 1.5 }), { speed: 1.5 });

  const base: StreamEditPlan = { schema_version: '1.1', variant: 'streamer-vertical-stack-40-60', clips: [clip()] };
  const withDefaults: StreamEditPlan = {
    ...base,
    clips: [clip({ edit: { speed: 1, source_volume: 1, fade_in_seconds: 0, fade_out_seconds: 0 } })],
  };
  assert.equal(planFingerprint(base), planFingerprint(withDefaults));
});

test('the fingerprint moves when a rendered field changes and not when updated_at does', () => {
  const base: StreamEditPlan = { schema_version: '1.1', variant: 'streamer-vertical-stack-40-60', clips: [clip()] };
  assert.equal(planFingerprint(base), planFingerprint({ ...base, updated_at: '2026-07-24T00:00:00Z' }));
  assert.notEqual(planFingerprint(base), planFingerprint({ ...base, music: { key: 'a', volume: 0.25 } }));
  assert.notEqual(
    planFingerprint(base),
    planFingerprint({ ...base, clips: [clip({ title: 'clutch' })] }),
  );
});

test('the clip band is positioned against the probed duration and skipped without one', () => {
  const geometry = clipTimelineGeometry(clip({ start_seconds: 25, end_seconds: 50 }), 100);
  assert.deepEqual(
    geometry && { start: geometry.startPercent, width: geometry.widthPercent },
    { start: 25, width: 25 },
  );
  assert.equal(clipTimelineGeometry(clip(), 0), null);
  assert.equal(clipTimelineGeometry(clip({ end_seconds: 5 }), 100), null);
});

test('fade wedges are measured against the output duration, so speed shrinks them', () => {
  const fast = clipTimelineGeometry(
    clip({ start_seconds: 0, end_seconds: 20, edit: { speed: 2, fade_in_seconds: 1 } }),
    100,
  );
  // 20 source seconds at 2x is 10 output seconds; a 1s fade is 10% of the clip.
  assert.equal(fast?.fadeInPercent, 10);
  assert.equal(clipOutputDuration(clip({ start_seconds: 0, end_seconds: 20, edit: { speed: 2 } })), 10);
});

test('an overlay without bounds spans its whole clip', () => {
  assert.deepEqual(overlayMarkerGeometry({ text: 'GG', position_y: 0.5 }, 10), {
    startPercent: 0,
    widthPercent: 100,
  });
  assert.deepEqual(
    overlayMarkerGeometry({ text: 'GG', position_y: 0.5, start_seconds: 2, end_seconds: 4 }, 10),
    { startPercent: 20, widthPercent: 20 },
  );
  assert.equal(overlayMarkerGeometry({ text: 'GG', position_y: 0.5 }, 0), null);
});
