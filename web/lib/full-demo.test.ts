import test from 'node:test';
import assert from 'node:assert/strict';
import { buildEditRequest } from './api/edit-request.ts';
import { reelIdentity } from './api/reel-identity.ts';
import { SERVICE_UNAVAILABLE_CODE } from './api/types.ts';
import {
  FULL_DEMO_CONTRACT,
  FULL_DEMO_EDIT,
  FULL_DEMO_EMPTY,
  FULL_DEMO_FORGE_HINT_EMPTY,
  FULL_DEMO_FORGE_HINT_ERROR,
  FULL_DEMO_PRESET,
  FULL_DEMO_RECAP_ERROR,
  FULL_DEMO_ROUNDS_PENDING,
  FULL_DEMO_VARIANT,
  FULL_DEMO_VOICE_VOLUME,
  classifyFullDemoLoadFailure,
  fullDemoEmptyState,
} from './full-demo.ts';
import { NATIVE_HUD_LABEL } from './preset-copy.ts';

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
  assert.equal(FULL_DEMO_PRESET.width, 1920);
  assert.equal(FULL_DEMO_PRESET.height, 1080);
});

test('full-demo mix is comms plus game with no music bed', () => {
  assert.equal(FULL_DEMO_VOICE_VOLUME, 0.85);
  const mix = FULL_DEMO_CONTRACT.find((row) => row.label === 'Mix');
  assert.equal(mix?.value, 'Comms + juego · sin música');
});

test('full-demo has no YouTube subscribe CTA or Shorts hook copy', () => {
  assert.equal(FULL_DEMO_EDIT.hookText, false);
  for (const row of FULL_DEMO_CONTRACT) {
    assert.equal(/subscribe|suscr[ií]b/i.test(row.value), false, `${row.label} has subscribe CTA`);
  }
});

test('full-demo contract names live rounds, not freeze-to-end dumps', () => {
  const labels = FULL_DEMO_CONTRACT.map((row) => row.label);
  assert.deepEqual(labels, ['Formato', 'Entrega', 'Comms', 'HUD', 'Efectos', 'Mix']);
  const byLabel = Object.fromEntries(FULL_DEMO_CONTRACT.map((row) => [row.label, row.value]));
  assert.equal(byLabel.Formato, 'Horizontal 16:9 · 1920×1080');
  assert.equal(byLabel.Entrega, 'Rondas en vivo · sin freeze');
  assert.match(byLabel.Comms, /comms/i);
  assert.equal(byLabel.HUD, NATIVE_HUD_LABEL);
  assert.match(byLabel.Efectos, /punch-in/i);
  assert.equal(byLabel.Mix, 'Comms + juego · sin música');
  for (const row of FULL_DEMO_CONTRACT) {
    assert.equal(/completas/i.test(row.value), false, `${row.label} still says completas`);
    assert.equal(/cama 32/i.test(row.value), false, `${row.label} still advertises a music bed`);
    assert.equal(/montage|jump-?cut|stitch|subscribe|suscr[ií]b/i.test(row.value), false, `${row.label} advertises a stitch or CTA`);
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

test('recap-plan failure copy is an error, not a pending parse or Shorts empty state', () => {
  assert.match(FULL_DEMO_RECAP_ERROR, /No se pudo cargar el plan de rondas/);
  assert.equal(/todavía|parseo|Demo no encontrada|jugada/i.test(FULL_DEMO_RECAP_ERROR), false);
  assert.match(FULL_DEMO_ROUNDS_PENDING, /rondas/);
  assert.equal(/jugada/i.test(FULL_DEMO_ROUNDS_PENDING), false);
  assert.notEqual(FULL_DEMO_RECAP_ERROR, FULL_DEMO_ROUNDS_PENDING);
  assert.match(FULL_DEMO_FORGE_HINT_EMPTY, /rondas/);
  assert.match(FULL_DEMO_FORGE_HINT_ERROR, /rondas/);
});

test('full-demo identity does not collapse into a Shorts reel', () => {
  const shorts = reelIdentity({ matchId: JOB, playIds: ['seg-001', 'seg-002'] });
  const full = reelIdentity({ matchId: JOB, playIds: ['seg-001', 'seg-002'], editConfig: FULL_DEMO_EDIT });
  assert.equal(full, `${JOB}__full-demo`);
  assert.notEqual(full, shorts);
});

test('classifyFullDemoLoadFailure keeps 503 offline and any other throw as a load error', () => {
  const cases: { name: string; err: unknown; want: 'offline' | 'error' }[] = [
    { name: 'service unavailable', err: { code: SERVICE_UNAVAILABLE_CODE }, want: 'offline' },
    { name: 'error instance with code', err: Object.assign(new Error('down'), { code: SERVICE_UNAVAILABLE_CODE }), want: 'offline' },
    { name: 'plan 500 without code', err: Object.assign(new Error('upstream error'), { status: 500 }), want: 'error' },
    { name: 'plain error', err: new Error('upstream error'), want: 'error' },
    { name: 'null', err: null, want: 'error' },
    { name: 'string', err: 'boom', want: 'error' },
  ];
  for (const { name, err, want } of cases) {
    assert.equal(classifyFullDemoLoadFailure(err), want, name);
  }
});

test('fullDemoEmptyState keeps 404 missing and does not paint a plan 500 as gone from disk', () => {
  const cases = [
    { failure: 'offline' as const, empty: FULL_DEMO_EMPTY.offline },
    { failure: 'error' as const, empty: FULL_DEMO_EMPTY.error },
    { failure: null, empty: FULL_DEMO_EMPTY.missing },
  ];
  for (const { failure, empty } of cases) {
    assert.deepEqual(fullDemoEmptyState(failure), empty);
  }
  assert.match(FULL_DEMO_EMPTY.error.title, /No se pudo cargar/);
  assert.equal(FULL_DEMO_EMPTY.missing.title, 'Demo no encontrada');
  assert.match(FULL_DEMO_EMPTY.missing.description, /ya no está en el disco/);
  assert.equal(/disco/i.test(FULL_DEMO_EMPTY.error.description), false);
});
