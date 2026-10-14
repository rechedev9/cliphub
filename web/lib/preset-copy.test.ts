import test from 'node:test';
import assert from 'node:assert/strict';
import { presetDescription } from './preset-copy.ts';

test('presetDescription returns the registry Spanish description', () => {
  const preset = {
    name: 'viral-60-clean',
    description: 'default clean viral edit',
    descriptionEs: 'Edición viral limpia por defecto',
  };
  assert.equal(presetDescription(preset), 'Edición viral limpia por defecto');
  assert.notEqual(presetDescription(preset), preset.description);
});

test('presetDescription falls back to the API description when Spanish is absent', () => {
  const preset = { name: 'some-future-preset', description: 'brand new registry copy' };
  assert.equal(presetDescription(preset), 'brand new registry copy');
  assert.equal(presetDescription({ ...preset, descriptionEs: '   ' }), 'brand new registry copy');
});
