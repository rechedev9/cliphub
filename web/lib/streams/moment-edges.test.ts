import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamClipRange, StreamPlay } from '../api/streams.ts';
import { MIN_MOMENT_SECONDS, momentZoomWindow, moveMomentEdge, nudgeMomentEdge, suggestedCut, withMomentRange } from './moment-edges.ts';

function moment(id: string, start: number, end: number): StreamClipRange {
  return { id, start_seconds: start, end_seconds: end };
}

test('a selected momento zooms the timeline to it plus a margin on each side', () => {
  assert.deepEqual(momentZoomWindow(moment('m1', 600, 630), 3600), { start: 555, end: 675 });
});

test('the zoom window stays inside the video', () => {
  assert.deepEqual(momentZoomWindow(moment('m1', 10, 40), 60), { start: 0, end: 60 });
});

const three = [moment('a', 100, 130), moment('b', 200, 230), moment('c', 300, 330)];

function range(clips: readonly StreamClipRange[], id: string): [number, number] | undefined {
  const clip = clips.find((c) => c.id === id);
  return clip ? [clip.start_seconds, clip.end_seconds] : undefined;
}

test('a dragged edge moves to where it is dropped', () => {
  const start = moveMomentEdge({ clips: three, clipId: 'b', edge: 'start', seconds: 190, sourceDuration: 600 });
  assert.deepEqual(range(start, 'b'), [190, 230]);
  const end = moveMomentEdge({ clips: three, clipId: 'b', edge: 'end', seconds: 250, sourceDuration: 600 });
  assert.deepEqual(range(end, 'b'), [200, 250]);
});

test('an edge stops at the neighbouring momento', () => {
  const start = moveMomentEdge({ clips: three, clipId: 'b', edge: 'start', seconds: 50, sourceDuration: 600 });
  assert.deepEqual(range(start, 'b'), [130, 230]);
  const end = moveMomentEdge({ clips: three, clipId: 'b', edge: 'end', seconds: 500, sourceDuration: 600 });
  assert.deepEqual(range(end, 'b'), [200, 300]);
});

test('an edge stops at the bounds of the video', () => {
  const start = moveMomentEdge({ clips: three, clipId: 'a', edge: 'start', seconds: -20, sourceDuration: 600 });
  assert.deepEqual(range(start, 'a'), [0, 130]);
  const end = moveMomentEdge({ clips: three, clipId: 'c', edge: 'end', seconds: 900, sourceDuration: 600 });
  assert.deepEqual(range(end, 'c'), [300, 600]);
});

test('an edge cannot shrink the momento below the minimum duration', () => {
  const start = moveMomentEdge({ clips: three, clipId: 'b', edge: 'start', seconds: 260, sourceDuration: 600 });
  assert.deepEqual(range(start, 'b'), [230 - MIN_MOMENT_SECONDS, 230]);
  const end = moveMomentEdge({ clips: three, clipId: 'b', edge: 'end', seconds: 150, sourceDuration: 600 });
  assert.deepEqual(range(end, 'b'), [200, 200 + MIN_MOMENT_SECONDS]);
});

test('-1 s and +1 s move one edge by one second', () => {
  const earlier = nudgeMomentEdge({ clips: three, clipId: 'b', edge: 'start', delta: -1, sourceDuration: 600 });
  assert.deepEqual(range(earlier, 'b'), [199, 230]);
  const later = nudgeMomentEdge({ clips: three, clipId: 'b', edge: 'end', delta: 1, sourceDuration: 600 });
  assert.deepEqual(range(later, 'b'), [200, 231]);
});

test('a nudge obeys the same limits as a drag', () => {
  const touching = [moment('a', 100, 130), moment('b', 130, 160)];
  const blocked = nudgeMomentEdge({ clips: touching, clipId: 'b', edge: 'start', delta: -1, sourceDuration: 600 });
  assert.deepEqual(range(blocked, 'b'), [130, 160]);
  const atEnd = nudgeMomentEdge({ clips: [moment('a', 590, 600)], clipId: 'a', edge: 'end', delta: 1, sourceDuration: 600 });
  assert.deepEqual(range(atEnd, 'a'), [590, 600]);
  const shortest = [moment('a', 100, 100 + MIN_MOMENT_SECONDS)];
  const kept = nudgeMomentEdge({ clips: shortest, clipId: 'a', edge: 'end', delta: -1, sourceDuration: 600 });
  assert.deepEqual(range(kept, 'a'), [100, 100 + MIN_MOMENT_SECONDS]);
});

function play(id: string, start: number, end: number): StreamPlay {
  return { id, label: id, start_seconds: start, end_seconds: end, kill_seconds: [start + 2], score: 50 };
}

test('the suggested cut is the range of the jugada the momento overlaps', () => {
  const trimmed = [moment('m1', 205, 215)];
  const cut = suggestedCut({ clips: trimmed, clipId: 'm1', plays: [play('p0', 50, 70), play('p1', 200, 230)], sourceDuration: 600 });
  assert.deepEqual(cut, { start_seconds: 200, end_seconds: 230 });
});

test('no suggested cut without an overlapped jugada or when the range already matches it', () => {
  const plays = [play('p1', 200, 230)];
  assert.equal(suggestedCut({ clips: [moment('m1', 400, 420)], clipId: 'm1', plays, sourceDuration: 600 }), null);
  assert.equal(suggestedCut({ clips: [moment('m1', 200, 230)], clipId: 'm1', plays, sourceDuration: 600 }), null);
});

test('the suggested cut stops at the neighbouring momentos', () => {
  const clips = [moment('a', 150, 195), moment('m1', 205, 215), moment('c', 225, 260)];
  const cut = suggestedCut({ clips, clipId: 'm1', plays: [play('p1', 190, 240)], sourceDuration: 600 });
  assert.deepEqual(cut, { start_seconds: 195, end_seconds: 225 });
});

test('the suggested cut follows the jugada the momento overlaps most', () => {
  const cut = suggestedCut({ clips: [moment('m1', 228, 250)], clipId: 'm1', plays: [play('p1', 200, 230), play('p2', 235, 260)], sourceDuration: 600 });
  assert.deepEqual(cut, { start_seconds: 235, end_seconds: 260 });
});

test('undoing the suggested cut puts the previous range back', () => {
  const before = [moment('m1', 205, 215)];
  const after = withMomentRange({ clips: before, clipId: 'm1', range: { start_seconds: 200, end_seconds: 230 } });
  assert.deepEqual(after && range(after, 'm1'), [200, 230]);
  const undone = after && withMomentRange({ clips: after, clipId: 'm1', range: { start_seconds: 205, end_seconds: 215 } });
  assert.deepEqual(undone && range(undone, 'm1'), [205, 215]);
});

test('a range is refused when it would overlap another momento or the momento is gone', () => {
  const clips = [moment('a', 100, 130), moment('b', 200, 230)];
  assert.equal(withMomentRange({ clips, clipId: 'b', range: { start_seconds: 120, end_seconds: 230 } }), null);
  assert.equal(withMomentRange({ clips, clipId: 'missing', range: { start_seconds: 300, end_seconds: 330 } }), null);
});

test('moving an edge leaves the other momentos and the fine adjustments alone', () => {
  const edited = [{ ...three[0] }, { ...three[1], title: 'Ace', edit: { speed: 2 } }, { ...three[2] }];
  const next = moveMomentEdge({ clips: edited, clipId: 'b', edge: 'end', seconds: 240, sourceDuration: 600 });
  assert.deepEqual(next[1], { id: 'b', start_seconds: 200, end_seconds: 240, title: 'Ace', edit: { speed: 2 } });
  assert.deepEqual(next[0], three[0]);
  assert.deepEqual(next[2], three[2]);
});
