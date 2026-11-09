import test from 'node:test';
import assert from 'node:assert/strict';
import { QUICK_TRAMOS, WHOLE_ROUND, inTramo, moveTramoEdge, quickTramoOf, roundClock, tramoFromQuery, tramoLabel, writeTramo } from './tramo.ts';

test('tramo: seconds since freeze end read as the round clock', () => {
  assert.equal(roundClock(0), '1:55');
  assert.equal(roundClock(15), '1:40');
  assert.equal(roundClock(55), '1:00');
  assert.equal(roundClock(115), '0:00');
  assert.equal(roundClock(300), '0:00');
});

test('tramo: the label always says Tramo', () => {
  assert.equal(tramoLabel({ from: 15, to: 45 }), 'Tramo 1:40 a 1:10');
  assert.equal(tramoLabel(WHOLE_ROUND), 'Tramo: toda la ronda');
  assert.equal(tramoLabel({ from: 0, to: 0, fromPlant: true }), 'Tramo desde el plant');
  for (const quick of QUICK_TRAMOS) assert.ok(tramoLabel(quick.tramo).startsWith('Tramo'));
});

test('tramo: each quick tramo is recognised and a dragged one is none of them', () => {
  for (const quick of QUICK_TRAMOS) assert.equal(quickTramoOf(quick.tramo), quick.id);
  assert.equal(quickTramoOf({ from: 15, to: 45 }), null);
  assert.equal(quickTramoOf({ from: 0, to: 115 }), 'all');
});

test('tramo: a moment counts when it falls inside', () => {
  assert.equal(inTramo({ from: 15, to: 45 }, 15, null), true);
  assert.equal(inTramo({ from: 15, to: 45 }, 46, null), false);
  assert.equal(inTramo(WHOLE_ROUND, 100, null), true);
});

test('tramo: from the plant keeps only what came after it, and nothing in a round with no plant', () => {
  const plant = { from: 0, to: 0, fromPlant: true };
  assert.equal(inTramo(plant, 70, 62), true);
  assert.equal(inTramo(plant, 50, 62), false);
  assert.equal(inTramo(plant, 70, null), false);
});

test('tramo: a handle never crosses the other nor leaves the round', () => {
  assert.deepEqual(moveTramoEdge({ from: 15, to: 45 }, 'from', 80), { from: 40, to: 45 });
  assert.deepEqual(moveTramoEdge({ from: 15, to: 45 }, 'to', 3), { from: 15, to: 20 });
  assert.deepEqual(moveTramoEdge(WHOLE_ROUND, 'to', 400), { from: 0, to: 115 });
  assert.deepEqual(moveTramoEdge({ from: 15, to: 45 }, 'from', -9), { from: 0, to: 45 });
});

test('tramo: it round-trips through the URL and a bad value is the whole round', () => {
  assert.equal(writeTramo('side=T', { from: 15, to: 45 }), 'side=T&tramo=15-45');
  assert.deepEqual(tramoFromQuery(new URLSearchParams('tramo=15-45')), { from: 15, to: 45 });
  assert.equal(writeTramo('tramo=15-45&side=T', WHOLE_ROUND), 'side=T');
  assert.equal(writeTramo('', { from: 0, to: 0, fromPlant: true }), 'tramo=plant');
  assert.equal(tramoFromQuery(new URLSearchParams('tramo=plant')).fromPlant, true);
  assert.deepEqual(tramoFromQuery(new URLSearchParams('tramo=45-15')), WHOLE_ROUND);
  assert.deepEqual(tramoFromQuery(new URLSearchParams('tramo=0-999')), WHOLE_ROUND);
});
