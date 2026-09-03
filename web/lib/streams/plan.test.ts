import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamClipRange, StreamEditPlan } from '../api/streams.ts';
import {
  clipOutputDuration,
  clipTimelineGeometry,
  errorMessage,
  fitPlanToSourceDuration,
  withDefaultStreamTitle,
  formatStreamClock,
  formatStreamTimestamp,
  insertClipSorted,
  isServiceUnavailable,
  nextClipId,
  nonVideoExtension,
  planFingerprint,
  pruneClipEdit,
  resolveStreamerBannerPlatform,
  streamSourceLabel,
  timelineClipAt,
  STREAM_INVALID_URL_MESSAGE,
  STREAM_OFFLINE_MESSAGE,
} from './plan.ts';

test('applies job title defaults only to editable plans', () => {
  const plan: StreamEditPlan = {
    schema_version: '1.1',
    variant: 'streamer-vertical-stack-40-60',
    clips: [{ id: 'clip-1', start_seconds: 0, end_seconds: 20, title: '' }],
  };
  const editable = withDefaultStreamTitle(plan, '  Título del trabajo  ', true);
  assert.equal(editable.clips[0].title, 'Título del trabajo');
  assert.equal(plan.clips[0].title, '');

  const rendering = withDefaultStreamTitle(plan, 'Título no renderizado', false);
  assert.equal(rendering, plan);
  assert.equal(rendering.clips[0].title, '');
});

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
  assert.equal(streamSourceLabel('https://kick.com/aimagia/clips/clip_01abc'), 'Kick · aimagia');
  assert.equal(streamSourceLabel('https://www.kick.com/aimagia?clip=clip_01abc'), 'Kick · aimagia');
  assert.equal(streamSourceLabel('https://kick.com/xqc/videos/5c697a87-afce-4256-b01f-3c8fe71ef5cb'), 'Kick · xqc');
  assert.equal(streamSourceLabel('https://youtu.be/abc'), 'YouTube');
  assert.equal(streamSourceLabel('https://vod.example.com/a.mp4'), 'vod.example.com');
  assert.equal(streamSourceLabel(undefined), null);
});

test('banner platform is explicit when set and otherwise follows the source host', () => {
  assert.equal(resolveStreamerBannerPlatform('kick'), 'kick');
  assert.equal(resolveStreamerBannerPlatform('twitch', 'https://kick.com/aimagia/clips/x'), 'twitch');
  assert.equal(resolveStreamerBannerPlatform(undefined, 'https://kick.com/aimagia/clips/x'), 'kick');
  assert.equal(resolveStreamerBannerPlatform(undefined, 'https://clips.twitch.tv/Abc'), 'twitch');
  assert.equal(resolveStreamerBannerPlatform(undefined), 'twitch');
});

test('an offline code wins over the generic fallback message', () => {
  const offline = { code: 'service_unavailable' };
  assert.equal(isServiceUnavailable(offline), true);
  assert.equal(errorMessage(offline, 'fallback'), STREAM_OFFLINE_MESSAGE);
  assert.equal(errorMessage({ code: 'invalid_source_url' }, 'fallback'), STREAM_INVALID_URL_MESSAGE);
  assert.equal(errorMessage(new Error('boom'), 'fallback'), 'boom');
  assert.equal(errorMessage('weird', 'fallback'), 'fallback');
});

test('clip ids are unique so a new range never collides with an existing one', () => {
  assert.notEqual(nextClipId(), nextClipId());
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
  assert.notEqual(
    planFingerprint({ ...base, streamer_banner: { nick: 'aimagia' } }),
    planFingerprint({ ...base, streamer_banner: { nick: 'aimagia', platform: 'kick' } }),
  );
  assert.equal(
    planFingerprint({ ...base, streamer_banner: { nick: 'aimagia' } }),
    planFingerprint({ ...base, streamer_banner: { nick: 'aimagia', platform: 'twitch' } }),
  );
});

test('the fingerprint moves when any KeyDrop plate field that burns into the Short changes', () => {
  const base: StreamEditPlan = {
    schema_version: '1.1',
    variant: 'streamer-vertical-stack-40-60',
    clips: [clip()],
    keydrop_banner: {
      family: 'KEYDROP',
      style: 'classic',
      code: 'ZACKCSGO',
      slide_enabled: false,
      start_seconds: 0,
      end_seconds: 4,
    },
  };
  assert.notEqual(
    planFingerprint(base),
    planFingerprint({ ...base, keydrop_banner: { ...base.keydrop_banner, code: 'OTROCODE' } }),
  );
  assert.notEqual(
    planFingerprint(base),
    planFingerprint({ ...base, keydrop_banner: { ...base.keydrop_banner, style: 'operator' } }),
  );
  assert.notEqual(
    planFingerprint(base),
    planFingerprint({ ...base, keydrop_banner: { ...base.keydrop_banner, slide_enabled: true } }),
  );
  assert.notEqual(
    planFingerprint(base),
    planFingerprint({ ...base, keydrop_banner: { ...base.keydrop_banner, position_y: 0.7 } }),
  );
  assert.notEqual(
    planFingerprint(base),
    planFingerprint({ ...base, keydrop_banner: { ...base.keydrop_banner, end_seconds: 8 } }),
  );
  assert.notEqual(planFingerprint(base), planFingerprint({ ...base, keydrop_banner: { style: '' } }));
  assert.notEqual(
    planFingerprint(base),
    planFingerprint({ ...base, keydrop_banner: { ...base.keydrop_banner, family: 'CSGOSKINS' } }),
  );
  // Case/whitespace on the sponsor code must not create a false mismatch.
  assert.equal(
    planFingerprint(base),
    planFingerprint({ ...base, keydrop_banner: { ...base.keydrop_banner, code: '  zackcsgo  ' } }),
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

test('the clock drops fractional seconds and adds hours only past sixty minutes', () => {
  assert.equal(formatStreamClock(7.9), '0:07');
  assert.equal(formatStreamClock(84), '1:24');
  assert.equal(formatStreamClock(3725), '1:02:05');
  assert.equal(formatStreamClock(Number.NaN), '0:00');
});

test('a timeline click opens a cut around the second, clamped to the source and its neighbours', () => {
  const clips: StreamClipRange[] = [
    { id: 'a', start_seconds: 10, end_seconds: 20 },
    { id: 'b', start_seconds: 40, end_seconds: 50 },
  ];
  const cases: [number, number, ReturnType<typeof timelineClipAt>][] = [
    [30, 90, { start_seconds: 26, end_seconds: 38 }],
    [22, 90, { start_seconds: 20, end_seconds: 30 }],
    [2, 90, { start_seconds: 0, end_seconds: 10 }],
    [88, 90, { start_seconds: 84, end_seconds: 90 }],
    [15, 90, null],
    [30, 0, null],
  ];
  for (const [seconds, duration, expected] of cases) {
    assert.deepEqual(timelineClipAt(clips, seconds, duration), expected);
  }
  const tight: StreamClipRange[] = [
    { id: 'a', start_seconds: 0, end_seconds: 20 },
    { id: 'b', start_seconds: 20.5, end_seconds: 30 },
  ];
  assert.equal(timelineClipAt(tight, 20.2, 90), null);
});

test('a cut clamped to non-decimal neighbours rounds inwards and never overlaps them', () => {
  const clips: StreamClipRange[] = [
    { id: 'a', start_seconds: 0, end_seconds: 3.04 },
    { id: 'b', start_seconds: 5.06, end_seconds: 10 },
  ];
  const cases: [readonly StreamClipRange[], number, ReturnType<typeof timelineClipAt>][] = [
    [clips, 4.5, { start_seconds: 3.1, end_seconds: 5 }],
    [clips, 3.05, { start_seconds: 3.1, end_seconds: 5 }],
    // A gap under the minimum collapses to nothing instead of stealing a frame.
    [
      [{ id: 'a', start_seconds: 0, end_seconds: 3.04 }, { id: 'b', start_seconds: 3.9, end_seconds: 10 }],
      3.5,
      null,
    ],
  ];
  for (const [ranges, seconds, expected] of cases) {
    const range = timelineClipAt(ranges, seconds, 20);
    assert.deepEqual(range, expected);
    if (range !== null) {
      for (const neighbour of ranges) {
        assert.ok(range.end_seconds <= neighbour.start_seconds || range.start_seconds >= neighbour.end_seconds);
      }
    }
  }
});

test('inserted cuts keep source order so numbering follows the timeline', () => {
  const clips: StreamClipRange[] = [{ id: 'b', start_seconds: 40, end_seconds: 50 }];
  const next = insertClipSorted(clips, { id: 'a', start_seconds: 10, end_seconds: 20 });
  assert.deepEqual(next.map((clip) => clip.id), ['a', 'b']);
  assert.equal(clips.length, 1);
});
