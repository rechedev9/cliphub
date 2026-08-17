import assert from 'node:assert/strict';
import test from 'node:test';
import { mapPlateKey, resolveMapPlate, resolveMapPlateId } from './map-plate.ts';

test('map plate keys collapse demo prefixes and punctuation', () => {
  const cases: Array<[string, string]> = [
    ['de_dust2', 'dust2'],
    ['Dust2', 'dust2'],
    ['cs_office', 'office'],
    ['de_mirage', 'mirage'],
    ['ANCIENT', 'ancient'],
    ['de_overpass', 'overpass'],
  ];
  for (const [input, want] of cases) {
    assert.equal(mapPlateKey(input), want, input);
  }
});

test('known active-duty maps resolve to a named plate, not the generic fallback', () => {
  const known = [
    'de_ancient',
    'Ancient',
    'de_anubis',
    'de_dust2',
    'Inferno',
    'de_mirage',
    'Nuke',
    'de_overpass',
    'Vertigo',
    'de_train',
    'cs_office',
    'cs_italy',
    'cs_agency',
  ];
  for (const map of known) {
    assert.notEqual(resolveMapPlateId(map), 'unknown', map);
  }
});

test('the same map name always yields the same plate, even across id spellings', () => {
  const a = resolveMapPlate('de_mirage');
  const b = resolveMapPlate('Mirage');
  assert.equal(a.id, 'mirage');
  assert.deepEqual(a, b);
});

test('unknown maps stay unknown and stay stable for the same label', () => {
  assert.equal(resolveMapPlateId('de_thera'), 'unknown');
  assert.deepEqual(resolveMapPlate('Thera'), resolveMapPlate('de_thera'));
  assert.notDeepEqual(resolveMapPlate('Thera'), resolveMapPlate('Grind'));
});
