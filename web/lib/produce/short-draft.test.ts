import test from 'node:test';
import assert from 'node:assert/strict';
import { clearShortDraft, defaultShortSettings, loadShortDraft, parseShortDraft, saveShortDraft, shortDraftKey } from './short-draft.ts';

const defaults = defaultShortSettings([]);
const draft = { ...defaults, version: 1, selectedIds: ['r1', 'r2'], musicDecided: true };
const validIds = new Set(['r1']);

test('rejects incompatible versions and malformed selection containers', () => {
  for (const raw of [null, 'bad', [], { ...draft, version: 2 }, { ...draft, selectedIds: 'r1' }]) {
    assert.equal(parseShortDraft(raw, validIds), null);
  }
});

test('filters missing/duplicate ids without losing the other decisions or an intentional empty selection', () => {
  const restored = parseShortDraft({ ...draft, selectedIds: ['r1', 'r1', 'missing', null] }, validIds);
  assert.deepEqual(restored?.selectedIds, ['r1']);
  assert.equal(restored?.musicDecided, true);
  for (const selectedIds of [[], ['missing']]) {
    const empty = parseShortDraft({ ...draft, selectedIds }, validIds);
    assert.deepEqual(empty?.selectedIds, []);
    assert.equal(empty?.musicDecided, true);
  }
});

test('validates edit fields and strips Full Demo data before forcing the Short format', () => {
  const restored = parseShortDraft({ ...draft, editConfig: {
    format: 'landscape-16x9', fullDemo: { invalid: true }, matchRecap: true, nativeHud: true,
    voiceComms: true, overlayTheme: 'neon-violet', intro: 'true', killEffect: 'unknown', introText: 123,
  } }, validIds);
  assert.deepEqual(restored?.editConfig, defaults.editConfig);
});

test('validates volumes and requires a complete music choice', () => {
  const invalid = parseShortDraft({ ...draft, songId: 'song', songTitle: null, musicVolume: -3, gameVolume: Infinity, variant: 42 }, validIds);
  assert.equal(invalid?.musicDecided, false);
  assert.equal(invalid?.songId, null);
  assert.equal(invalid?.variant, null);
  assert.equal(invalid?.musicVolume, 100);
  assert.equal(invalid?.gameVolume, 70);
  const valid = parseShortDraft({ ...draft, songId: 'song', songTitle: 'Track', musicVolume: 5, gameVolume: 0 }, validIds);
  assert.equal(valid?.songId, 'song');
  assert.equal(valid?.musicVolume, 5);
  assert.equal(valid?.gameVolume, 0);
});

test('storage round-trip is scoped to a match, corrupt JSON is ignored, and clearing removes the draft', (t) => {
  const values = new Map<string, string>();
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'window');
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { sessionStorage: {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
  } } });
  t.after(() => {
    if (previous) Object.defineProperty(globalThis, 'window', previous);
    else Reflect.deleteProperty(globalThis, 'window');
  });
  saveShortDraft('match-a', { ...defaults, selectedIds: ['r1'] });
  assert.deepEqual(loadShortDraft('match-a', validIds)?.selectedIds, ['r1']);
  assert.equal(loadShortDraft('match-b', validIds), null);
  values.set(shortDraftKey('match-b'), '{invalid');
  assert.equal(loadShortDraft('match-b', validIds), null);
  clearShortDraft('match-a');
  assert.equal(loadShortDraft('match-a', validIds), null);
});

test('storage helpers tolerate an unavailable browser', () => {
  assert.equal(loadShortDraft('match', validIds), null);
  assert.doesNotThrow(() => saveShortDraft('match', defaults));
  assert.doesNotThrow(() => clearShortDraft('match'));
});
