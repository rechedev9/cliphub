import test from 'node:test';
import assert from 'node:assert/strict';
import { TACTICAL_SAMPLE_FLAGS } from '../api/tactical.ts';
import type { TacticalEventKind } from '../api/tactical.ts';
import {
  DEFAULT_LAYERS,
  layerEvents,
  layerLabels,
  layerSamples,
  layerTrails,
  showAllPlayers,
  toggleLayer,
  togglePlayer,
} from './layers.ts';

const event = (kind: TacticalEventKind, actor?: number) => ({ event: { kind, actor_slot: actor } });
const events = [event('kill', 1), event('smoke', 2), event('plant', 3), event('flash', 4), event('defuse', 5)];

test('layers: utility and bomb layers drop their own events, kills stay', () => {
  const noUtility = toggleLayer(DEFAULT_LAYERS, 'utility');
  assert.deepEqual(layerEvents(events, noUtility).map((e) => e.event.kind), ['kill', 'plant', 'defuse']);
  const noBomb = toggleLayer(DEFAULT_LAYERS, 'bomb');
  assert.deepEqual(layerEvents(events, noBomb).map((e) => e.event.kind), ['kill', 'smoke', 'flash']);
});

test('layers: a hidden player takes their grenades and dot off the map', () => {
  const layers = togglePlayer(DEFAULT_LAYERS, 2);
  assert.deepEqual(layers.hiddenSlots, [2]);
  assert.deepEqual(layerEvents(events, layers).map((e) => e.event.kind), ['kill', 'plant', 'flash', 'defuse']);
  const samples = [{ slot: 1, flags: 1 }, { slot: 2, flags: 1 }];
  assert.deepEqual(layerSamples(samples, layers).map((s) => s.slot), [1]);
  assert.deepEqual(togglePlayer(layers, 2).hiddenSlots, []);
  assert.deepEqual(showAllPlayers(togglePlayer(layers, 5)).hiddenSlots, []);
});

test('layers: bomb off clears the carrier flag without touching the input', () => {
  const carrier = { slot: 1, flags: TACTICAL_SAMPLE_FLAGS.alive | TACTICAL_SAMPLE_FLAGS.hasBomb };
  const [shown] = layerSamples([carrier], toggleLayer(DEFAULT_LAYERS, 'bomb'));
  assert.equal(shown.flags, TACTICAL_SAMPLE_FLAGS.alive);
  assert.equal(carrier.flags & TACTICAL_SAMPLE_FLAGS.hasBomb, TACTICAL_SAMPLE_FLAGS.hasBomb);
});

test('layers: trails and names switch off whole, hidden players lose their trail', () => {
  const trails = new Map([[1, 'a'], [2, 'b']]);
  assert.equal(layerTrails(trails, toggleLayer(DEFAULT_LAYERS, 'trails')).size, 0);
  assert.deepEqual([...layerTrails(trails, togglePlayer(DEFAULT_LAYERS, 1)).keys()], [2]);
  assert.equal(layerTrails(trails, DEFAULT_LAYERS), trails);
  assert.equal(layerLabels(new Map([[1, 'x']]), toggleLayer(DEFAULT_LAYERS, 'names')).size, 0);
});
