import test from 'node:test';
import assert from 'node:assert/strict';
import { buildEditRequest } from './edit-request.ts';
import { parseEffectiveEditConfig } from './render-hydration.ts';
import { coerceEditConfig } from './reel-store.ts';
import { constrainEditConfig } from '../reel-brief.ts';
import { FULL_DEMO_EDIT } from '../full-demo.ts';
import type { EditConfig } from './types.ts';

function edit(overrides: Partial<EditConfig> = {}): EditConfig {
  return {
    format: 'short-9x16',
    killEffect: 'punch-in',
    transition: 'flash',
    intro: false,
    outro: false,
    hookText: false,
    killCounter: false,
    matchRecap: false,
    voiceComms: false,
    nativeHud: false,
    coverStrategy: 'generated-gameplay',
    ...overrides,
  };
}

const CARDS = { openingCard: 'carta', endCard: 'mvp', coverCard: 'ficha' } as const;

test('each player card slot travels in a 9:16 Short and comes back from a render', () => {
  const body = buildEditRequest(edit({ ...CARDS, statsCard: true }));
  assert.equal(body.opening_card, 'carta');
  assert.equal(body.end_card, 'mvp');
  assert.equal(body.cover_card, 'ficha');
  assert.equal('stats_card' in body, false, 'the end card takes the text stats card place');
  const parsed = parseEffectiveEditConfig(body);
  assert.equal(parsed?.openingCard, 'carta');
  assert.equal(parsed?.endCard, 'mvp');
  assert.equal(parsed?.coverCard, 'ficha');
  assert.equal('opening_card' in buildEditRequest(edit()), false, 'no card is the default');
});

test('the cards stay out of a 16:9 video, a recap, a Full Demo and a Short without a cover', () => {
  for (const body of [
    buildEditRequest(edit({ ...CARDS, format: 'landscape-16x9' })),
    buildEditRequest(edit({ ...CARDS, matchRecap: true })),
    buildEditRequest({ ...FULL_DEMO_EDIT, ...CARDS }),
  ]) {
    assert.equal('opening_card' in body || 'end_card' in body || 'cover_card' in body, false);
  }
  const noCover = buildEditRequest(edit({ ...CARDS, coverStrategy: 'no-cover' }));
  assert.equal('cover_card' in noCover, false);
  assert.equal(noCover.opening_card, 'carta');
  const landscape = constrainEditConfig(edit({ ...CARDS, format: 'landscape-16x9' }));
  assert.equal(landscape.openingCard ?? landscape.endCard ?? landscape.coverCard, undefined);
});

test('a saved draft keeps its card slots and drops a malformed one', () => {
  const kept = coerceEditConfig({ ...edit(), ...CARDS });
  assert.equal(kept.openingCard, 'carta');
  assert.equal(kept.coverCard, 'ficha');
  assert.equal(coerceEditConfig({ ...edit(), openingCard: '<b>', endCard: 7 }).openingCard, undefined);
});
