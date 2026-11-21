import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamEditPlan, StreamRenderState } from '../api/streams.ts';
import { EDIT_PLAN_SCHEMA_VERSION } from './plan.ts';
import { CREATE_FLOW_STEPS } from '../produce/create-flow.ts';
import {
  STREAM_WORKFLOW_STEPS,
  streamCtaLabel,
  streamEditorSteps,
  streamFooterLabels,
  streamOutputSummary,
  type StreamStep,
  streamPlanBlocker,
  streamTimelineLabel,
} from './editor.ts';
import { joinStreamMoments, streamPlanShorts } from './groups.ts';

test('the editor rail continues the landing workflow with the same names and numbers', () => {
  assert.deepEqual(STREAM_WORKFLOW_STEPS, CREATE_FLOW_STEPS.stream);
  const steps = streamEditorSteps({ plan: plan(), renderState: null, stale: false });
  assert.deepEqual(
    steps.map((step) => [step.number, step.label]),
    STREAM_WORKFLOW_STEPS.slice(1).map((label, index) => [String(index + 2), label]),
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
test('the editor shows every step; saving opens only once there is an export', () => {
  const state = { plan: plan(), renderState: null, stale: false };
  assert.deepEqual(
    streamEditorSteps(state).map((s) => [s.key, s.done, s.disabled]),
    [
      ['cuts', true, false],
      ['layout', true, false],
      ['review', false, false],
      ['results', false, true],
    ],
  );
  const rendering = streamEditorSteps({ ...state, rendering: true });
  assert.equal(rendering.at(-1)?.disabled, false);
  assert.equal(rendering.at(-1)?.done, false);
  const rendered = { ...state, renderState: { status: 'rendered' as const, videos: [{ clip_id: 'c1', key: 'v' }] } };
  assert.equal(streamEditorSteps(rendered).at(-1)?.done, true);
  assert.equal(streamEditorSteps({ ...rendered, stale: true }).at(-1)?.done, false);
});
test('the rail counts momentos in Momentos and Shorts only from Revisar on', () => {
  const joined = joinStreamMoments(plan(), ['c1', 'c2']);
  const detail = (key: string) =>
    streamEditorSteps({ plan: joined, renderState: null, stale: false }).find((s) => s.key === key)?.detail;
  assert.equal(detail('cuts'), '2 momentos');
  assert.equal(detail('review'), '1 Short');
});
test('the footer counts momentos until Aspecto and Shorts from Revisar on', () => {
  const joined = joinStreamMoments(plan(), ['c1', 'c2']);
  const footer = (step: StreamStep) => streamFooterLabels({ step, plan: joined, stale: false, sourceDuration: 600 });
  assert.equal(footer('cuts').count, '2 momentos · 0:36 en total');
  assert.equal(footer('layout').count, '2 momentos · 0:36 en total');
  assert.doesNotMatch(footer('layout').summary, /\d/);
  assert.equal(footer('review').count, '1 Short');
  assert.equal(footer('results').summary, '01 · 0:36');
});
test('in Momentos the footer names the momento to fix in place of the hint while one does not fit', () => {
  const broken = plan({ clips: [{ id: 'a', start_seconds: 10, end_seconds: 40 }, { id: 'b', start_seconds: 590, end_seconds: 640 }] });
  const fine = streamFooterLabels({ step: 'cuts', plan: plan(), stale: false, sourceDuration: 600 }).summary;
  const issue = streamFooterLabels({ step: 'cuts', plan: broken, stale: false, sourceDuration: 600 }).summary;
  assert.match(issue, /momento 2\b/);
  assert.notEqual(issue, fine);
  assert.doesNotMatch(fine, /momento \d/);
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
  assert.equal(streamCtaLabel({ ...base, rendering: true, queued: true }), 'En cola…');
});
test('an export says it is queued until a worker claims the render', () => {
  const detail = (renderState: StreamRenderState | null) =>
    streamEditorSteps({ plan: plan(), renderState, stale: false, rendering: true }).at(-1)?.detail;
  assert.equal(detail(null), 'En cola…');
  assert.equal(detail({ status: 'queued', videos: [] }), 'En cola…');
  assert.equal(detail({ status: 'rendering', videos: [] }), 'Creando vídeos…');
});
test('the summary shows output durations, including speed, and unsaved exports', () => {
  assert.equal(streamOutputSummary(plan(), false), '01 · 0:17, 02 · 0:19');
  assert.match(streamOutputSummary(plan(), true), /Cambios sin exportar/);
  assert.match(streamOutputSummary(plan({ clips: [] }), false), /elige tu primer momento/);
});

test('the timeline heading follows the monitor mode and names the Short out of all of them', () => {
  const shorts = streamPlanShorts(plan());
  assert.equal(streamTimelineLabel({ mode: 'source', shorts, clipId: 'c2' }), 'Recorrer el vídeo original');
  assert.equal(streamTimelineLabel({ mode: 'selected', shorts, clipId: 'c2' }), 'Short seleccionado · Short 2 de 2');
  assert.equal(streamTimelineLabel({ mode: 'sequence', shorts, clipId: 'c1' }), 'Todos los Shorts · ahora el Short 1 de 2');
  assert.equal(streamTimelineLabel({ mode: 'selected', shorts, clipId: null }), 'Recorrer el vídeo original');
  const joined = streamPlanShorts(joinStreamMoments(plan(), ['c1', 'c2']));
  assert.equal(streamTimelineLabel({ mode: 'selected', shorts: joined, clipId: 'c2' }), 'Short seleccionado · Short 1 de 1');
  assert.match(streamTimelineLabel({ mode: 'selected', shorts: joined, clipId: 'c2', moments: joined[0].clips }), /omento 2 de 2/);
  assert.match(streamTimelineLabel({ mode: 'sequence', shorts: joined, clipId: 'c1', moments: joined[0].clips }), /momento 1 de 2/);
});
