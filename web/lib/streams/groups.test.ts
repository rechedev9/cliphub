import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamEditPlan } from '../api/streams.ts';
import { EDIT_PLAN_SCHEMA_VERSION, planFingerprint } from './plan.ts';
import {
  joinStreamMoments,
  moveStreamGroupMoment,
  pruneStreamGroups,
  restoreStreamGroup,
  splitStreamGroup,
  streamMusicOffsets,
  streamPlanShorts,
  streamShortCount,
} from './groups.ts';

function plan(overrides: Partial<StreamEditPlan> = {}): StreamEditPlan {
  return {
    schema_version: EDIT_PLAN_SCHEMA_VERSION,
    variant: 'streamer-vertical-stack-40-60',
    clips: [
      { id: 'c1', start_seconds: 10, end_seconds: 20, title: 'TRIPLE KILL' },
      { id: 'c2', start_seconds: 30, end_seconds: 40 },
      { id: 'c3', start_seconds: 50, end_seconds: 79, title: 'ACE' },
    ],
    ...overrides,
  };
}

test('a group of two moments and one lone moment export as two Shorts', () => {
  const joined = joinStreamMoments(plan(), ['c3', 'c1']);
  const shorts = streamPlanShorts(joined);
  assert.equal(streamShortCount(joined), 2);
  assert.deepEqual(
    shorts.map((s) => [s.title, s.clips.map((c) => c.id), s.durationSeconds]),
    [
      ['TRIPLE KILL + ACE', ['c1', 'c3'], 39],
      ['Momento 2', ['c2'], 10],
    ],
  );
});

test('a reordered group plays in its own order and a named group keeps its title', () => {
  const joined = joinStreamMoments(plan(), ['c1', 'c3']);
  const groupId = joined.groups?.[0]?.id ?? '';
  const moved = moveStreamGroupMoment(
    { ...joined, groups: joined.groups?.map((g) => ({ ...g, title: 'Ronda 12' })) },
    { groupId, clipId: 'c3', offset: -1 },
  );
  const [first] = streamPlanShorts(moved);
  assert.equal(first?.title, 'Ronda 12');
  assert.deepEqual(first?.clips.map((c) => c.id), ['c3', 'c1']);
});

test('separating a group gives each moment its own Short back', () => {
  const joined = joinStreamMoments(plan(), ['c1', 'c2']);
  const split = splitStreamGroup(joined, joined.groups?.[0]?.id ?? '');
  assert.equal(split.groups, undefined);
  assert.equal(streamShortCount(split), 3);
});

test('removing a moment drops a group left with one member', () => {
  const joined = joinStreamMoments(plan(), ['c1', 'c2']);
  const pruned = pruneStreamGroups({ ...joined, clips: joined.clips.filter((c) => c.id !== 'c2') });
  assert.equal(pruned.groups, undefined);
});

test('joining a moment moves it out of its old group', () => {
  const first = joinStreamMoments(plan(), ['c1', 'c2']);
  const second = joinStreamMoments(first, ['c2', 'c3']);
  assert.deepEqual(second.groups?.map((g) => g.clip_ids), [['c2', 'c3']]);
});

test('grouping moves the export fingerprint and survives a JSON round trip', () => {
  const joined = joinStreamMoments(plan(), ['c1', 'c3']);
  assert.notEqual(planFingerprint(joined), planFingerprint(plan()));
  const restored: StreamEditPlan = JSON.parse(JSON.stringify(joined));
  assert.equal(planFingerprint(restored), planFingerprint(joined));
  assert.equal(streamShortCount(restored), 2);
});

test('music runs on inside a joined Short and starts over in a lone one', () => {
  const joined = joinStreamMoments(plan(), ['c1', 'c3']);
  const offsets = streamMusicOffsets(streamPlanShorts(joined));
  assert.deepEqual([offsets.get('c1'), offsets.get('c3'), offsets.get('c2')], [0, 10, 0]);
});

test('undoing a removed moment puts its group back', () => {
  const joined = joinStreamMoments(plan(), ['c1', 'c2']);
  const group = joined.groups?.[0];
  assert.ok(group);
  const removed = pruneStreamGroups({ ...joined, clips: joined.clips.filter((c) => c.id !== 'c2') });
  const restored = restoreStreamGroup({ ...removed, clips: joined.clips }, group);
  assert.deepEqual(restored.groups, joined.groups);
  const elsewhere = joinStreamMoments({ ...removed, clips: joined.clips }, ['c1', 'c3']);
  assert.deepEqual(restoreStreamGroup(elsewhere, group).groups?.map((g) => g.clip_ids), [['c1', 'c3']]);
});
