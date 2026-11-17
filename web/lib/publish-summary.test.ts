import test from 'node:test';
import assert from 'node:assert/strict';
import type { Video } from './api/types.ts';
import type { FullDemoSnapshot } from './full-demo-plan.ts';
import { DEFAULT_EDIT_CONFIG } from './api/reel-store.ts';
import { FULL_DEMO_EDIT } from './full-demo.ts';
import { publishSummary, videoHudLabel } from './publish-summary.ts';

const FACTS = { map: 'de_mirage', player: 'YEKINDAR', score: '' };

function video(editConfig: Video['editConfig']): Video {
  return { id: 'v', title: 'Demo completa', map: 'de_mirage', score: '', mode: 'clean', status: 'ready', createdAt: 1, editConfig };
}

function snapshot(hudTheme: string | undefined, voice: 'available' | 'unavailable'): FullDemoSnapshot {
  return { document: { options: { overlays: { hud_theme: hudTheme } }, voice: { availability: voice } } } as unknown as FullDemoSnapshot;
}

test('a Full Demo with a broadcast HUD names that design, not the native HUD', () => {
  const edit = { ...FULL_DEMO_EDIT, fullDemo: snapshot('arena', 'unavailable') };
  assert.equal(videoHudLabel(video(edit)), 'HUD de retransmisión Arena');
  assert.equal(publishSummary(video(edit), FACTS), 'Rondas de YEKINDAR en Mirage, POV con HUD de retransmisión Arena.');
});

test('a native Full Demo mentions team voices only when the render mixed them', () => {
  const muted = { ...FULL_DEMO_EDIT, fullDemo: snapshot(undefined, 'available'), voiceComms: true, voiceVolume: 0 };
  assert.equal(publishSummary(video(muted), FACTS), 'Rondas de YEKINDAR en Mirage, POV con HUD nativo de CS2.');
  const voiced = { ...muted, voiceVolume: 0.8 };
  assert.equal(publishSummary(video(voiced), FACTS), 'Rondas de YEKINDAR en Mirage, POV con HUD nativo de CS2 y voces del equipo.');
});

test('a Short names its vertical HUD and claims no HUD it did not draw', () => {
  assert.equal(publishSummary(video({ ...DEFAULT_EDIT_CONFIG, verticalHud: true, verticalHudTheme: 'royale' }), FACTS), 'Highlights de YEKINDAR en Mirage, con HUD vertical Royale.');
  assert.equal(publishSummary(video({ ...DEFAULT_EDIT_CONFIG, nativeHud: false }), FACTS), 'Highlights de YEKINDAR en Mirage.');
  assert.equal(publishSummary(video(undefined), { ...FACTS, score: '13-9' }), 'Highlights de YEKINDAR en Mirage (13-9).');
});

test('a landscape recap without the native HUD or comms does not promise them', () => {
  const recap = { ...FULL_DEMO_EDIT, nativeHud: false, voiceComms: false };
  assert.equal(publishSummary(video(recap), FACTS), 'Partida completa de YEKINDAR en Mirage.');
  assert.equal(publishSummary(video({ ...recap, voiceComms: true }), FACTS), 'Partida completa de YEKINDAR en Mirage, con comms.');
  assert.equal(publishSummary(video({ ...recap, nativeHud: true, voiceComms: true }), FACTS), 'Partida completa de YEKINDAR en Mirage, POV con HUD nativo de CS2 y comms.');
});
