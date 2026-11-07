import assert from 'node:assert/strict';
import test from 'node:test';
import { formatVideoDuration, fullDemoDurationSeconds, fullDemoOrder, fullDemoOrderRounds } from './full-demo-order.ts';

test('the sponsor sits after the second round, or after the only one', () => {
  assert.deepEqual(fullDemoOrder([1, 2, 3, 4]).map((part) => part.kind === 'bumper' ? part.slot : part.rounds), ['intro', [1, 2], 'sponsor', [3, 4], 'outro']);
  assert.deepEqual(fullDemoOrder([7]).map((part) => part.kind === 'bumper' ? part.slot : part.rounds), ['intro', [7], 'sponsor', [], 'outro']);
});

test('chosen rounds win over the plan, and the plan over the recap', () => {
  assert.deepEqual(fullDemoOrderRounds({ chosen: [5, 3], planned: [1, 2, 3], recap: [1] }), [3, 5]);
  assert.deepEqual(fullDemoOrderRounds({ chosen: [], planned: [2, 1], recap: [1, 2, 3] }), [1, 2]);
  assert.deepEqual(fullDemoOrderRounds({ chosen: [], planned: null, recap: [3, 1, 3] }), [1, 3]);
});

test('the duration comes from the last frame of the timeline', () => {
  const clock = { source_clock_kind: 'ingame_tick', tick_rate: 64, output_fps: 60, audio_sample_rate: 48000 } as const;
  const entry = { role: 'round', source_ref: 'r1', source_start_tick: 0, source_end_tick: 1, source_offset_frames: 0, start_frame: 0, start_sample: 0, end_sample: 0, reason: '' } as const;
  assert.equal(fullDemoDurationSeconds({ clock, timeline: null }), null);
  assert.equal(fullDemoDurationSeconds({ clock, timeline: [{ ...entry, end_frame: 600 }, { ...entry, end_frame: 148_080 }] }), 2468);
  assert.equal(formatVideoDuration(2468), '41:08');
  assert.equal(formatVideoDuration(3725), '1:02:05');
});
