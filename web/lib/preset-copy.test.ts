// Unit tests for the Spanish preset description overrides.
// Run: node --test preset-copy.test.ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { PRESET_DESCRIPTION_ES, presetDescription } from './preset-copy.ts';

// The registry in internal/editor/preset.go is the source of truth: read the
// Go names and English descriptions so a renamed, added or reworded preset
// fails here instead of silently falling back to English in the picker.
const PRESET_GO = readFileSync(new URL('../../internal/editor/preset.go', import.meta.url), 'utf8');
const PRESET_CONSTS = new Map(
  [...PRESET_GO.matchAll(/^\s*(Preset\w+)\s*=\s*"([^"]+)"/gm)].map((m) => [m[1], m[2]]),
);
const ENGLISH_DESCRIPTIONS = new Map(
  [...PRESET_GO.matchAll(/^\s*Name:\s*(Preset\w+),[\s\S]*?^\s*Description:\s*"([^"]+)"/gm)].map((m) => {
    const name = PRESET_CONSTS.get(m[1]);
    assert.ok(name, `unresolved preset constant ${m[1]}`);
    return [name, m[2]];
  }),
);

test('overrides cover exactly the registry preset names', () => {
  assert.ok(ENGLISH_DESCRIPTIONS.size > 0, 'no presets parsed from preset.go');
  assert.deepEqual(Object.keys(PRESET_DESCRIPTION_ES).sort(), [...ENGLISH_DESCRIPTIONS.keys()].sort());
});

test('no override value is empty or left as the English source', () => {
  for (const [name, english] of ENGLISH_DESCRIPTIONS) {
    const value = PRESET_DESCRIPTION_ES[name];
    assert.ok(value && value.trim().length > 0, `empty override for ${name}`);
    assert.notEqual(value, english, `override for ${name} is still English`);
  }
});

test('presetDescription returns the Spanish override for a known preset', () => {
  const preset = { name: 'viral-60-clean', description: ENGLISH_DESCRIPTIONS.get('viral-60-clean') ?? '' };
  assert.equal(presetDescription(preset), PRESET_DESCRIPTION_ES['viral-60-clean']);
  assert.notEqual(presetDescription(preset), preset.description);
});

test('presetDescription falls back to the API description for an unknown preset', () => {
  const preset = { name: 'some-future-preset', description: 'brand new registry copy' };
  assert.equal(presetDescription(preset), 'brand new registry copy');
});
