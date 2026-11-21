import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamClipRange, StreamPlay } from '../api/streams.ts';
import {
  fullTimelineWindow,
  momentRangeLabel,
  playMarkLabel,
  playNumbers,
  secondsAtFraction,
  timelineKeySeek,
  timelineMomentRanges,
  timelinePercent,
  timelinePlayhead,
  timelinePlayMarks,
  timelineSeekKeepsPlayback,
} from './timeline.ts';

test('a time maps to its share of the shown window and back', () => {
  const window = fullTimelineWindow(600);
  assert.equal(timelinePercent(150, window), 25);
  assert.equal(secondsAtFraction(0.25, window), 150);
  const zoomed = { start: 100, end: 200 };
  assert.equal(timelinePercent(150, zoomed), 50);
  assert.equal(secondsAtFraction(0.5, zoomed), 150);
});

test('a click past either end of the lane lands on that end of the window', () => {
  const window = { start: 100, end: 200 };
  assert.equal(secondsAtFraction(-0.2, window), 100);
  assert.equal(secondsAtFraction(1.4, window), 200);
  assert.equal(timelinePercent(50, window), 0);
  assert.equal(timelinePercent(900, window), 100);
});

function play(id: string, start: number, end: number, score = 50): StreamPlay {
  return { id, label: id, start_seconds: start, end_seconds: end, kill_seconds: [start + 2], score };
}

function moment(id: string, start: number, end: number): StreamClipRange {
  return { id, start_seconds: start, end_seconds: end };
}

test('jugadas are numbered in time order whatever order the list shows', () => {
  const best = [play('late', 300, 310, 90), play('early', 10, 20, 10), play('mid', 100, 110, 50)];
  const numbers = playNumbers(best);
  assert.deepEqual(['early', 'mid', 'late'].map((id) => numbers.get(id)), [1, 2, 3]);
});

test('each jugada is a mark at its start, with its number and kind', () => {
  const shout: StreamPlay = { ...play('shout', 300, 306), kind: 'voice_peak', kill_seconds: [], voice_seconds: [302] };
  const marks = timelinePlayMarks({ plays: [shout, play('kill', 150, 160)], clips: [], activePlayId: null, window: fullTimelineWindow(600) });
  assert.deepEqual(
    marks.map((m) => [m.play.id, m.number, m.voice, m.leftPercent]),
    [['kill', 1, false, 25], ['shout', 2, true, 50]],
  );
});

test('a mark tells apart a jugada in a momento and the jugada being watched', () => {
  const plays = [play('open', 10, 20), play('kept', 100, 110), play('watched', 200, 210)];
  const marks = timelinePlayMarks({ plays, clips: [moment('m1', 105, 130)], activePlayId: 'watched', window: fullTimelineWindow(600) });
  assert.deepEqual(marks.map((m) => [m.play.id, m.state]), [['open', 'open'], ['kept', 'covered'], ['watched', 'selected']]);
  const watchedKept = timelinePlayMarks({ plays, clips: [moment('m1', 105, 130)], activePlayId: 'kept', window: fullTimelineWindow(600) });
  assert.equal(watchedKept.find((m) => m.play.id === 'kept')?.state, 'selected');
});

test('a zoomed window leaves out the marks it does not show but keeps their numbers', () => {
  const plays = [play('a', 10, 20), play('b', 100, 110), play('c', 200, 210)];
  const marks = timelinePlayMarks({ plays, clips: [], activePlayId: null, window: { start: 90, end: 190 } });
  assert.deepEqual(marks.map((m) => [m.play.id, m.number, m.leftPercent]), [['b', 2, 10]]);
});

test('momentos are filled ranges numbered in time order', () => {
  const clips = [moment('late', 300, 360), moment('early', 60, 120)];
  const ranges = timelineMomentRanges({ clips, selectedClipIds: ['late'], window: fullTimelineWindow(600) });
  assert.deepEqual(
    ranges.map((r) => [r.clip.id, r.number, r.leftPercent, r.widthPercent, r.selected]),
    [['early', 1, 10, 10, false], ['late', 2, 50, 10, true]],
  );
});

test('a range that leaves the window is cut at its edge and one outside it is left out', () => {
  const clips = [moment('before', 0, 50), moment('across', 80, 150), moment('inside', 160, 180)];
  const ranges = timelineMomentRanges({ clips, selectedClipIds: [], window: { start: 100, end: 200 } });
  assert.deepEqual(ranges.map((r) => [r.clip.id, r.number, r.leftPercent, r.widthPercent]), [['across', 2, 0, 50], ['inside', 3, 60, 20]]);
});

test('a mark and a range are named with their number, what they are and when', () => {
  const plays = [play('Doble kill', 1437, 1449), { ...play('Grito', 1500, 1506), kind: 'voice_peak' as const, kill_seconds: [] }];
  const [kill, shout] = timelinePlayMarks({ plays, clips: [moment('m1', 1440, 1460)], activePlayId: null, window: fullTimelineWindow(3600) });
  assert.equal(playMarkLabel(kill), 'Jugada 1, kill: Doble kill, 23:57, en tus momentos');
  assert.equal(playMarkLabel(shout), 'Jugada 2, pico de voz: Grito, 25:00');
  const [range] = timelineMomentRanges({ clips: [{ ...moment('m1', 1440, 1460), title: 'Ace' }], selectedClipIds: [], window: fullTimelineWindow(3600) });
  assert.equal(momentRangeLabel(range), 'Momento 1: Ace, 24:00 a 24:20');
  const [untitled] = timelineMomentRanges({ clips: [moment('m1', 0, 30)], selectedClipIds: [], window: fullTimelineWindow(3600) });
  assert.equal(momentRangeLabel(untitled), 'Momento 1: 0:00 a 0:30');
});

test('arrow keys on the lane step through the video and Home and End jump to its ends', () => {
  const window = fullTimelineWindow(600);
  assert.equal(timelineKeySeek({ key: 'ArrowRight', seconds: 100, window }), 105);
  assert.equal(timelineKeySeek({ key: 'ArrowLeft', seconds: 2, window }), 0);
  assert.equal(timelineKeySeek({ key: 'PageUp', seconds: 100, window }), 160);
  assert.equal(timelineKeySeek({ key: 'PageDown', seconds: 100, window }), 40);
  assert.equal(timelineKeySeek({ key: 'End', seconds: 100, window }), 600);
  assert.equal(timelineKeySeek({ key: 'Home', seconds: 100, window: { start: 50, end: 150 } }), 50);
  assert.equal(timelineKeySeek({ key: 'Enter', seconds: 100, window }), null);
});

test('the playhead shows where it is inside the window and the slider value stays within its bounds', () => {
  assert.deepEqual(timelinePlayhead({ seconds: 150, window: { start: 100, end: 200 } }), { leftPercent: 50, valueMin: 100, valueMax: 200, valueNow: 150 });
  assert.deepEqual(timelinePlayhead({ seconds: 200, window: { start: 100, end: 200 } }), { leftPercent: 100, valueMin: 100, valueMax: 200, valueNow: 200 });
});

test('a playhead outside the zoomed window is hidden instead of stuck on the edge', () => {
  assert.deepEqual(timelinePlayhead({ seconds: 40, window: { start: 100, end: 200 } }), { leftPercent: null, valueMin: 100, valueMax: 200, valueNow: 100 });
  assert.deepEqual(timelinePlayhead({ seconds: 900.4, window: { start: 100, end: 200 } }), { leftPercent: null, valueMin: 100, valueMax: 200, valueNow: 200 });
  assert.equal(timelinePlayhead({ seconds: 0, window: fullTimelineWindow(0) }).leftPercent, null);
});

test('a click inside the momento being played keeps playing it and a click outside goes back to the original video', () => {
  const selected = moment('m1', 100, 130);
  const seekAt = (seconds: number) => timelineSeekKeepsPlayback({ mode: 'selected', seconds, preview: null, selected, clips: [selected] });
  assert.equal(seekAt(115), true);
  assert.equal(seekAt(130), true);
  assert.equal(seekAt(140), false);
});

test('while a jugada is previewed only a click inside that jugada keeps the preview', () => {
  const selected = moment('m1', 100, 130);
  const preview = moment('play:a', 300, 312);
  const seekAt = (seconds: number) => timelineSeekKeepsPlayback({ mode: 'selected', seconds, preview, selected, clips: [preview] });
  assert.equal(seekAt(305), true);
  assert.equal(seekAt(115), false);
  assert.equal(seekAt(320), false);
});

test('playing every momento in a row keeps going wherever a momento is, and the original video stays itself', () => {
  const clips = [moment('m1', 100, 130), moment('m2', 200, 230)];
  assert.equal(timelineSeekKeepsPlayback({ mode: 'sequence', seconds: 210, preview: null, selected: clips[0], clips }), true);
  assert.equal(timelineSeekKeepsPlayback({ mode: 'sequence', seconds: 160, preview: null, selected: clips[0], clips }), false);
  assert.equal(timelineSeekKeepsPlayback({ mode: 'source', seconds: 110, preview: null, selected: clips[0], clips }), false);
  assert.equal(timelineSeekKeepsPlayback({ mode: 'selected', seconds: 110, preview: null, selected: undefined, clips: [] }), false);
});

test('a video with no known length maps everything to the start', () => {
  const window = fullTimelineWindow(0);
  assert.equal(timelinePercent(30, window), 0);
  assert.equal(secondsAtFraction(0.5, window), 0);
});
