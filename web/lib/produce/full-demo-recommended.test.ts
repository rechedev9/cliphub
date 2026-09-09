import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { isFullDemoSnapshot, isFullDemoOptions } from '../full-demo-plan.ts';
import { recommendedFullDemoSettings } from './full-demo-recommended.ts';
import { CUSTOM_HUD_CAPTURE_PROFILE } from '../custom-hud.ts';

test('recommended tuning preserves a selected custom HUD and its capture contract', () => {
  const raw: unknown = JSON.parse(readFileSync(new URL('../full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(raw));
  const defaults = raw.document.options;
  const current = structuredClone(defaults);
  current.overlays.hud_theme = 'apex';
  current.capture.hud_profile = CUSTOM_HUD_CAPTURE_PROFILE;
  assert.ok(isFullDemoOptions(current));
  const next = recommendedFullDemoSettings(current, defaults);
  assert.equal(next.overlays.hud_theme, 'apex');
  assert.equal(next.capture.hud_profile, CUSTOM_HUD_CAPTURE_PROFILE);
  assert.ok(isFullDemoOptions(next));
});

test('recommended tuning preserves media and voice fallback decisions and requires a new plan', () => {
  const raw: unknown = JSON.parse(readFileSync(new URL('../full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(raw));
  const defaults = raw.document.options;
  const current = structuredClone(defaults);
  current.audio.voice.approved_fallback = 'without-voice';
  current.audio.voice.enabled = false;
  current.audio.game.gain = 0;
  current.editorial.manual_ranges = [{ round_id: 'round-001', start_tick: 100, end_tick: 200 }];
  const before = structuredClone(current);
  const next = recommendedFullDemoSettings(current, defaults);
  assert.ok(isFullDemoOptions(next));
  assert.equal(next.audio.game.gain, defaults.audio.game.gain);
  assert.equal(next.audio.voice.approved_fallback, 'without-voice');
  assert.equal(next.audio.voice.enabled, false);
  assert.deepEqual(next.audio.music.assets, current.audio.music.assets);
  assert.deepEqual(next.sponsor, current.sponsor);
  assert.deepEqual(next.overlays, current.overlays);
  assert.deepEqual(next.editorial.manual_ranges, []);
  assert.equal(next.editorial.freeze_seconds, 2);
  assert.deepEqual(current, before);
});
