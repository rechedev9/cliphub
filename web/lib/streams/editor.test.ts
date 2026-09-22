import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamEditPlan } from '../api/streams.ts';
import { EDIT_PLAN_SCHEMA_VERSION } from './plan.ts';
import {
  STREAM_STEP_LABEL,
  STREAM_WORKFLOW_STEPS,
  streamCtaLabel,
  streamEditorSteps,
  streamOutputSummary,
  streamPlanBlocker,
} from './editor.ts';

test('the editor rail continues the landing workflow with the same names and numbers', () => {
  assert.deepEqual(STREAM_WORKFLOW_STEPS, [
    'Importar vídeo',
    'Elegir momentos',
    'Ajustar aspecto',
    'Revisar y exportar',
    'Guardar vídeos',
  ]);
  const steps = streamEditorSteps({ plan: plan(), renderState: null, stale: false, rendering: true });
  assert.deepEqual(
    steps.map((step) => [step.number, step.label]),
    STREAM_WORKFLOW_STEPS.slice(1).map((label, index) => [String(index + 2), label]),
  );
  assert.deepEqual(
    steps.map((step) => step.label),
    steps.map((step) => STREAM_STEP_LABEL[step.key]),
  );
});

test('the layout step names what the chosen variant renders', () => {
  const detail = (variant: StreamEditPlan['variant']) =>
    streamEditorSteps({ plan: plan({ variant }), renderState: null, stale: false }).find((s) => s.key === 'layout')
      ?.detail;
  assert.equal(detail('streamer-vertical-stack-40-60'), 'Cámara grande');
  assert.equal(detail('streamer-vertical-stack'), 'Cámara compacta');
  assert.equal(detail('streamer-fullframe-nocam'), 'Solo juego');
});

function plan(overrides: Partial<StreamEditPlan> = {}): StreamEditPlan {
  return {
    schema_version: EDIT_PLAN_SCHEMA_VERSION,
    variant: 'streamer-vertical-stack-40-60',
    face_crop_reviewed: true,
    clips: [
      { id: 'c1', start_seconds: 7, end_seconds: 24 },
      { id: 'c2', start_seconds: 40, end_seconds: 59 },
    ],
    ...overrides,
  };
}
test('the editor starts with moments, then aspect and review; results appear only for an export', () => {
  const state = { plan: plan(), renderState: null, stale: false };
  assert.deepEqual(
    streamEditorSteps(state).map((s) => s.key),
    ['cuts', 'layout', 'review'],
  );
  assert.deepEqual(
    streamEditorSteps(state).map((s) => s.done),
    [true, true, false],
  );
  const rendering = streamEditorSteps({ ...state, rendering: true });
  assert.equal(rendering.at(-1)?.key, 'results');
  assert.equal(rendering.at(-1)?.done, false);
  const rendered = { ...state, renderState: { status: 'rendered' as const, videos: [{ clip_id: 'c1', key: 'v' }] } };
  assert.equal(streamEditorSteps(rendered).at(-1)?.done, true);
  assert.equal(streamEditorSteps({ ...rendered, stale: true }).at(-1)?.done, false);
});
test('moments must exist before camera confirmation, and no-camera layouts skip it', () => {
  assert.equal(streamPlanBlocker(plan({ clips: [], face_crop_reviewed: false })), 'cuts');
  assert.equal(streamPlanBlocker(plan({ face_crop_reviewed: false })), 'layout');
  assert.equal(streamPlanBlocker(plan({ variant: 'streamer-fullframe-nocam', face_crop_reviewed: false })), null);
});

test('a completed render without videos keeps review and saving incomplete', () => {
  const steps = streamEditorSteps({
    plan: plan(),
    renderState: { status: 'rendered', videos: [] },
    stale: false,
  });
  assert.equal(steps.find((step) => step.key === 'review')?.done, false);
  assert.equal(steps.at(-1)?.key, 'results');
  assert.equal(steps.at(-1)?.done, false);
  assert.equal(steps.at(-1)?.detail, 'Sin vídeos · vuelve a exportar');
});
test('actions describe the next step and do not re-export an unchanged result', () => {
  const base = { plan: plan(), rendering: false, hasRender: false };
  assert.equal(streamCtaLabel({ ...base, activeStep: 'cuts' }), 'Continuar al aspecto →');
  assert.equal(
    streamCtaLabel({ ...base, activeStep: 'layout', plan: plan({ face_crop_reviewed: false }) }),
    'Confirmar cámara y continuar →',
  );
  assert.equal(streamCtaLabel(base), 'Exportar 2 Shorts →');
  assert.equal(streamCtaLabel({ ...base, hasRender: true }), 'Ver vídeos terminados →');
  assert.equal(streamCtaLabel({ ...base, hasRender: true, stale: true }), 'Exportar 2 Shorts →');
  assert.equal(streamCtaLabel({ ...base, rendering: true }), 'Creando vídeos…');
});
test('the summary shows output durations, including speed, and unsaved exports', () => {
  assert.equal(streamOutputSummary(plan(), false), '01 · 0:17 — 02 · 0:19');
  assert.match(streamOutputSummary(plan(), true), /Cambios sin exportar/);
  assert.match(streamOutputSummary(plan({ clips: [] }), false), /Marca el inicio y el final/);
});
