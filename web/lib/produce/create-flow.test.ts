import test from 'node:test';
import assert from 'node:assert/strict';
import { CREATE_FLOW, createFlowSteps } from './create-flow.ts';

test('the player pick is the second step of a demo flow and the first one reads as done', () => {
  assert.deepEqual(
    createFlowSteps(CREATE_FLOW.short, 1).map((step) => [step.label, step.state]),
    [
      ['Demo', 'done'],
      ['Jugador', 'current'],
      ['Jugadas', 'pending'],
      ['Grabar', 'pending'],
    ],
  );
});

test('preparing a demo flow leaves recording as the last step still to come', () => {
  const full = createFlowSteps(CREATE_FLOW.full, 2).map((step) => [step.label, step.state]);
  assert.deepEqual(full.slice(2), [
    ['Ajustes', 'current'],
    ['Grabar', 'pending'],
  ]);
});

test('an out-of-range step is clamped instead of lighting no step', () => {
  assert.equal(createFlowSteps(CREATE_FLOW.short, -3)[0]?.state, 'current');
  assert.equal(createFlowSteps(CREATE_FLOW.short, 99).at(-1)?.state, 'current');
  assert.equal(createFlowSteps(CREATE_FLOW.stream, 4).at(-1)?.state, 'current');
});
