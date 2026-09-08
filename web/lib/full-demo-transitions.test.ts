import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fullDemoApprovalKey, fullDemoOptionsKey, fullDemoPlanEdit, isFullDemoOptions, isFullDemoSnapshot } from './full-demo-plan.ts';
import { DEFAULT_FULL_DEMO_TRANSITIONS, fullDemoTransitionPreset } from './full-demo-transitions.ts';

function fixture() {
  const raw: unknown = JSON.parse(readFileSync(new URL('./full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(raw));
  return raw;
}

test('old approvals stay unchanged until a transition decision is edited', () => {
  const snapshot = fixture();
  const before = JSON.stringify(snapshot);
  assert.ok(fullDemoApprovalKey(snapshot.document, snapshot.document.options));
  fullDemoPlanEdit(snapshot);
  assert.equal(JSON.stringify(snapshot), before);
  const changed = { ...snapshot.document.options, transitions: fullDemoTransitionPreset('kinetic') };
  assert.ok(isFullDemoOptions(changed));
  assert.notEqual(fullDemoOptionsKey(changed), fullDemoOptionsKey(snapshot.document.options));
  assert.equal(fullDemoApprovalKey(snapshot.document, changed), null);
  snapshot.document.options = changed;
  assert.deepEqual(fullDemoPlanEdit(snapshot).fullDemo?.document.options.transitions, changed.transitions);
});

test('all presets and independent switches survive the strict options boundary', () => {
  for (const preset of ['subtle', 'kinetic', 'audio'] as const) {
    const transitions = fullDemoTransitionPreset(preset);
    assert.ok(isFullDemoOptions({ ...fixture().document.options, transitions }));
    for (const key of ['whip', 'zoom', 'flash', 'rgb_split', 'whoosh', 'impact'] as const) {
      const modified = { ...transitions, [key]: !transitions[key] };
      assert.ok(isFullDemoOptions({ ...fixture().document.options, transitions: modified }));
    }
  }
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
