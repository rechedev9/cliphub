// The colour of a comparison is a claim ("better", "worse"); these pin when it is made.
import test from 'node:test';
import assert from 'node:assert/strict';
import { compareDelta, heatLevel, weightedMean } from './compare.ts';
import { linePath, niceTicks, seriesPoints } from './chart.ts';

test('a delta is good news in the direction the stat improves', () => {
  assert.equal(compareDelta(1.2, 1, { higherIsBetter: true }).tone, 'better');
  assert.equal(compareDelta(1.2, 1, { higherIsBetter: false }).tone, 'worse');
  assert.equal(compareDelta(1.004, 1, { higherIsBetter: true, epsilon: 0.005 }).tone, 'even');
});

test('a heat level grows with the gap and stops at the strongest step', () => {
  assert.equal(heatLevel(0.5, 0.5, 0.15, true), 0);
  assert.equal(heatLevel(0.55, 0.5, 0.15, true), 1);
  assert.equal(heatLevel(0.9, 0.5, 0.15, true), 3);
  assert.equal(heatLevel(0.9, 0.5, 0.15, false), -3);
  assert.equal(heatLevel(Number.NaN, 0.5, 0.15, true), 0);
});

test('a weighted mean ignores rows without weight', () => {
  const rows = [{ v: 1, n: 30 }, { v: 0, n: 10 }, { v: 99, n: 0 }];
  assert.equal(weightedMean(rows, (r) => r.v, (r) => r.n), 0.75);
  assert.equal(weightedMean([], (r: { v: number }) => r.v, () => 1), null);
});

test('axis ticks are round numbers that cover the data', () => {
  assert.deepEqual(niceTicks(0.62, 1.48, 4), { ticks: [0.5, 1, 1.5], domain: [0.5, 1.5] });
  const flat = niceTicks(1, 1);
  assert.ok(flat.domain[0] < 1 && flat.domain[1] > 1);
});

test('a series spans the width and a single point sits in the middle', () => {
  assert.equal(linePath(seriesPoints([0, 1], [0, 1], 100, 50)), 'M0 50 L100 0');
  assert.deepEqual(seriesPoints([2], [0, 4], 100, 40), [{ x: 50, y: 20 }]);
});
