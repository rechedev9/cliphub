import test from 'node:test';
import assert from 'node:assert/strict';
import { replayShortcut, targetTakesKey } from './shortcut-keys.ts';
import type { KeyTarget, ReplayKey } from './shortcut-keys.ts';

const press = (key: string, extra: Partial<ReplayKey> = {}): ReplayKey => ({ key, shiftKey: false, ctrlKey: false, altKey: false, metaKey: false, altGraph: false, ...extra });
const target = (extra: Partial<KeyTarget> = {}): KeyTarget => ({ tag: 'DIV', inputType: null, editable: false, keyboardFocus: false, ...extra });

test('maps the documented keys to replay shortcuts', () => {
  assert.equal(replayShortcut(press(' ')), 'play');
  assert.equal(replayShortcut(press('[')), 'round-prev');
  assert.equal(replayShortcut(press(']')), 'round-next');
  assert.equal(replayShortcut(press('ArrowLeft')), 'seek-back');
  assert.equal(replayShortcut(press('ArrowRight', { shiftKey: true })), 'event-forward');
  assert.equal(replayShortcut(press('?', { shiftKey: true })), 'help');
  assert.equal(replayShortcut(press('/', { shiftKey: true })), 'help');
  assert.equal(replayShortcut(press('P')), 'fullscreen');
  assert.equal(replayShortcut(press('m')), 'multi');
  assert.equal(replayShortcut(press('f')), 'follow');
  assert.equal(replayShortcut(press('G')), 'gif');
  assert.equal(replayShortcut(press('h')), 'tool-move');
  assert.equal(replayShortcut(press('D')), 'tool-draw');
  assert.equal(replayShortcut(press('a')), 'tool-arrow');
  assert.equal(replayShortcut(press('e')), 'tool-erase');
  assert.equal(replayShortcut(press('Delete')), 'tool-erase');
  assert.equal(replayShortcut(press('t')), 'tool-timed');
  assert.equal(replayShortcut(press('z')), 'undo-stroke');
  assert.equal(replayShortcut(press('z', { ctrlKey: true })), null);
  assert.equal(replayShortcut(press('Escape')), 'escape');
  assert.equal(replayShortcut(press('x')), null);
});

test('J and K change round like [ and ], with or without Shift, and leave Ctrl K to the search', () => {
  assert.equal(replayShortcut(press('j')), 'round-prev');
  assert.equal(replayShortcut(press('K', { shiftKey: true })), 'round-next');
  assert.equal(replayShortcut(press('k', { ctrlKey: true })), null);
  assert.equal(targetTakesKey(target({ tag: 'INPUT', inputType: 'search' }), 'round-next'), false);
  assert.equal(targetTakesKey(target({ tag: 'INPUT', inputType: 'range', keyboardFocus: true }), 'round-prev'), true);
});

test('AltGr characters still work, as on a Spanish keyboard where ] is AltGr and +', () => {
  // Chromium on Windows reports AltGr as Ctrl and Alt held together.
  assert.equal(replayShortcut(press(']', { ctrlKey: true, altKey: true })), 'round-next');
  assert.equal(replayShortcut(press('[', { altGraph: true })), 'round-prev');
});

test('real Ctrl, Alt or Meta chords are left to the browser', () => {
  assert.equal(replayShortcut(press(']', { ctrlKey: true })), null);
  assert.equal(replayShortcut(press('p', { altKey: true })), null);
  assert.equal(replayShortcut(press('ArrowRight', { ctrlKey: true, altKey: true })), null);
  assert.equal(replayShortcut(press(']', { metaKey: true })), null);
});

test('text fields keep their keys', () => {
  assert.equal(targetTakesKey(target({ tag: 'TEXTAREA' }), 'play'), false);
  assert.equal(targetTakesKey(target({ tag: 'INPUT', inputType: 'number' }), 'seek-back'), false);
  assert.equal(targetTakesKey(target({ editable: true }), 'round-next'), false);
  assert.equal(targetTakesKey(target({ tag: 'SELECT' }), 'play'), false);
});

test('Space plays or pauses after a mouse click on a button, but presses a button reached with the keyboard', () => {
  assert.equal(targetTakesKey(target({ tag: 'BUTTON' }), 'play'), true);
  assert.equal(targetTakesKey(target({ tag: 'BUTTON', keyboardFocus: true }), 'play'), false);
  assert.equal(targetTakesKey(target({ tag: 'INPUT', inputType: 'checkbox', keyboardFocus: true }), 'play'), false);
  assert.equal(targetTakesKey(target({ tag: 'BUTTON', keyboardFocus: true }), 'round-next'), true);
});

test('the arrows on the timeline seek like everywhere else in the replay', () => {
  const timeline = target({ tag: 'INPUT', inputType: 'range', keyboardFocus: true });
  assert.equal(targetTakesKey(timeline, 'seek-forward'), true);
  assert.equal(targetTakesKey(timeline, 'event-back'), true);
  assert.equal(targetTakesKey(timeline, 'play'), true);
});
