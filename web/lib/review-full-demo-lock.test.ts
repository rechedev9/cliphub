import test from 'node:test';
import assert from 'node:assert/strict';
import { FULL_DEMO_EDIT, fullDemoEdit } from './full-demo.ts';
import { constrainEditConfig, isLandscapeRecap } from './reel-brief.ts';
import { DEFAULT_EDIT_CONFIG } from './api/reel-store.ts';
import { DEMO_SOURCE } from './api/types.ts';

/** Shared QA dialog seam: flipping format to 9:16 must not collapse Full Demo. */
test('constrainEditConfig strips recap only when format becomes short-9x16', () => {
  assert.equal(isLandscapeRecap(FULL_DEMO_EDIT), true);
  const kept = constrainEditConfig({ ...FULL_DEMO_EDIT });
  assert.equal(kept.matchRecap, true);
  assert.equal(kept.voiceComms, true);
  assert.equal(kept.nativeHud, true);
  assert.equal(kept.format, 'landscape-16x9');

  const collapsed = constrainEditConfig({ ...fullDemoEdit(DEMO_SOURCE.faceit), format: 'short-9x16' });
  assert.equal(collapsed.format, 'short-9x16');
  assert.equal(collapsed.matchRecap, false);
  assert.equal(collapsed.voiceComms, false);
  assert.equal(collapsed.nativeHud, false);
  assert.equal(collapsed.demoSource, undefined);
  assert.equal(isLandscapeRecap(collapsed), false);
});

test('Shorts default edit is not a landscape recap', () => {
  assert.equal(isLandscapeRecap(DEFAULT_EDIT_CONFIG), false);
  assert.equal(DEFAULT_EDIT_CONFIG.format, 'short-9x16');
  assert.equal(constrainEditConfig(DEFAULT_EDIT_CONFIG).matchRecap, false);
});
