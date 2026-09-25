import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { currentFullDemoOptions, isFullDemoOptions, isFullDemoSnapshot } from './full-demo-plan.ts';
import { DEFAULT_FULL_DEMO_TRANSITIONS, fullDemoTransitionPreset } from './full-demo-transitions.ts';

function fixture() {
  const raw: unknown = JSON.parse(readFileSync(new URL('./full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(raw));
  raw.document.options = currentFullDemoOptions(raw.document.options);
  return raw;
}

test('the Dynamic preset stays within the strict options boundary', () => {
  const transitions = fullDemoTransitionPreset();
  assert.ok(isFullDemoOptions({ ...fixture().document.options, transitions }));
  assert.equal(transitions.direction, 'follow-motion');
  assert.equal(transitions.flash, true);
  assert.equal(transitions.rgb_split, true);
});

test('partial, unknown and out-of-range transitions are rejected even when disabled', () => {
  const options = fixture().document.options;
  for (const patch of [
    { duration_frames: 8.5 }, { duration_frames: 5 }, { duration_frames: 19 }, { blur_pixels: 65 },
    { direction: 'auto-expression' }, { zoom_percent: 16 }, { flash_frames: 1 }, { rgb_pixels: 7 },
    { whoosh_gain_db: 0 }, { comms_tail_seconds: NaN }, { comms_tail_seconds: 2 }, { game_tail_lowpass_hz: 100 },
    { enabled: undefined }, { surprise: true },
  ]) assert.equal(isFullDemoOptions({ ...options, transitions: { ...DEFAULT_FULL_DEMO_TRANSITIONS, ...patch } }), false, JSON.stringify(patch));
  assert.equal(isFullDemoOptions({ ...options, transitions: {} }), false);
});
