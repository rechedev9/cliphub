import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { buildEditRequest } from '../api/edit-request.ts';
import { DEFAULT_EDIT_CONFIG } from '../api/reel-store.ts';
import { currentFullDemoOptions, fullDemoPlanEdit, isFullDemoSnapshot, type FullDemoSnapshot } from '../full-demo-plan.ts';
import { buildCloudFullDemoSubmit, buildCloudShortSubmit, fullDemoCloudBlocker, type CloudShortInput } from './submit.ts';
import { CLOUD_OWN_FILES_BLOCKER } from './view.ts';

const MATCH_ID = '22222222-2222-4222-8222-222222222222';

function input(overrides: Partial<CloudShortInput> = {}): CloudShortInput {
  return {
    matchId: MATCH_ID,
    segmentIds: ['seg-003', 'seg-007'],
    selectionLabel: '2 jugadas · Rondas 3, 7',
    preset: 'viral-60-clean',
    presetLabel: 'Killfeed',
    songId: null,
    editConfig: { ...DEFAULT_EDIT_CONFIG, format: 'short-9x16' },
    ...overrides,
  };
}

test('a Short without music sends the match, the preset, the segments in plan order and no music', () => {
  const body = buildCloudShortSubmit(input());
  assert.equal(body.job_id, MATCH_ID);
  assert.equal(body.kind, 'short');
  assert.equal(body.preset, 'viral-60-clean');
  assert.deepEqual(body.segment_ids, ['seg-003', 'seg-007']);
  assert.equal(body.music, undefined);
  assert.equal(body.title, '2 jugadas · Rondas 3, 7 - Killfeed');
  assert.equal('music' in JSON.parse(JSON.stringify(body)), false);
});

test('the edit part is the same request the local capture sends', () => {
  const editConfig = { ...DEFAULT_EDIT_CONFIG, format: 'short-9x16' as const, intro: true, introText: '  Hola  ', killCounter: true };
  assert.deepEqual(buildCloudShortSubmit(input({ editConfig })).edit, buildEditRequest(editConfig));
});

test('music travels as the local request does: a bare key at the default mix, an object otherwise', () => {
  assert.equal(buildCloudShortSubmit(input({ songId: 'track-1' })).music, 'track-1');
  assert.deepEqual(buildCloudShortSubmit(input({ songId: 'track-1', musicVolume: 0.5, gameVolume: 0.7 })).music, {
    key: 'track-1',
    volume: 0.5,
    game_volume: 0.7,
  });
  assert.deepEqual(buildCloudShortSubmit(input({ songId: 'track-1', gameVolume: 0.2 })).music, { key: 'track-1', game_volume: 0.2 });
  assert.equal(buildCloudShortSubmit(input({ songId: 'track-1' })).title, '2 jugadas · Rondas 3, 7 - Killfeed + Music');
});

test('volumes left over from a removed song never reach the cloud', () => {
  assert.equal(buildCloudShortSubmit(input({ songId: null, musicVolume: 0.5, gameVolume: 0.3 })).music, undefined);
});

test('the title fits the portal limit', () => {
  assert.equal(buildCloudShortSubmit(input({ selectionLabel: 'x'.repeat(300) })).title.length, 120);
});

function fullDemo(): FullDemoSnapshot {
  const value: unknown = JSON.parse(readFileSync(new URL('../full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(value));
  value.document.options = currentFullDemoOptions(value.document.options);
  return value;
}

test('a long video sends its approved plan as the local generate edit, with no segments or music', () => {
  const snapshot = fullDemo();
  const body = buildCloudFullDemoSubmit({ matchId: MATCH_ID, title: 'Vídeo largo · de_mirage', snapshot });
  assert.equal(body.kind, 'full_demo');
  assert.equal(body.preset, 'gameplay-pov-60');
  assert.deepEqual(body.segment_ids, []);
  assert.equal(body.music, undefined);
  assert.deepEqual(body.edit, buildEditRequest(fullDemoPlanEdit(snapshot)));
});

test('only the own intro, sponsor or outro videos and the Focus portrait keep a long video out of the cloud', () => {
  const options = fullDemo().document.options;
  const clip = { id: '33333333-3333-4333-8333-333333333333', sha256: 'a'.repeat(64) };
  const off = { enabled: false, video: null };
  assert.equal(fullDemoCloudBlocker(options), null);
  assert.equal(fullDemoCloudBlocker({ ...options, bumpers: { intro: { enabled: false, video: clip }, outro: off } }), null);
  assert.equal(fullDemoCloudBlocker({ ...options, bumpers: { intro: off, outro: { enabled: true, video: clip } } }), CLOUD_OWN_FILES_BLOCKER);
  assert.equal(fullDemoCloudBlocker({ ...options, overlays: { ...options.overlays, hud_theme: 'focus', hud_portrait: clip } }), CLOUD_OWN_FILES_BLOCKER);
});
