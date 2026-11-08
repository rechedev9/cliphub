import test from 'node:test';
import assert from 'node:assert/strict';
import type { TacticalEvent, TacticalEventKind } from '../../api/tactical.ts';
import type { TimelineEvent } from '../../tactical-timeline.ts';
import { ARC_LINGER_SECONDS, bombVisual, flicker, grenadeArc, puffLayout, utilityEffect, utilityVisuals } from './utility-visuals.ts';

const TICKRATE = 64;
const secondsOf = (tick: number): number => tick / TICKRATE;

function entry(kind: TacticalEventKind, seconds: number, extra: Partial<TacticalEvent> = {}): TimelineEvent {
  return {
    seconds,
    fraction: 0,
    event: { tick: seconds * TICKRATE, kind, pos: [100, 200, 10], target_pos: [0, 0, 0], ...extra },
  };
}

test('utility: a smoke blooms, holds for 20 seconds and fades by 22, as in game', () => {
  const start = utilityEffect('smoke', 0);
  const held = utilityEffect('smoke', 19.5);
  const ending = utilityEffect('smoke', 21.5);
  assert.ok(start !== null && held !== null && ending !== null);
  assert.ok(start.grow < held.grow);
  assert.equal(held.strength, 1);
  assert.ok(ending.strength < 0.5);
  assert.equal(utilityEffect('smoke', 22.5), null);
  assert.equal(utilityEffect('smoke', -0.1), null);
  assert.equal(utilityEffect('kill', 0), null);
});

test('utility: flashes and HE are brief pulses', () => {
  const flash = utilityEffect('flash', 0.3);
  assert.ok(flash !== null && flash.strength > 0 && flash.strength < 1);
  assert.equal(utilityEffect('he', 1), null);
});

test('utility: a throw shows its path in flight and for a moment after landing', () => {
  const thrown = entry('smoke', 10, { throw: { tick: 8 * TICKRATE, pos: [0, 0, 64], yaw: 0, pitch: -10 } });
  const events = [entry('kill', 5), thrown];
  assert.deepEqual(utilityVisuals(events, 7, secondsOf), []);
  const flying = utilityVisuals(events, 9, secondsOf);
  assert.equal(flying.length, 1);
  assert.equal(flying[0].index, 1);
  assert.equal(flying[0].effect, null);
  assert.equal(flying[0].arc?.progress, 0.5);
  const landed = utilityVisuals(events, 10 + ARC_LINGER_SECONDS / 2, secondsOf);
  assert.equal(landed[0].arc?.progress, 1);
  assert.equal(landed[0].arc?.fade, 0.5);
  assert.ok(landed[0].effect !== null);
  const later = utilityVisuals(events, 15, secondsOf);
  assert.equal(later[0].arc, null);
});

test('utility: an old analysis without throws still shows the effect', () => {
  const visuals = utilityVisuals([entry('molotov', 3)], 4, secondsOf);
  assert.equal(visuals.length, 1);
  assert.equal(visuals[0].arc, null);
  assert.deepEqual(visuals[0].at, { x: 100, y: 200, z: 10 });
});

test('utility: a fire put out by a smoke goes when the demo says it went out', () => {
  const fire = entry('molotov', 3, { end_tick: 4 * TICKRATE });
  assert.equal(utilityVisuals([fire], 3.5, secondsOf).length, 1);
  assert.deepEqual(utilityVisuals([fire], 4.5, secondsOf), []);
  const smoke = entry('smoke', 0, { end_tick: 12 * TICKRATE });
  const thinning = utilityVisuals([smoke], 11.5, secondsOf)[0].effect;
  assert.ok(thinning !== null && thinning.strength < 0.5);
  assert.deepEqual(utilityVisuals([smoke], 13, secondsOf), []);
});

test('utility: a decoy keeps sounding for 15 seconds', () => {
  assert.equal(utilityVisuals([entry('decoy', 0)], 10, secondsOf).length, 1);
  assert.deepEqual(utilityVisuals([entry('decoy', 0)], 16, secondsOf), []);
});

test('arc: starts and ends at the throw and the landing, peaking above both', () => {
  const from = { x: 0, y: 0, z: 64 };
  const to = { x: 1000, y: 0, z: 0 };
  const points = grenadeArc(from, to, 16);
  assert.equal(points.length, 17);
  assert.deepEqual(points[0], from);
  assert.deepEqual(points[16], to);
  assert.ok(Math.max(...points.map((point) => point.z)) > 64);
});

test('bomb: planted until defused or exploded, a drop only while nobody carries it', () => {
  const events = [entry('bomb_drop', 2), entry('plant', 10), entry('defuse', 30)];
  const at = (now: number, carried: boolean, list = events) => bombVisual({ events: list, nowSeconds: now, carried, secondsOf });
  assert.equal(at(1, true), null);
  assert.equal(at(3, false)?.state, 'dropped');
  assert.equal(at(3, true), null);
  assert.equal(at(12, false)?.state, 'planted');
  assert.equal(at(31, false)?.state, 'defused');
  const blown = [entry('plant', 10), entry('explode', 50)];
  assert.equal(at(50.5, false, blown)?.state, 'exploded');
  assert.equal(at(53, false, blown), null);
});

test('bomb: a drop lies there until its pickup, whoever is drawn', () => {
  // Picked up at 6 s by a player the viewer hid: no carrier among the drawn samples.
  const events = [entry('bomb_drop', 2, { end_tick: 6 * TICKRATE })];
  assert.equal(bombVisual({ events, nowSeconds: 4, carried: true, secondsOf })?.state, 'dropped');
  assert.equal(bombVisual({ events, nowSeconds: 7, carried: false, secondsOf }), null);
});

test('bomb: an old analysis without a plant does not blow up at the world origin', () => {
  const events = [entry('explode', 50, { pos: [0, 0, 0] })];
  assert.equal(bombVisual({ events, nowSeconds: 50.5, carried: false, secondsOf }), null);
});

test('smoke puffs: the same grenade always gets the same cloud, inside its dome', () => {
  const puffs = puffLayout(7, 14);
  assert.deepEqual(puffs, puffLayout(7, 14));
  assert.notDeepEqual(puffs, puffLayout(8, 14));
  for (const puff of puffs) {
    assert.ok(Math.hypot(puff.x, puff.y) <= 0.78 + 1e-9);
    assert.ok(puff.z >= 0 && puff.z <= 0.8);
    assert.ok(puff.size > 0.7 && puff.size < 1.5);
  }
});

test('fire: flames flicker within their range and differ by phase', () => {
  for (let t = 0; t < 5; t += 0.05) {
    const height = flicker(t, 1.3);
    assert.ok(height >= 0.6 && height <= 1.2);
  }
  assert.notEqual(flicker(1, 0), flicker(1, 2));
});
