import test from 'node:test';
import assert from 'node:assert/strict';
import { clearShortDraft, defaultShortSettings, loadShortDraft, parseShortDraft, saveShortDraft, shortDraftKey, summaryDraftId } from './short-draft.ts';

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

test('a saved death-reel draft keeps its mode', () => {
  const restored = parseShortDraft({ version: 1, selectedIds: ['death-001'], editConfig: { deathReel: true } }, new Set(['death-001']));
  assert.equal(restored?.editConfig.deathReel, true);
  assert.deepEqual(restored?.selectedIds, ['death-001']);
});

test('a player without kills opens a death reel of the deaths that fit the minute', () => {
  const deaths = [
    { id: 'death-001', matchId: 'm', kind: 'clean' as const, round: 2, kills: 0, label: 'Muerte · Ronda 2' },
    { id: 'death-002', matchId: 'm', kind: 'clean' as const, round: 5, kills: 0, label: 'Muerte · Ronda 5' },
  ];
  const settings = defaultShortSettings([], { deathPlays: deaths });
  assert.deepEqual(settings.selectedIds, ['death-001', 'death-002']);
  assert.equal(settings.editConfig.deathReel, true);
  assert.equal(settings.editConfig.killCounter, false);
  const many = Array.from({ length: 15 }, (_, i) => ({ ...deaths[0], id: `death-${i}`, round: i + 1, startSeconds: i * 100, endSeconds: i * 100 + 12 }));
  assert.equal(defaultShortSettings([], { deathPlays: many }).selectedIds.length, 5, 'five 12 s deaths fill the minute');
});

test('a one-player summary opens with every kill in match order and five seconds before each kill', () => {
  const kills = Array.from({ length: 30 }, (_, i) => ({
    id: `seg-${String(i + 1).padStart(3, '0')}`, matchId: 'm', kind: 'highlight' as const, round: i + 1, kills: 1 + (i % 3),
    label: `Ronda ${i + 1}`, startSeconds: i * 100, endSeconds: i * 100 + 20, score: ((i * 7) % 10) / 10,
  }));
  const summary = defaultShortSettings(kills, { summary: true, targetSeconds: 180 });
  assert.deepEqual(summary.selectedIds, kills.map((play) => play.id));
  assert.equal(summary.editConfig.preRollSeconds, 5);
  const short = defaultShortSettings(kills);
  assert.equal(short.editConfig.preRollSeconds, undefined, 'a Short keeps the 3 s of the analysis');
  assert.equal(short.selectedIds.length, 3, 'a Short still opens with the best minute');
});

test('a summary draft keeps its pre-roll and capture style, apart from the Short draft of the same match', (t) => {
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
  const ids = new Set(['r1', 'r2']);
  saveShortDraft(summaryDraftId('match'), { ...defaults, selectedIds: ['r1', 'r2'], editConfig: { ...defaults.editConfig, preRollSeconds: 8, captureHud: 'clean' } });
  saveShortDraft('match', { ...defaults, selectedIds: ['r2'] });
  const summary = loadShortDraft(summaryDraftId('match'), ids);
  assert.deepEqual(summary?.selectedIds, ['r1', 'r2']);
  assert.equal(summary?.editConfig.preRollSeconds, 8);
  assert.equal(summary?.editConfig.captureHud, 'clean');
  assert.deepEqual(loadShortDraft('match', ids)?.selectedIds, ['r2']);
  const odd = parseShortDraft({ ...draft, editConfig: { preRollSeconds: 4, captureHud: 'radar' } }, validIds);
  assert.equal(odd?.editConfig.preRollSeconds, undefined);
  assert.equal(odd?.editConfig.captureHud, undefined);
});
