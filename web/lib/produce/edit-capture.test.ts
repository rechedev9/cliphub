import test from 'node:test';
import assert from 'node:assert/strict';
import { DEFAULT_EDIT_CONFIG } from '../api/reel-store.ts';
import type { EditConfig } from '../api/types.ts';
import {
  captureNote,
  editCtaLabel,
  pickVersion,
  recapSeconds,
  shortEditNeedsCapture,
  shortSettingsFromVersion,
  spaceWarning,
  versionOptionsLabel,
  type ShortCaptureChoice,
} from './edit-capture.ts';

const FALLBACK = 'Crear demo completa';

test('an edit only re-renders when the server keeps every capture', () => {
  assert.equal(editCtaLabel({ capture: { needed: false, missingRounds: [] }, editing: true, fallback: FALLBACK }), 'Volver a montar');
  assert.equal(editCtaLabel({ capture: { needed: true, missingRounds: [4] }, editing: true, fallback: FALLBACK }), 'Volver a grabar y montar');
  assert.equal(editCtaLabel({ capture: null, editing: true, fallback: FALLBACK }), FALLBACK);
  assert.equal(editCtaLabel({ capture: { needed: false, missingRounds: [] }, editing: false, fallback: FALLBACK }), FALLBACK);
});

test('the capture note names only the rounds an edit records', () => {
  assert.equal(captureNote({ needed: false, missingRounds: [] }, 3), 'Las capturas de todas las rondas están guardadas: solo se vuelve a montar.');
  assert.equal(captureNote({ needed: true, missingRounds: [3, 5] }, 6), 'Se grabarán otra vez las rondas 3 y 5; las demás ya están capturadas.');
  assert.equal(captureNote({ needed: true, missingRounds: [7] }, 6), 'Se grabará otra vez la ronda 7; las demás ya están capturadas.');
  assert.equal(captureNote({ needed: true, missingRounds: [1, 2] }, 2), 'Las capturas de este vídeo ya no están guardadas: se grabarán todas sus rondas.');
});

test('the space warning appears only when the estimate does not fit', () => {
  const gib = 1024 ** 3;
  assert.equal(spaceWarning({ freeBytes: 2 * gib, neededBytes: 5 * gib, short: true }), 'Puede que no haya espacio suficiente: hacen falta unos 5 GB y quedan 2 GB libres.');
  assert.equal(spaceWarning({ freeBytes: 9 * gib, neededBytes: 5 * gib, short: false }), null);
  assert.equal(spaceWarning(null), null);
});

const recorded: ShortCaptureChoice = { playIds: ['a', 'b', 'c'], hudMode: 'deathnotices', edit: { ...DEFAULT_EDIT_CONFIG, format: 'short-9x16' } };

test('a Short edit records again only for what its capture lacks', () => {
  const edit = (patch: Partial<EditConfig>): EditConfig => ({ ...recorded.edit, ...patch });
  assert.equal(shortEditNeedsCapture(recorded, { ...recorded, playIds: ['c', 'a'] }), false, 'fewer plays reuse their clips');
  assert.equal(shortEditNeedsCapture(recorded, { ...recorded, edit: edit({ killEffect: 'glitch', transition: 'whip', intro: true }) }), false, 'effects are render only');
  assert.equal(shortEditNeedsCapture(recorded, { ...recorded, playIds: ['a', 'd'] }), true, 'a new play');
  assert.equal(shortEditNeedsCapture(recorded, { ...recorded, hudMode: 'clean' }), true, 'another HUD');
  assert.equal(shortEditNeedsCapture(recorded, { ...recorded, edit: edit({ nativeHud: true }) }), true, 'the native HUD');
  assert.equal(shortEditNeedsCapture(recorded, { ...recorded, edit: edit({ victimReplay: true }) }), true, 'a victim replay');
  assert.equal(shortEditNeedsCapture({ ...recorded, edit: edit({ victimReplay: true }) }, recorded), false, 'dropping the replay');
  assert.equal(shortEditNeedsCapture(recorded, { ...recorded, edit: edit({ format: 'landscape-16x9' }) }), true, 'another framing');
  assert.equal(shortEditNeedsCapture(recorded, { ...recorded, edit: edit({ preRollSeconds: 8 }) }), true, 'another pre-roll');
  assert.equal(shortEditNeedsCapture(recorded, { ...recorded, edit: edit({ preRollSeconds: 3 }) }), false, '3 s is the analysis cut');
  const summary: ShortCaptureChoice = { playIds: ['a'], hudMode: 'gameplay', edit: edit({ format: 'landscape-16x9', nativeHud: true }) };
  assert.equal(shortEditNeedsCapture(summary, { ...summary, edit: { ...summary.edit, nativeHud: false, captureHud: 'clean' } }), true, 'another capture style');
});

test('editing a Short version preloads its plays, options and music', () => {
  const wire = { format: 'short-9x16', killEffect: 'glitch', transition: 'whip', cover_strategy: 'generated-gameplay', intro: false, outro: true, hook_text: true, kill_counter: true };
  const settings = shortSettingsFromVersion({
    variant: 'viral-60-clean',
    segmentIds: ['b', 'a'],
    edit: wire,
    music: { key: 'phonk-01', volume: 0.5, game_volume: 0.4 },
  });
  assert.ok(settings);
  assert.deepEqual(settings.selectedIds, ['b', 'a']);
  assert.equal(settings.variant, 'viral-60-clean');
  assert.equal(settings.editConfig.killEffect, 'glitch');
  assert.equal(settings.editConfig.outro, true);
  assert.equal(settings.songId, 'phonk-01');
  assert.equal(settings.musicDecided, true);
  assert.equal(settings.musicVolume, 50);
  assert.equal(settings.gameVolume, 40);
  const silent = shortSettingsFromVersion({ variant: 'viral-60-clean', segmentIds: ['a'], edit: wire, music: { key: '', volume: 0 } });
  assert.equal(silent?.songId, null);
  assert.equal(silent?.musicDecided, true);
  assert.equal(shortSettingsFromVersion({ variant: 'viral-60-clean', segmentIds: ['a'], edit: 'bad' }), null);
});

const version = (revision: string, current: boolean) => ({ revision, createdAt: '2026-11-18T10:00:00Z', current, videos: ['v'], segmentIds: ['a', 'b'], edit: {}, music: { key: 'phonk-01', volume: 0.5 }, planId: null, rounds: [] });

test('Editar opens the version asked for, else the one Mis vídeos shows', () => {
  const versions = [version('new', false), version('shown', true), version('old', false)];
  assert.equal(pickVersion(versions, 'old')?.revision, 'old');
  assert.equal(pickVersion(versions, null)?.revision, 'shown');
  assert.equal(pickVersion([version('only', false)], 'gone')?.revision, 'only');
  assert.equal(pickVersion([], null), null);
});

test('a version names its rounds or its plays and music', () => {
  assert.equal(versionOptionsLabel({ planId: 'p', rounds: [7, 3, 5], segmentIds: [], music: undefined }), 'Rondas 3, 5 y 7');
  assert.equal(versionOptionsLabel({ planId: 'p', rounds: [4], segmentIds: [], music: undefined }), 'Ronda 4');
  assert.equal(versionOptionsLabel(version('a', true)), '2 jugadas, con música');
  assert.equal(versionOptionsLabel({ planId: null, rounds: [], segmentIds: ['a'], music: { key: '', volume: 0 } }), '1 jugada, sin música');
});

test('a new Full Demo estimates its capture from the chosen rounds of the recap', () => {
  const recap = [{ round: 1, startSeconds: 0, endSeconds: 100 }, { round: 2, startSeconds: 100, endSeconds: 190 }, { round: 3 }];
  assert.equal(recapSeconds(recap, []), 190);
  assert.equal(recapSeconds(recap, [2, 3]), 90);
});
