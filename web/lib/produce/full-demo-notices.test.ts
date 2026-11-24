import assert from 'node:assert/strict';
import { test } from 'node:test';
import { fullDemoBlockerText, fullDemoWarningText } from './full-demo-notices.ts';

test('an English planner blocker reads as a Spanish next step', () => {
  const text = fullDemoBlockerText({ code: 'voice_unavailable', message: 'Team voice is unavailable: silent' });
  assert.doesNotMatch(text, /Team voice|silent/);
  assert.match(text, /Quita «Incluir voces del equipo»/);
  assert.match(fullDemoBlockerText({ code: 'something_new', message: 'Unknown failure' }), /Revisa los ajustes/);
});

test('blockers the planner already writes in Spanish keep their detail', () => {
  const crosshair = 'La demo no incluye la mira del jugador en la ronda 4. ClipHub no graba con otra mira.';
  assert.equal(fullDemoBlockerText({ code: 'pov_contract_failed', message: crosshair }), crosshair);
  assert.equal(fullDemoBlockerText({ code: 'full_demo_asset_missing', message: 'Sube la captura de FACEIT' }), 'Sube la captura de FACEIT');
  assert.match(fullDemoBlockerText({ code: 'full_demo_asset_missing', message: 'The intro video is missing or invalid' }), /Falta un archivo/);
});

test('a warning names its round and an unknown one stays out of sight', () => {
  assert.equal(
    fullDemoWarningText({ code: 'pov_dead_in_freeze', message: 'Round excluded: player died before live play', round_id: 'round-007' }),
    'Una ronda se queda fuera: el jugador murió antes de empezar a jugarla. Ronda 7.',
  );
  assert.equal(fullDemoWarningText({ code: 'tick_drift', message: 'Tick drift' }), '');
});
