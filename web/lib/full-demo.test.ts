import test from 'node:test';
import assert from 'node:assert/strict';
import { buildEditRequest } from './api/edit-request.ts';
import { reelIdentity } from './api/reel-identity.ts';
import {
  FULL_DEMO_CONTRACT,
  FULL_DEMO_EDIT,
  FULL_DEMO_PRESET,
  FULL_DEMO_VARIANT,
  FULL_DEMO_VOICE_VOLUME,
  resolveFullDemoPreset,
} from './full-demo.ts';

const JOB = '11111111-1111-4111-8111-111111111111';

test('full-demo edit is landscape recap with comms and native HUD', () => {
  assert.equal(FULL_DEMO_EDIT.format, 'landscape-16x9');
  assert.equal(FULL_DEMO_EDIT.matchRecap, true);
  assert.equal(FULL_DEMO_EDIT.voiceComms, true);
  assert.equal(FULL_DEMO_EDIT.nativeHud, true);
  assert.equal(FULL_DEMO_EDIT.killEffect, 'clean');
  assert.equal(FULL_DEMO_EDIT.transition, 'cut');
  assert.equal(FULL_DEMO_EDIT.intro, false);
  assert.equal(FULL_DEMO_EDIT.outro, false);
  assert.equal(FULL_DEMO_EDIT.hookText, false);
  assert.equal(FULL_DEMO_EDIT.killCounter, false);
  assert.equal(FULL_DEMO_EDIT.voiceVolume, FULL_DEMO_VOICE_VOLUME);
  assert.equal(FULL_DEMO_VARIANT, 'gameplay-pov-60');
  assert.equal(FULL_DEMO_PRESET.name, 'gameplay-pov-60');
  assert.equal(FULL_DEMO_PRESET.label, 'POV nativo');
  assert.equal(FULL_DEMO_PRESET.hudMode, 'gameplay');
});

test('resolveFullDemoPreset prefers the registry card and falls back to the locked native POV', () => {
  const fromApi = {
    name: 'gameplay-pov-60',
    label: 'POV nativo',
    description: 'from orchestrator',
    hudMode: 'gameplay',
  };
  assert.equal(resolveFullDemoPreset([fromApi]).description, 'from orchestrator');
  assert.equal(resolveFullDemoPreset([]).name, FULL_DEMO_PRESET.name);
  assert.equal(resolveFullDemoPreset([{ name: 'viral-60-clean', label: 'Killfeed', description: '' }]).name, FULL_DEMO_VARIANT);
});

test('full-demo mix is comms plus game with no music bed', () => {
  assert.equal(FULL_DEMO_VOICE_VOLUME, 0.85);
  const mix = FULL_DEMO_CONTRACT.find((row) => row.label === 'Mix');
  assert.equal(mix?.value, 'Comms + juego · sin música');
});

test('full-demo contract names live rounds, not freeze-to-end dumps', () => {
  const labels = FULL_DEMO_CONTRACT.map((row) => row.label);
  assert.deepEqual(labels, ['Formato', 'Entrega', 'Comms', 'HUD', 'Efectos', 'Mix']);
  const byLabel = Object.fromEntries(FULL_DEMO_CONTRACT.map((row) => [row.label, row.value]));
  assert.equal(byLabel.Formato, 'Horizontal 16:9 · 1920×1080');
  assert.equal(byLabel.Entrega, 'Rondas en vivo · sin freeze');
  assert.match(byLabel.Comms, /comms/i);
  assert.match(byLabel.HUD, /Nativo CS2/);
  assert.match(byLabel.Efectos, /punch-in/i);
  assert.equal(byLabel.Mix, 'Comms + juego · sin música');
  for (const row of FULL_DEMO_CONTRACT) {
    assert.equal(/completas/i.test(row.value), false, `${row.label} still says completas`);
    assert.equal(/cama 32/i.test(row.value), false, `${row.label} still advertises a music bed`);
  }
});

test('buildEditRequest is the native-HUD wire: recap, gameplay HUD, comms, no shorts garnish', () => {
  const body = buildEditRequest(FULL_DEMO_EDIT);
  assert.equal(body.format, 'landscape-16x9');
  assert.equal(body.match_recap, true);
  assert.equal(body.native_hud, true);
  assert.equal(body.voice_comms, true);
  assert.equal(body.voice_volume, FULL_DEMO_VOICE_VOLUME);
  assert.equal(body.killEffect, 'clean');
  assert.equal(body.transition, 'cut');
  assert.equal(body.intro, false);
  assert.equal(body.outro, false);
  assert.equal(body.hook_text, false);
  assert.equal(body.kill_counter, false);
  assert.equal('music' in body, false);
  assert.equal('song_id' in body, false);
});

test('full-demo identity does not collapse into a Shorts reel', () => {
  const shorts = reelIdentity({ matchId: JOB, playIds: ['seg-001', 'seg-002'] });
  const full = reelIdentity({ matchId: JOB, playIds: ['seg-001', 'seg-002'], editConfig: FULL_DEMO_EDIT });
  assert.equal(full, `${JOB}__full-demo`);
  assert.notEqual(full, shorts);
});
