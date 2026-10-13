import test from 'node:test';
import assert from 'node:assert/strict';
import { buildEditRequest } from '../api/edit-request.ts';
import { DEFAULT_EDIT_CONFIG } from '../api/reel-store.ts';
import { buildCloudShortSubmit, type CloudShortInput } from './submit.ts';

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
