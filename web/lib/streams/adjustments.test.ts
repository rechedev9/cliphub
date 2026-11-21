import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamEditPlan } from '../api/streams.ts';
import { EDIT_PLAN_SCHEMA_VERSION } from './plan.ts';
import { fadeChoices, shortMoments, textTimingPatch, textWindow, withMomentEdit } from './adjustments.ts';

function plan(overrides: Partial<StreamEditPlan> = {}): StreamEditPlan {
  return {
    schema_version: EDIT_PLAN_SCHEMA_VERSION,
    variant: 'streamer-vertical-stack-40-60',
    clips: [
      { id: 'c1', start_seconds: 10, end_seconds: 20, title: 'TRIPLE KILL' },
      { id: 'c2', start_seconds: 30, end_seconds: 40 },
      { id: 'c3', start_seconds: 50, end_seconds: 79, edit: { speed: 1.5 } },
    ],
    ...overrides,
  };
}

test('a Short of one momento shows that momento alone', () => {
  const moments = shortMoments(plan(), 'c2');
  assert.deepEqual(
    moments.map((m) => [m.clip.id, m.name]),
    [['c2', 'Momento 2']],
  );
});

test('a joined Short shows each of its momentos separately, in the order it plays them', () => {
  const joined = plan({ groups: [{ id: 'g1', clip_ids: ['c3', 'c1'] }] });
  assert.deepEqual(
    shortMoments(joined, 'g1').map((m) => [m.clip.id, m.name]),
    [
      ['c3', 'Momento 3'],
      ['c1', 'TRIPLE KILL'],
    ],
  );
});

test('an unknown Short shows no momentos', () => {
  assert.deepEqual(shortMoments(plan(), 'missing'), []);
});

test('an adjustment changes only its own momento and keeps the values it had', () => {
  const next = withMomentEdit(plan(), { clipId: 'c3', patch: { source_volume: 0 } });
  assert.deepEqual(next.clips[2].edit, { speed: 1.5, source_volume: 0 });
  assert.deepEqual(next.clips[0], plan().clips[0]);
  assert.equal(next.clips[1].edit, undefined);
});

test('setting an adjustment back to its default leaves the momento with no edit', () => {
  const next = withMomentEdit(plan(), { clipId: 'c3', patch: { speed: 1 } });
  assert.equal(next.clips[2].edit, undefined);
});

test('a fade is picked from a few lengths, none typed', () => {
  assert.deepEqual(
    fadeChoices({ current: 0, other: 0, outputSeconds: 10 }).map((c) => [c.seconds, c.label]),
    [
      [0, 'Sin fundido'],
      [0.5, '0,5 s'],
      [1, '1 s'],
      [2, '2 s'],
    ],
  );
});

test('a fade saved with another length before this change is still offered and kept', () => {
  assert.deepEqual(
    fadeChoices({ current: 1.3, other: 0, outputSeconds: 10 }).map((c) => c.label),
    ['Sin fundido', '0,5 s', '1 s', '1,3 s', '2 s'],
  );
});

test('a fade that would not fit beside the other one in the momento is not available', () => {
  const choices = fadeChoices({ current: 0, other: 1, outputSeconds: 2.5 });
  assert.deepEqual(
    choices.filter((c) => !c.fits).map((c) => c.seconds),
    [2],
  );
});

test('a text with no times shows for the whole momento, and saved times load as they were', () => {
  assert.deepEqual(textWindow({ text: 'GG', position_y: 0.5 }, 10), { start: 0, end: 10 });
  assert.deepEqual(textWindow({ text: 'GG', position_y: 0.5, start_seconds: 1, end_seconds: 3 }, 10), { start: 1, end: 3 });
});

test('dragging when a text appears stops half a second before it disappears', () => {
  const overlay = { text: 'GG', position_y: 0.5, end_seconds: 3 };
  assert.deepEqual(textTimingPatch({ overlay, edge: 'start', seconds: 5, clipDuration: 10 }), { start_seconds: 2.5 });
  assert.deepEqual(textTimingPatch({ overlay, edge: 'end', seconds: -4, clipDuration: 10 }), { end_seconds: 0.5 });
});

test('a text dragged to the edges of the momento goes back to showing from the start or to the end', () => {
  const overlay = { text: 'GG', position_y: 0.5, start_seconds: 2, end_seconds: 6 };
  assert.deepEqual(textTimingPatch({ overlay, edge: 'start', seconds: 0, clipDuration: 10 }), { start_seconds: undefined });
  assert.deepEqual(textTimingPatch({ overlay, edge: 'end', seconds: 12, clipDuration: 10 }), { end_seconds: undefined });
});

test('the groups of a joined Short stay as they were after an adjustment', () => {
  const groups = [{ id: 'g1', clip_ids: ['c1', 'c2'] }];
  assert.deepEqual(withMomentEdit(plan({ groups }), { clipId: 'c1', patch: { speed: 2 } }).groups, groups);
});
