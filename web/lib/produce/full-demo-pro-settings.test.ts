import assert from 'node:assert/strict';
import { test } from 'node:test';
import { proResolutionLabel, proSettingsSummary } from './full-demo-pro-settings.ts';

test('labels name the aspect ratio and scaling the way players say it', () => {
  assert.equal(proResolutionLabel({ width: 1280, height: 960, scaling: 'stretched' }), '1280x960 · 4:3 estirado');
  assert.equal(proResolutionLabel({ width: 1280, height: 1024, scaling: 'black-bars' }), '1280x1024 · 5:4 con bandas negras');
  assert.equal(proResolutionLabel({ width: 1680, height: 1050, scaling: 'native' }), '1680x1050 · 16:10 nativo');
  assert.equal(proResolutionLabel({ width: 1920, height: 1080, scaling: 'native' }), '1920x1080 · 16:9');
  assert.equal(proSettingsSummary(undefined), 'Los tuyos');
});
