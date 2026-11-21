import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamClipRange } from '../api/streams.ts';
import { fullVideoMoment, markMomentAt } from './mark-moment.ts';

function moment(id: string, start: number, end: number): StreamClipRange {
  return { id, start_seconds: start, end_seconds: end };
}

test('marking in open video creates a momento from 5 s before the playhead to 10 s after it', () => {
  assert.deepEqual(markMomentAt({ clips: [], seconds: 600, sourceDuration: 3600 }), {
    kind: 'created',
    range: { start_seconds: 595, end_seconds: 610 },
  });
});

test('a marked momento stops at its neighbours and at the video bounds', () => {
  const clips = [moment('a', 100, 197), moment('b', 205, 230)];
  assert.deepEqual(markMomentAt({ clips, seconds: 199, sourceDuration: 600 }), {
    kind: 'created',
    range: { start_seconds: 197, end_seconds: 205 },
  });
  assert.deepEqual(markMomentAt({ clips: [], seconds: 2, sourceDuration: 600 }), {
    kind: 'created',
    range: { start_seconds: 0, end_seconds: 12 },
  });
  assert.deepEqual(markMomentAt({ clips: [], seconds: 598, sourceDuration: 600 }), {
    kind: 'created',
    range: { start_seconds: 593, end_seconds: 600 },
  });
});

test('marking inside a momento selects it and creates nothing', () => {
  const clips = [moment('a', 100, 130), moment('b', 200, 230)];
  assert.deepEqual(markMomentAt({ clips, seconds: 200, sourceDuration: 600 }), { kind: 'inside', clip: clips[1] });
  assert.deepEqual(markMomentAt({ clips, seconds: 129.5, sourceDuration: 600 }), { kind: 'inside', clip: clips[0] });
});

test('marking where the gap is shorter than the minimum creates nothing', () => {
  const clips = [moment('a', 100, 130), moment('b', 131, 160)];
  assert.deepEqual(markMomentAt({ clips, seconds: 130.5, sourceDuration: 600 }), { kind: 'no-room' });
  assert.deepEqual(markMomentAt({ clips: [], seconds: 0, sourceDuration: 0 }), { kind: 'no-room' });
});

test('a source of three minutes or less with no momentos offers the whole video as one momento', () => {
  assert.deepEqual(fullVideoMoment({ clips: [], sourceDuration: 160 }), { start_seconds: 0, end_seconds: 160 });
  assert.deepEqual(fullVideoMoment({ clips: [], sourceDuration: 180 }), { start_seconds: 0, end_seconds: 180 });
});

test('the whole video is not offered past three minutes, before the video loads or once a momento exists', () => {
  assert.equal(fullVideoMoment({ clips: [], sourceDuration: 181 }), null);
  assert.equal(fullVideoMoment({ clips: [], sourceDuration: 0 }), null);
  assert.equal(fullVideoMoment({ clips: [moment('a', 10, 20)], sourceDuration: 160 }), null);
});
