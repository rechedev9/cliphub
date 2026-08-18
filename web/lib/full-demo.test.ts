import test from 'node:test';
import assert from 'node:assert/strict';
import { buildEditRequest } from './api/edit-request.ts';
import { reelIdentity } from './api/reel-identity.ts';
import { FULL_DEMO_CONTRACT, FULL_DEMO_EDIT, FULL_DEMO_VARIANT } from './full-demo.ts';

const JOB = '11111111-1111-4111-8111-111111111111';

test('full-demo edit is landscape recap with comms and native HUD', () => {
  assert.equal(FULL_DEMO_EDIT.format, 'landscape-16x9');
  assert.equal(FULL_DEMO_EDIT.matchRecap, true);
  assert.equal(FULL_DEMO_EDIT.voiceComms, true);
  assert.equal(FULL_DEMO_EDIT.nativeHud, true);
  assert.equal(FULL_DEMO_EDIT.killEffect, 'clean');
  assert.equal(FULL_DEMO_EDIT.transition, 'cut');
  assert.equal(FULL_DEMO_VARIANT, 'viral-60-clean');
});

test('full-demo contract names the locked delivery', () => {
  const labels = FULL_DEMO_CONTRACT.map((row) => row.label);
  assert.deepEqual(labels, ['Formato', 'Entrega', 'Comms', 'HUD', 'Efectos']);
});

test('buildEditRequest sends recap and native HUD on the wire', () => {
  const body = buildEditRequest(FULL_DEMO_EDIT);
  assert.equal(body.format, 'landscape-16x9');
  assert.equal(body.match_recap, true);
  assert.equal(body.native_hud, true);
  assert.equal(body.voice_comms, true);
  assert.equal(body.killEffect, 'clean');
  assert.equal(body.transition, 'cut');
});

test('full-demo identity does not collapse into a Shorts reel', () => {
  const shorts = reelIdentity({ matchId: JOB, playIds: ['seg-001', 'seg-002'] });
  const full = reelIdentity({ matchId: JOB, playIds: ['seg-001', 'seg-002'], editConfig: FULL_DEMO_EDIT });
  assert.equal(full, `${JOB}__full-demo`);
  assert.notEqual(full, shorts);
});
