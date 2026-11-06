import test from 'node:test';
import assert from 'node:assert/strict';
import { betaCdf, betaQuantile, differsFrom, estimate, ratioEstimate } from './shrinkage.ts';

const near = (a: number, b: number, tolerance = 1e-6): boolean => Math.abs(a - b) < tolerance;

test('shrinkage: the Beta CDF matches closed forms', () => {
  assert.ok(near(betaCdf(0.3, 1, 1), 0.3));
  // Beta(2, 3) CDF is 1 - (1 - x)^3 (1 + 3x).
  assert.ok(near(betaCdf(0.4, 2, 3), 1 - 0.6 ** 3 * (1 + 1.2)));
  assert.ok(near(betaQuantile(0.5, 5, 5), 0.5, 1e-9));
  assert.ok(near(betaQuantile(0.95, 1, 1), 0.95, 1e-9));
});

test('shrinkage: a rate is pulled toward the reference by its rounds', () => {
  const prior = { mean: 0.7, priorStddev: 0.1, roundStddev: 0.8 };
  const few = estimate({ kind: 'rate', value: 1.0, rounds: 16 }, prior);
  const many = estimate({ kind: 'rate', value: 1.0, rounds: 1600 }, prior);
  assert.ok(few !== null && many !== null);
  // Prior precision 100, data precision 16 / 0.64 = 25: the data weighs a fifth.
  assert.ok(near(few.weight, 0.2) && near(few.shrunk, 0.76));
  assert.ok(many.shrunk > 0.98 && many.weight > 0.95);
  assert.equal(differsFrom(few, 0.7), false);
  assert.equal(differsFrom(many, 0.7), true);
  assert.ok(few.low < few.shrunk && few.shrunk < few.high);
  assert.equal(estimate({ kind: 'rate', value: 1, rounds: 10 }, { mean: 0.7, priorStddev: 0.1 }), null);
});

test('shrinkage: a percentage uses a Beta prior and stays inside 0 to 100', () => {
  const prior = { mean: 50, priorStddev: 10 };
  // m(1-m)/s² - 1 = 24: the prior weighs as 24 duels.
  const e = estimate({ kind: 'share', successes: 6, trials: 6 }, prior);
  assert.ok(e !== null);
  assert.equal(e.raw, 100);
  assert.ok(near(e.shrunk, (100 * (12 + 6)) / 30));
  assert.ok(near(e.weight, 6 / 30));
  assert.ok(e.low > 0 && e.high < 100 && e.low < e.shrunk && e.shrunk < e.high);
  assert.equal(differsFrom(e, 50), false, 'six of six opening duels is not proof');
  const strong = estimate({ kind: 'share', successes: 400, trials: 500 }, prior);
  assert.ok(strong !== null && differsFrom(strong, 50));
  assert.equal(estimate({ kind: 'share', successes: 0, trials: 0 }, prior), null);
});

test('shrinkage: a ratio carries both intervals', () => {
  const prior = { mean: 0.7, priorStddev: 0.1, roundStddev: 0.8 };
  const kills = estimate({ kind: 'rate', value: 0.9, rounds: 200 }, prior);
  const deaths = estimate({ kind: 'rate', value: 0.6, rounds: 200 }, prior);
  assert.ok(kills !== null && deaths !== null);
  const kd = ratioEstimate(kills, deaths);
  assert.ok(kd !== null);
  assert.ok(near(kd.raw, 1.5) && near(kd.shrunk, kills.shrunk / deaths.shrunk));
  assert.ok(kd.low < kd.shrunk && kd.shrunk < kd.high);
});
