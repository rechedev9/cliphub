import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamClipRange, StreamEditPlan, StreamPlay } from '../api/streams.ts';
import { clipsWithPlays } from './plays.ts';
import { bulkAddLabel, bulkAddPlays, momentsFooterText, momentsInOrder, momentsRangeIssue, playRowAction, playTimeLabel, removeMoments, topPlayIds, undoRemoval, withoutMoments } from './moments.ts';

function play(id: string, start: number, end: number, score = 50): StreamPlay {
  return { id, label: id, start_seconds: start, end_seconds: end, kill_seconds: [start + 2], score };
}

function moment(id: string, start: number, end: number): StreamClipRange {
  return { id, start_seconds: start, end_seconds: end };
}

test('a jugada no momento overlaps offers Añadir', () => {
  assert.deepEqual(playRowAction([moment('m1', 0, 10)], play('a', 20, 30)), { kind: 'add' });
});

test('a jugada inside a momento jumps to that momento instead of adding or removing', () => {
  const covering = moment('hand', 25, 40);
  assert.deepEqual(playRowAction([moment('m1', 0, 10), covering], play('a', 20, 30)), { kind: 'jump', clip: covering });
});

test('the bulk add takes only kill jugadas that no momento covers', () => {
  const shout: StreamPlay = { ...play('shout', 100, 110), kind: 'voice_peak', kill_seconds: [], voice_seconds: [104] };
  const plays = [play('covered', 5, 15), play('free', 40, 50), shout, play('late', 200, 210)];
  assert.deepEqual(bulkAddPlays([moment('m1', 0, 10)], plays).map((p) => p.id), ['free', 'late']);
});

test('the bulk button label carries how many jugadas it adds', () => {
  assert.equal(bulkAddLabel(19), 'Añadir las 19 con kill');
  assert.equal(bulkAddLabel(1), 'Añadir la que queda con kill');
  assert.doesNotMatch(bulkAddLabel(0), /Añadir/);
});

test('the bulk count matches the momentos it creates when two jugadas overlap each other', () => {
  const plays = [play('first', 40, 50), play('overlapping', 45, 55), play('apart', 90, 100)];
  const picked = bulkAddPlays([], plays);
  let n = 0;
  assert.equal(clipsWithPlays([], picked, () => `m${++n}`).length, picked.length);
  assert.deepEqual(picked.map((p) => p.id), ['first', 'apart']);
});

test('Top marks the three highest scores among the jugadas shown', () => {
  const plays = [play('a', 0, 5, 10), play('b', 10, 15, 90), play('c', 20, 25, 40), play('d', 30, 35, 70), play('e', 40, 45, 55)];
  assert.deepEqual([...topPlayIds(plays)].sort(), ['b', 'd', 'e']);
  assert.deepEqual([...topPlayIds([play('only', 0, 5, 10)])], ['only']);
});

test('the strip lists momentos in time order, hand-marked ones included, numbered from 1', () => {
  const strip = momentsInOrder([moment('late', 300, 320), { ...moment('hand', 10, 25), title: 'Marcado a mano' }, moment('mid', 100, 130)]);
  assert.deepEqual(strip.map((m) => [m.number, m.clip.id]), [[1, 'hand'], [2, 'mid'], [3, 'late']]);
});

test('the footer reads the count and the total length of the momentos', () => {
  const clips = [moment('a', 0, 100), moment('b', 200, 250), { ...moment('c', 400, 440), edit: { speed: 2 } }];
  assert.equal(momentsFooterText(clips), '3 momentos · 2:50 en total');
  assert.equal(momentsFooterText([moment('a', 0, 12)]), '1 momento · 0:12 en total');
  assert.equal(momentsFooterText([]), 'Aún no has elegido ningún momento');
});


const EDITOR_WORDS = /solap|rango|duración|segundo|inválid|\d+ s\b/i;

function issueOf(clips: StreamClipRange[]): string {
  const issue = momentsRangeIssue({ clips, sourceDuration: 600 });
  assert.notEqual(issue, null);
  assert.doesNotMatch(issue ?? '', EDITOR_WORDS);
  return issue ?? '';
}

test('momentos that share a piece of the video name both, by the number the strip shows', () => {
  const issue = issueOf([moment('a', 10, 40), moment('c', 200, 230), moment('b', 30, 60)]);
  assert.match(issue, /momentos 1 y 2\b/);
});

test('a momento that runs past the end of the video names that momento and its end', () => {
  const issue = issueOf([moment('a', 10, 40), moment('b', 590, 640)]);
  assert.match(issue, /momento 2\b/);
  assert.match(issue, /final/);
});

test('a momento with its edges the wrong way round or before the video starts names the edge to move', () => {
  assert.match(issueOf([moment('a', 50, 50)]), /momento 1\b.*final/);
  assert.match(issueOf([moment('ok', 100, 120), moment('a', -5, 20)]), /momento 1\b.*inicio/);
});

test('momentos that fit the video, or none at all, have nothing to fix', () => {
  assert.equal(momentsRangeIssue({ clips: [moment('a', 10, 40), moment('b', 40, 60)], sourceDuration: 600 }), null);
  assert.equal(momentsRangeIssue({ clips: [], sourceDuration: 600 }), null);
  assert.equal(momentsRangeIssue({ clips: [moment('a', 10, 4000)], sourceDuration: 0 }), null);
});

test('a jugada row reads its start as m:ss and its length in seconds, or m:ss past a minute', () => {
  assert.equal(playTimeLabel(play('a', 1437, 1449)), '23:57 · 12 s');
  assert.equal(playTimeLabel(play('b', 65, 155)), '1:05 · 1:30');
});

test('undoing an add takes back only the momentos it created', () => {
  const clips = [moment('mine', 0, 10), moment('added', 20, 30), moment('later', 40, 50)];
  assert.deepEqual(withoutMoments(clips, ['added', 'gone']).map((c) => c.id), ['mine', 'later']);
});

const basePlan: StreamEditPlan = { schema_version: '1', variant: 'streamer-fullframe-nocam', clips: [] };

test('removing a momento drops it and leaves a joined Short of one as a single momento', () => {
  const plan: StreamEditPlan = {
    ...basePlan,
    clips: [moment('a', 0, 10), moment('b', 20, 30), moment('c', 40, 50)],
    groups: [{ id: 'g1', clip_ids: ['a', 'b'] }],
  };
  const { plan: next } = removeMoments(plan, ['b']);
  assert.deepEqual(next.clips.map((c) => c.id), ['a', 'c']);
  assert.equal(next.groups, undefined);
});

test('undoing a removal puts the momento and its joined Short back', () => {
  const plan: StreamEditPlan = {
    ...basePlan,
    clips: [moment('a', 0, 10), moment('b', 20, 30)],
    groups: [{ id: 'g1', clip_ids: ['a', 'b'] }],
  };
  const removal = removeMoments(plan, ['b']);
  const undone = undoRemoval(removal.plan, removal);
  assert.deepEqual(undone.clips.map((c) => c.id), ['a', 'b']);
  assert.deepEqual(undone.groups, [{ id: 'g1', clip_ids: ['a', 'b'] }]);
});

test('undoing a removal skips a momento whose place a newer momento took', () => {
  const removal = removeMoments({ ...basePlan, clips: [moment('a', 0, 10), moment('b', 20, 30)] }, ['b']);
  const later: StreamEditPlan = { ...removal.plan, clips: [...removal.plan.clips, moment('new', 25, 35)] };
  assert.deepEqual(undoRemoval(later, removal).clips.map((c) => c.id), ['a', 'new']);
});
