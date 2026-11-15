import test from 'node:test';
import assert from 'node:assert/strict';
import { buildEditRequest } from './edit-request.ts';
import { parseEffectiveEditConfig } from './render-hydration.ts';
import { constrainEditConfig } from '../reel-brief.ts';
import type { EditConfig } from './types.ts';
import { FULL_DEMO_EDIT } from '../full-demo.ts';

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

test('the replay speed travels only when it is not the default half speed', () => {
  assert.equal(buildEditRequest(edit({ victimReplay: true, victimReplaySpeed: 0.25 })).victim_replay_speed, 0.25);
  assert.equal('victim_replay_speed' in buildEditRequest(edit({ victimReplay: true, victimReplaySpeed: 0.5 })), false);
  assert.equal('victim_replay_speed' in buildEditRequest(edit({ victimReplaySpeed: 1 })), false);
});

test('play badges and the stats card travel for a kill Short only', () => {
  const body = buildEditRequest(edit({ playBadges: true, statsCard: true }));
  assert.equal(body.play_badges, true);
  assert.equal(body.stats_card, true);
  assert.equal('stats_card' in buildEditRequest(edit({ statsCard: true, format: 'landscape-16x9' })), false);
  assert.equal('play_badges' in buildEditRequest(edit({ playBadges: true, deathReel: true })), false);
  const fullDemo = buildEditRequest({ ...FULL_DEMO_EDIT, playBadges: true, statsCard: true });
  assert.equal('play_badges' in fullDemo || 'stats_card' in fullDemo, false);
});

test('constrainEditConfig drops the badges from a death reel and the card from 16:9', () => {
  assert.equal(constrainEditConfig(edit({ deathReel: true, playBadges: true })).playBadges, undefined);
  assert.equal(constrainEditConfig(edit({ format: 'landscape-16x9', statsCard: true })).statsCard, undefined);
  assert.equal(constrainEditConfig(edit({ statsCard: true })).statsCard, true);
});

test('a rendered edit keeps its overlays and replay speed', () => {
  const parsed = parseEffectiveEditConfig(buildEditRequest(edit({ victimReplay: true, victimReplaySpeed: 1, playBadges: true, statsCard: true })));
  assert.equal(parsed?.victimReplaySpeed, 1);
  assert.equal(parsed?.playBadges, true);
  assert.equal(parsed?.statsCard, true);
  assert.equal(parseEffectiveEditConfig({ ...buildEditRequest(edit({ victimReplay: true })), victim_replay_speed: 0.75 })?.victimReplaySpeed, undefined);
});

test('the post-plant win chance travels for a kill Short and comes back from a render', () => {
  const body = buildEditRequest(edit({ winProbability: true, format: 'landscape-16x9' }));
  assert.equal(body.win_probability, true);
  assert.equal(parseEffectiveEditConfig(body)?.winProbability, true);
  assert.equal('win_probability' in buildEditRequest(edit({ winProbability: true, deathReel: true })), false);
  assert.equal('win_probability' in buildEditRequest({ ...FULL_DEMO_EDIT, winProbability: true }), false);
  assert.equal(constrainEditConfig(edit({ deathReel: true, winProbability: true })).winProbability, undefined);
});

test('player labels and grenade trails travel for plays, a death reel included, and come back from a render', () => {
  const body = buildEditRequest(edit({ playerLabels: true, grenadeTrails: true, deathReel: true }));
  assert.equal(body.player_labels, true);
  assert.equal(body.grenade_trails, true);
  const parsed = parseEffectiveEditConfig(body);
  assert.equal(parsed?.playerLabels, true);
  assert.equal(parsed?.grenadeTrails, true);
  const recap = buildEditRequest(edit({ format: 'landscape-16x9', matchRecap: true, playerLabels: true, grenadeTrails: true }));
  assert.equal('player_labels' in recap || 'grenade_trails' in recap, false);
  const fullDemo = buildEditRequest({ ...FULL_DEMO_EDIT, playerLabels: true, grenadeTrails: true });
  assert.equal('player_labels' in fullDemo || 'grenade_trails' in fullDemo, false);
});
