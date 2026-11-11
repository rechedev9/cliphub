import assert from 'node:assert/strict';
import { test } from 'node:test';
import { isBombKind, isUtilityKind, timelineTickShown } from './event-kinds.ts';

const ALL = { tlEvents: true, tlUtility: true, tlKills: true };

test('an event is a grenade, a bomb event or neither', () => {
  assert.equal(isUtilityKind('smoke'), true);
  assert.equal(isUtilityKind('plant'), false);
  assert.equal(isBombKind('defuse'), true);
  assert.equal(isBombKind('kill'), false);
});

test('each timeline setting hides only its own ticks', () => {
  assert.equal(timelineTickShown('kill', { ...ALL, tlKills: false }), false);
  assert.equal(timelineTickShown('smoke', { ...ALL, tlKills: false }), true);
  assert.equal(timelineTickShown('flash', { ...ALL, tlUtility: false }), false);
  assert.equal(timelineTickShown('plant', { ...ALL, tlEvents: false }), false);
  assert.equal(timelineTickShown('plant', { ...ALL, tlUtility: false }), true);
});
