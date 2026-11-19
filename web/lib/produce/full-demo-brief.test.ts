import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { currentFullDemoOptions, isFullDemoSnapshot, type FullDemoOptions } from '../full-demo-plan.ts';
import { fullDemoBriefItems } from './full-demo-brief.ts';

function options(): FullDemoOptions {
  const value: unknown = JSON.parse(readFileSync(new URL('../full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(value));
  return currentFullDemoOptions(value.document.options);
}

function brief(value: FullDemoOptions, availability: string | null = null): Record<string, string> {
  return Object.fromEntries(fullDemoBriefItems(value, availability).map((item) => [item.label, item.value]));
}

test('the recap names each setting with the words of its control', () => {
  const base = { ...options(), source_kind: 'faceit' as const };
  const rows = brief({ ...base, bumpers: { intro: { enabled: false, video: null }, outro: { enabled: false, video: null } } });
  assert.equal(rows['Origen de la demo'], 'FACEIT');
  assert.equal(rows['Intro, sponsor y outro'], 'Ninguno');
  assert.equal(rows['Resolución y viewmodel'], 'Tu configuración (1920x1080)');
  assert.equal(rows['POV original 1:1'], 'No');
  assert.equal(rows['Efectos entre rondas'], 'Activados');
  assert.equal(rows['Presentación y marcador'], 'Neón');
  assert.equal(brief({ ...base, overlays: { ...base.overlays, theme: 'hud' } })['Presentación y marcador'], 'A juego con el HUD');
});

test('voices read as missing once a plan found none, and pending before', () => {
  const on = options();
  on.audio.voice.enabled = true;
  assert.equal(brief(on, 'available')['Voces del equipo'], 'Incluidas');
  assert.equal(brief(on, 'no_packets')['Voces del equipo'], 'No hay en esta demo');
  assert.equal(brief(on, null)['Voces del equipo'], 'Si la demo las tiene');
  assert.equal(brief({ ...on, audio: { ...on.audio, voice: { ...on.audio.voice, enabled: false } } }, 'no_packets')['Voces del equipo'], 'Sin voces');
});
