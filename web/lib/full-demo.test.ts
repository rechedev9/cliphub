import test from 'node:test';
import assert from 'node:assert/strict';
import { DEMO_SOURCE, OVERLAY_THEME, SERVICE_UNAVAILABLE_CODE } from './api/types.ts';
import {
  FULL_DEMO_CONTRACT,
  FULL_DEMO_EDIT,
  FULL_DEMO_EMPTY,
  FULL_DEMO_PRESET,
  FULL_DEMO_VARIANT,
  FULL_DEMO_VOICE_VOLUME,
  classifyFullDemoLoadFailure,
  fullDemoEmptyState,
} from './full-demo.ts';
import { NATIVE_HUD_LABEL } from './preset-copy.ts';

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
	assert.equal(FULL_DEMO_EDIT.demoSource, DEMO_SOURCE.faceit);
	assert.equal(FULL_DEMO_EDIT.overlayTheme, OVERLAY_THEME.faceitOrange);
  assert.equal(FULL_DEMO_VARIANT, 'gameplay-pov-60');
  assert.equal(FULL_DEMO_PRESET.name, 'gameplay-pov-60');
  assert.equal(FULL_DEMO_PRESET.label, 'POV nativo');
  assert.equal(FULL_DEMO_PRESET.hudMode, 'gameplay');
  assert.equal(FULL_DEMO_PRESET.width, 1920);
  assert.equal(FULL_DEMO_PRESET.height, 1080);
});

test('full-demo contract names live rounds, not freeze-to-end dumps', () => {
  const labels = FULL_DEMO_CONTRACT.map((row) => row.label);
	assert.deepEqual(labels, ['Formato', 'Entrega', 'Datos', 'Comms', 'HUD', 'Efectos', 'Mix']);
  const byLabel = Object.fromEntries(FULL_DEMO_CONTRACT.map((row) => [row.label, row.value]));
  assert.equal(byLabel.Formato, 'Horizontal 16:9 · 1920×1080');
  assert.equal(byLabel.Entrega, 'Rondas en vivo · sin freeze');
	assert.match(byLabel.Datos, /FACEIT obligatorio/);
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
