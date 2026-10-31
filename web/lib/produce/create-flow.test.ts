import test from 'node:test';
import assert from 'node:assert/strict';
import { CREATE_FLOW, CREATE_FLOW_STEPS, CREATE_PHASES, createFlowPhases, type CreateFlow } from './create-flow.ts';

const FLOWS: readonly CreateFlow[] = Object.values(CREATE_FLOW);

test('each flow lays every one of its own steps into source, settings and result', () => {
  for (const flow of FLOWS) {
    const phases = createFlowPhases(flow, 0);
    assert.deepEqual(phases.map((phase) => phase.label), [...CREATE_PHASES], flow);
    assert.deepEqual(phases.flatMap((phase) => phase.steps), [...CREATE_FLOW_STEPS[flow]], flow);
    for (const phase of phases) assert.ok(phase.steps.length > 0, `${flow} ${phase.label} is empty`);
  }
});

test('picking the player is still the source phase of a demo flow; preparing is settings', () => {
  const picking = createFlowPhases(CREATE_FLOW.short, 1);
  assert.deepEqual(picking.map((phase) => phase.state), ['current', 'pending', 'pending']);
  assert.equal(picking[0]?.detail, 'Elegir jugador');
  const preparing = createFlowPhases(CREATE_FLOW.full, 2);
  assert.deepEqual(preparing.map((phase) => phase.state), ['done', 'current', 'pending']);
  assert.equal(preparing[1]?.detail, 'Preparar demo completa');
});

test('a stream edit spans the settings phase and the export lands in the result phase', () => {
  assert.deepEqual(createFlowPhases(CREATE_FLOW.stream, 0).map((phase) => phase.state), ['current', 'pending', 'pending']);
  const layout = createFlowPhases(CREATE_FLOW.stream, 2);
  assert.deepEqual(layout.map((phase) => phase.state), ['done', 'current', 'pending']);
  assert.equal(layout[1]?.detail, 'Ajustar aspecto');
  assert.deepEqual(createFlowPhases(CREATE_FLOW.stream, 4).map((phase) => phase.state), ['done', 'done', 'current']);
});

test('an out-of-range step is clamped instead of lighting no phase', () => {
  assert.deepEqual(createFlowPhases(CREATE_FLOW.short, -3).map((phase) => phase.state), ['current', 'pending', 'pending']);
  assert.deepEqual(createFlowPhases(CREATE_FLOW.short, 99).map((phase) => phase.state), ['done', 'done', 'current']);
});
