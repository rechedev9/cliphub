import test from 'node:test';
import assert from 'node:assert/strict';
import {
  blankPlan,
  clipOutputDuration,
  clipTimelineGeometry,
  clipsAreValid,
  detectedKillfeedEventCount,
  errorMessage,
  formatStreamTimestamp,
  groupCaptionWords,
  isServiceUnavailable,
  nextClipId,
  nonVideoExtension,
  overlayMarkerGeometry,
  planFingerprint,
  pruneClipEdit,
  streamSourceLabel,
  STREAM_OFFLINE_MESSAGE,
} from './plan.ts';
import type { StreamCaptionWord, StreamClipRange, StreamEditPlan } from '../api/streams.ts';

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

test('a blank plan starts with one valid range and captions off', () => {
  const plan = blankPlan(120);
  assert.equal(plan.clips.length, 1);
  assert.equal(plan.captions?.enabled, false);
  assert.equal(plan.face_crop_reviewed, false);
  assert.equal(clipsAreValid(plan.clips), true);
  assert.equal(clipsAreValid([clip({ end_seconds: 10 })]), false);
  assert.equal(clipsAreValid([]), false);
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
    planFingerprint({ ...base, clips: [clip({ caption_reviewed: true })] }),
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

function word(text: string, start: number, end: number): StreamCaptionWord {
  return { word: text, start_seconds: start, end_seconds: end };
}

test('caption words group into lines on a pause, keeping their flat indices', () => {
  const segments = groupCaptionWords([
    word('menudo', 0, 0.3),
    word('clutch', 0.3, 0.7),
    word('tio', 2, 2.4),
  ]);
  assert.equal(segments.length, 2);
  assert.equal(segments[0].text, 'menudo clutch');
  assert.deepEqual(segments[0].entries.map((entry) => entry.index), [0, 1]);
  assert.deepEqual(segments[1].entries.map((entry) => entry.index), [2]);
  assert.equal(segments[1].startSeconds, 2);
  assert.equal(segments[1].endSeconds, 2.4);
});

test('a sentence-final word also closes the line, and no words means no lines', () => {
  const segments = groupCaptionWords([word('vamos.', 0, 0.4), word('otra', 0.4, 0.8)]);
  assert.deepEqual(segments.map((segment) => segment.text), ['vamos.', 'otra']);
  assert.deepEqual(groupCaptionWords([]), []);
});

test('detected killfeed events are counted across every analysed clip', () => {
  assert.equal(detectedKillfeedEventCount(null), 0);
  assert.equal(
    detectedKillfeedEventCount({
      job_id: 'j',
      generation_id: 'g',
      status: 'applied',
      updated_at: '',
      clips: [
        { clip_id: 'a', start_seconds: 0, end_seconds: 1, events: [] },
        {
          clip_id: 'b',
          start_seconds: 0,
          end_seconds: 1,
          events: [
            {
              event_id: 'e1',
              source_pts: 0,
              time_base: { num: 1, den: 30 },
              cue_seconds: 1,
              onset_start_pts: 0,
              onset_end_pts: 1,
              sample_pts: 1,
              sample_seconds: 1,
              mode: 'aligned_frame',
              rows: [],
              kills: [],
            },
          ],
        },
      ],
    }),
    1,
  );
});
