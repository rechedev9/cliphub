import assert from 'node:assert/strict';
import test from 'node:test';
import {
  coerceRadarOpacity,
  isRadarPosition,
  isRadarSize,
  radarAllowed,
  radarAutoPosition,
  radarBriefLabel,
  radarEditFields,
} from './radar-overlay.ts';
import { DEFAULT_EDIT_CONFIG } from './api/reel-store.ts';
import type { EditConfig } from './api/types.ts';

const SHORT: EditConfig = { ...DEFAULT_EDIT_CONFIG, format: 'short-9x16' };

test('radar options accept only the corners, sizes and opacities the editor draws', () => {
  assert.equal(isRadarPosition('bottom-right'), true);
  assert.equal(isRadarPosition('center'), false);
  assert.equal(isRadarSize('large'), true);
  assert.equal(isRadarSize(2), false);
  assert.equal(coerceRadarOpacity(0.55), 0.55);
  assert.equal(coerceRadarOpacity(1), 1);
  // The default and anything the server refuses read as unset.
  for (const value of [0.9, 0.2, 1.5, Number.NaN, '0.5', undefined]) assert.equal(coerceRadarOpacity(value), undefined);
});

test('the radar is refused over a style that records CS2 HUD', () => {
  assert.equal(radarAllowed({ hudMode: 'deathnotices' }), true);
  assert.equal(radarAllowed(null), true);
  assert.equal(radarAllowed({ hudMode: 'gameplay' }), false);
});

test('radarEditFields keeps the chosen options and drops the defaults', () => {
  assert.deepEqual(radarEditFields({ ...SHORT, radar: true, radarPosition: 'top-left', radarSize: 'small', radarOpacity: 0.4 }), {
    radar: true,
    radarPosition: 'top-left',
    radarSize: 'small',
    radarOpacity: 0.4,
  });
  assert.deepEqual(radarEditFields({ ...SHORT, radar: true, radarSize: 'medium', radarOpacity: 0.9 }), { radar: true });
  for (const edit of [
    { ...SHORT, radarPosition: 'top-left' as const },
    { ...SHORT, radar: true, nativeHud: true },
    { ...SHORT, radar: true, matchRecap: true },
  ]) {
    assert.deepEqual(radarEditFields(edit), {});
  }
});

test('the brief names the corner the format picks when none is chosen', () => {
  assert.equal(radarAutoPosition('short-9x16'), 'bottom-left');
  assert.equal(radarAutoPosition('landscape-16x9'), 'top-left');
  assert.equal(radarBriefLabel({ ...SHORT, radar: true }), 'Abajo a la izquierda · Mediano · 90 %');
  assert.equal(radarBriefLabel({ ...SHORT, radar: true, radarPosition: 'top-right', radarSize: 'large', radarOpacity: 0.65 }), 'Arriba a la derecha · Grande · 65 %');
  assert.equal(radarBriefLabel(SHORT), 'No');
});
