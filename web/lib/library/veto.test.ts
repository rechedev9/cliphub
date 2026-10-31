import test from 'node:test';
import assert from 'node:assert/strict';
import { countOf, edgeLabel, seriesVetoTemplate, vetoSequence, vetoStepsProblem } from './veto.ts';

const describe = (steps: { team: string; action: string }[] | null): string =>
  (steps ?? []).map((step) => `${step.team}:${step.action}`).join(' ');

test('the veto order matches the Go sequence for every format', () => {
  assert.equal(describe(vetoSequence('bo1', 7, 'a')), 'a:ban b:ban a:ban b:ban a:ban b:ban :decider');
  assert.equal(describe(vetoSequence('bo3', 7, 'a')), 'a:ban b:ban a:pick b:pick a:ban b:ban :decider');
  assert.equal(describe(vetoSequence('bo5', 7, 'b')), 'b:ban a:ban b:pick a:pick b:pick a:pick :decider');
  assert.equal(vetoSequence('bo5', 4, 'a'), null);
});

test('the series template keeps the maps already entered', () => {
  const steps = seriesVetoTemplate('bo3', 7, 'a', [{ team: 'b', action: 'pick', map: 'de_nuke' }]);
  assert.equal(steps.length, 7);
  assert.deepEqual(steps[0], { team: 'a', action: 'ban', map: 'de_nuke' });
  assert.equal(steps[6]?.action, 'decider');
});

test('a typed veto names its first problem', () => {
  assert.equal(vetoStepsProblem([{ team: 'a', action: 'ban', map: 'de_nuke' }], true), null);
  assert.match(vetoStepsProblem([{ team: 'a', action: 'ban', map: '' }], true) ?? '', /Falta el mapa/);
  assert.match(vetoStepsProblem([{ team: 'a', action: 'ban', map: 'de_nuke' }, { team: 'b', action: 'ban', map: 'de_nuke' }], true) ?? '', /repite/);
  assert.match(vetoStepsProblem([{ team: '', action: 'decider', map: 'de_nuke' }, { team: 'a', action: 'ban', map: 'de_x' }], true) ?? '', /último/);
  assert.match(vetoStepsProblem([{ team: 'a', action: 'ban', map: 'de_nuke' }], false) ?? '', /dos equipos/);
});

test('edges and counts read as Spanish text', () => {
  assert.equal(edgeLabel(0.0634), '+6,3 pp');
  assert.equal(edgeLabel(-0.12), '-12,0 pp');
  assert.equal(edgeLabel(0), '0,0 pp');
  assert.equal(countOf(8, 14), '8 de 14');
  assert.equal(countOf(0, 0), 'Sin rondas');
});
