import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamClipRange, StreamPlay } from '../api/streams.ts';
import { momentsKeyAction, momentsShortcutLegend, removableMomentId, type MomentsKeyFocus, type MomentsKeyState } from './moments-keys.ts';

function play(id: string, start: number, end: number): StreamPlay {
  return { id, label: `Jugada ${id}`, start_seconds: start, end_seconds: end, kill_seconds: [start + 1], score: 1 };
}

function moment(id: string, start: number, end: number): StreamClipRange {
  return { id, start_seconds: start, end_seconds: end };
}

const PLAYS = [play('a', 10, 20), play('b', 100, 110), play('c', 300, 310)];
const PAGE: MomentsKeyFocus = { tag: 'BODY', role: null, editable: false, playRowId: null, inDialog: false };
const STATE: MomentsKeyState = {
  plays: PLAYS,
  clips: [],
  selectedPlayId: null,
  lastPlayId: null,
  selectedClipId: null,
  canMark: true,
  busy: false,
};

function press(key: string, over: { focus?: Partial<MomentsKeyFocus>; state?: Partial<MomentsKeyState>; repeat?: boolean; modified?: boolean } = {}) {
  return momentsKeyAction({
    key,
    repeat: over.repeat ?? false,
    modified: over.modified ?? false,
    focus: { ...PAGE, ...over.focus },
    state: { ...STATE, ...over.state },
  });
}

test('Space plays and pauses from anywhere on the page', () => {
  assert.deepEqual(press(' '), { kind: 'toggle-play' });
});

test('no shortcut fires while the user types in a text field', () => {
  for (const key of [' ', 'ArrowDown', 'Enter', 'Delete', 'm']) {
    const state = { selectedPlayId: 'a', selectedClipId: 'x', clips: [moment('x', 400, 420)] };
    assert.deepEqual(press(key, { focus: { tag: 'INPUT' }, state }), { kind: 'none' });
    assert.deepEqual(press(key, { focus: { tag: 'TEXTAREA' }, state }), { kind: 'none' });
    assert.deepEqual(press(key, { focus: { tag: 'DIV', editable: true }, state }), { kind: 'none' });
  }
});

test('Space on a focused button other than a jugada row keeps the button behaviour', () => {
  assert.deepEqual(press(' ', { focus: { tag: 'BUTTON' } }), { kind: 'none' });
  assert.deepEqual(press(' ', { focus: { tag: 'BUTTON', role: 'switch' } }), { kind: 'none' });
  assert.deepEqual(press(' ', { focus: { tag: 'BUTTON', playRowId: 'b' } }), { kind: 'toggle-play' });
  assert.deepEqual(press(' ', { focus: { tag: 'DIV', role: 'slider' } }), { kind: 'toggle-play' });
});

test('Down and Up move to the next and previous jugada in the order shown and play it', () => {
  assert.deepEqual(press('ArrowDown', { state: { selectedPlayId: 'a' } }), { kind: 'watch-play', play: PLAYS[1] });
  assert.deepEqual(press('ArrowUp', { state: { selectedPlayId: 'c' } }), { kind: 'watch-play', play: PLAYS[1] });
  const shown = [PLAYS[2], PLAYS[0], PLAYS[1]];
  assert.deepEqual(press('ArrowDown', { state: { plays: shown, selectedPlayId: 'c' } }), { kind: 'watch-play', play: PLAYS[0] });
});

test('with no jugada selected the arrows start at the first jugada, or continue from the last one watched', () => {
  assert.deepEqual(press('ArrowDown'), { kind: 'watch-play', play: PLAYS[0] });
  assert.deepEqual(press('ArrowUp'), { kind: 'watch-play', play: PLAYS[0] });
  assert.deepEqual(press('ArrowDown', { state: { lastPlayId: 'a' } }), { kind: 'watch-play', play: PLAYS[1] });
  assert.deepEqual(press('ArrowDown', { state: { lastPlayId: 'gone' } }), { kind: 'watch-play', play: PLAYS[0] });
});

test('the arrows stop at the ends of the list and do nothing without jugadas', () => {
  assert.deepEqual(press('ArrowDown', { state: { selectedPlayId: 'c' } }), { kind: 'none' });
  assert.deepEqual(press('ArrowUp', { state: { selectedPlayId: 'a' } }), { kind: 'none' });
  assert.deepEqual(press('ArrowDown', { state: { plays: [] } }), { kind: 'none' });
});

test('the arrows keep their own behaviour on the timeline and on other controls that use them', () => {
  assert.deepEqual(press('ArrowDown', { focus: { tag: 'DIV', role: 'slider' } }), { kind: 'none' });
  assert.deepEqual(press('ArrowUp', { focus: { tag: 'BUTTON', role: 'radio' } }), { kind: 'none' });
  assert.deepEqual(press('ArrowDown', { focus: { tag: 'BUTTON' } }), { kind: 'watch-play', play: PLAYS[0] });
});

test('Enter adds the selected jugada', () => {
  assert.deepEqual(press('Enter', { state: { selectedPlayId: 'b' } }), { kind: 'add-play', play: PLAYS[1] });
});

test('Enter on a jugada already in a momento jumps to that momento', () => {
  const covering = moment('x', 105, 120);
  assert.deepEqual(press('Enter', { state: { selectedPlayId: 'b', clips: [covering] } }), { kind: 'jump', clip: covering });
});

test('Enter on a focused jugada row acts on that row, and does nothing with no jugada selected', () => {
  assert.deepEqual(press('Enter', { focus: { tag: 'BUTTON', playRowId: 'c' }, state: { selectedPlayId: 'a' } }), { kind: 'add-play', play: PLAYS[2] });
  assert.deepEqual(press('Enter'), { kind: 'none' });
});

test('Enter on a focused button other than a jugada row keeps the button behaviour', () => {
  assert.deepEqual(press('Enter', { focus: { tag: 'BUTTON' }, state: { selectedPlayId: 'b' } }), { kind: 'none' });
  assert.deepEqual(press('Enter', { focus: { tag: 'A' }, state: { selectedPlayId: 'b' } }), { kind: 'none' });
});

test('Delete removes the selected momento, and does nothing without one', () => {
  const clips = [moment('x', 400, 420), moment('y', 500, 520)];
  assert.deepEqual(press('Delete', { state: { clips, selectedClipId: 'y' } }), { kind: 'remove', clip: clips[1] });
  assert.deepEqual(press('Backspace', { state: { clips, selectedClipId: 'x' } }), { kind: 'remove', clip: clips[0] });
  assert.deepEqual(press('Delete', { state: { clips } }), { kind: 'none' });
  assert.deepEqual(press('Delete', { state: { clips: [], selectedClipId: 'gone' } }), { kind: 'none' });
});

test('Delete leaves the momentos alone while a jugada is being previewed', () => {
  const clips = [moment('x', 400, 420)];
  assert.deepEqual(press('Delete', { state: { clips, selectedClipId: 'x', selectedPlayId: 'a' } }), { kind: 'none' });
  assert.equal(removableMomentId({ clips, selectedClipId: 'x', selectedPlayId: 'a' }), null);
});

test('only a momento the user selected can be removed with the key', () => {
  const clips = [moment('x', 400, 420), moment('y', 500, 520)];
  assert.equal(removableMomentId({ clips, selectedClipId: 'y', selectedPlayId: null }), 'y');
  assert.equal(removableMomentId({ clips, selectedClipId: null, selectedPlayId: null }), null);
  assert.equal(removableMomentId({ clips, selectedClipId: 'gone', selectedPlayId: null }), null);
});

test('M marks a momento at the playhead once the video is loaded', () => {
  assert.deepEqual(press('m'), { kind: 'mark' });
  assert.deepEqual(press('M'), { kind: 'mark' });
  assert.deepEqual(press('m', { focus: { tag: 'BUTTON' } }), { kind: 'mark' });
  assert.deepEqual(press('m', { state: { canMark: false } }), { kind: 'none' });
});

test('shortcuts leave key combinations, held keys, dialogs and a busy editor alone', () => {
  assert.deepEqual(press('m', { modified: true }), { kind: 'none' });
  assert.deepEqual(press('ArrowDown', { modified: true }), { kind: 'none' });
  assert.deepEqual(press('m', { repeat: true }), { kind: 'none' });
  assert.deepEqual(press(' ', { repeat: true }), { kind: 'none' });
  assert.deepEqual(press('ArrowDown', { repeat: true }), { kind: 'watch-play', play: PLAYS[0] });
  assert.deepEqual(press(' ', { focus: { inDialog: true } }), { kind: 'none' });
  assert.deepEqual(press(' ', { state: { busy: true } }), { kind: 'none' });
});

test('the legend under the jugadas names every shortcut with the key printed on its control', () => {
  assert.deepEqual(momentsShortcutLegend({ vod: true }), [
    { keys: ['↑', '↓'], label: 'pasar de jugada' },
    { keys: ['Intro'], label: 'añadir' },
    { keys: ['Espacio'], label: 'reproducir' },
    { keys: ['Supr'], label: 'quitar momento' },
    { keys: ['M'], label: 'marcar momento' },
  ]);
});

test('a short video has no jugadas, so its legend leaves out the jugada keys', () => {
  assert.deepEqual(momentsShortcutLegend({ vod: false }), [
    { keys: ['Espacio'], label: 'reproducir' },
    { keys: ['Supr'], label: 'quitar momento' },
    { keys: ['M'], label: 'marcar momento' },
  ]);
});
