import test from 'node:test';
import assert from 'node:assert/strict';
import type { TacticalEvent, TacticalEventKind } from '../../api/tactical.ts';
import type { TimelineEvent } from '../../tactical-timeline.ts';
import { ARC_LINGER_SECONDS, bombVisual, grenadeArc, utilityEffect, utilityVisuals } from './utility-visuals.ts';

const TICKRATE = 64;
const secondsOf = (tick: number): number => tick / TICKRATE;

function entry(kind: TacticalEventKind, seconds: number, extra: Partial<TacticalEvent> = {}): TimelineEvent {
  return {
    seconds,
    fraction: 0,
    event: { tick: seconds * TICKRATE, kind, pos: [100, 200, 10], target_pos: [0, 0, 0], ...extra },
  };
}

test('utility: a smoke blooms, holds and fades over its 18 seconds', () => {
  const start = utilityEffect('smoke', 0);
  const held = utilityEffect('smoke', 8);
  const ending = utilityEffect('smoke', 17.5);
  assert.ok(start !== null && held !== null && ending !== null);
  assert.ok(start.grow < held.grow);
  assert.equal(held.strength, 1);
  assert.ok(ending.strength < 0.5);
  assert.equal(utilityEffect('smoke', 18.5), null);
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
  assert.equal(bombVisual(events, 1, true), null);
  assert.equal(bombVisual(events, 3, false)?.state, 'dropped');
  assert.equal(bombVisual(events, 3, true), null);
  assert.equal(bombVisual(events, 12, false)?.state, 'planted');
  assert.equal(bombVisual(events, 31, false)?.state, 'defused');
  const blown = [entry('plant', 10), entry('explode', 50)];
  assert.equal(bombVisual(blown, 50.5, false)?.state, 'exploded');
  assert.equal(bombVisual(blown, 53, false), null);
});
