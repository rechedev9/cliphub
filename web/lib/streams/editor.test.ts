import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamEditPlan } from '../api/streams.ts';
import { EDIT_PLAN_SCHEMA_VERSION } from './plan.ts';
import {
  STREAM_STEP,
  streamBriefCanBeApproved,
  streamCtaLabel,
  streamCtaTarget,
  streamEditorSteps,
  streamNextStep,
  streamOutputSummary,
  streamPlanBlocker,
  type StreamStep,
} from './editor.ts';

function plan(overrides: Partial<StreamEditPlan> = {}): StreamEditPlan {
  return {
    schema_version: EDIT_PLAN_SCHEMA_VERSION,
    variant: 'streamer-vertical-stack-40-60',
    face_crop_reviewed: true,
    clips: [
      { id: 'c1', start_seconds: 7, end_seconds: 24 },
      { id: 'c2', start_seconds: 40, end_seconds: 59, edit: { speed: 1 } },
    ],
    ...overrides,
  };
}

test('rail steps describe the plan and only list results after a render', () => {
  const steps = streamEditorSteps({ plan: plan(), musicLabel: 'Night Drive', renderState: null, stale: false });
  assert.deepEqual(
    steps.map((step) => [step.key, step.detail, step.done]),
    [
      [STREAM_STEP.layout, 'Facecam 40 · recorte ✓', true],
      [STREAM_STEP.banners, 'opcional · sin banner', false],
      [STREAM_STEP.cuts, '2 cortes → 2 Shorts', true],
      [STREAM_STEP.music, 'opcional · sin música', false],
      [STREAM_STEP.review, 'brief pendiente', false],
    ],
  );
  assert.deepEqual(
    steps.map((step) => step.optional),
    [false, true, false, true, false],
  );
  const withRender = streamEditorSteps({
    plan: plan({
      streamer_banner: { nick: 'zack', platform: 'kick' },
      keydrop_banner: { family: 'KEYDROP', style: 'classic' },
      music: { key: 'm1' },
      effects: { grade: true },
    }),
    musicLabel: 'Night Drive',
    renderState: { status: 'rendered', videos: [{ clip_id: 'c1', key: 'k' }] },
    stale: true,
  });
  assert.equal(withRender[1].detail, '@zack · Kick · KeyDrop');
  assert.equal(withRender[3].detail, 'Night Drive · grade');
  assert.equal(withRender[5]?.detail, '1 Short · desactualizados');
  const inFlight = streamEditorSteps({ plan: plan(), musicLabel: '', renderState: null, stale: false, rendering: true });
  assert.deepEqual([inFlight[5]?.detail, inFlight[5]?.done], ['Renderizando…', false]);
  const approved = streamEditorSteps({ plan: plan(), musicLabel: '', renderState: null, stale: false, briefApproved: true });
  assert.deepEqual([approved[4]?.detail, approved[4]?.done], ['brief aprobado', true]);
  const blocked = streamEditorSteps({ plan: plan({ clips: [] }), musicLabel: '', renderState: null, stale: false });
  assert.equal(blocked[4]?.detail, 'faltan pasos');
});

test('steps chain in editing order and stop at review', () => {
  assert.equal(streamNextStep(STREAM_STEP.layout), STREAM_STEP.banners);
  assert.equal(streamNextStep(STREAM_STEP.music), STREAM_STEP.review);
  assert.equal(streamNextStep(STREAM_STEP.review), null);
  assert.equal(streamNextStep(STREAM_STEP.results), null);
});

test('facecam layouts stay pending until the crop is confirmed', () => {
  const pending = streamEditorSteps({
    plan: plan({ face_crop_reviewed: false }),
    musicLabel: '',
    renderState: null,
    stale: false,
  });
  assert.equal(pending[0].detail, 'Facecam 40 · sin confirmar');
  assert.equal(pending[0].done, false);
  const noCam = streamEditorSteps({
    plan: plan({ variant: 'streamer-fullframe-nocam', face_crop_reviewed: false }),
    musicLabel: '',
    renderState: null,
    stale: false,
  });
  assert.equal(noCam[0].detail, 'Full-frame');
  assert.equal(noCam[0].done, true);
});

test('the CTA names the first blocker, then the review step, then the action', () => {
  const base = { briefApproved: false, rendering: false, hasRender: false, onReview: false };
  const cases: [Parameters<typeof streamCtaLabel>[0], string][] = [
    [{ ...base, plan: plan(), rendering: true }, 'Renderizando…'],
    [{ ...base, plan: plan({ face_crop_reviewed: false }), briefApproved: true }, 'Confirma el recorte primero'],
    [{ ...base, plan: plan({ clips: [] }) }, 'Añade un corte primero'],
    [{ ...base, plan: plan() }, 'Revisar y renderizar'],
    [{ ...base, plan: plan(), hasRender: true }, 'Revisar y renderizar de nuevo'],
    [{ ...base, plan: plan(), onReview: true }, 'Aprueba el brief'],
    [{ ...base, plan: plan(), onReview: true, briefApproved: true }, 'Crear Shorts →'],
    [{ ...base, plan: plan(), onReview: true, briefApproved: true, hasRender: true }, 'Crear Shorts de nuevo →'],
  ];
  for (const [state, expected] of cases) assert.equal(streamCtaLabel(state), expected);
});

test('the CTA target is the first blocker, else the review step, else nothing', () => {
  const cases: [StreamEditPlan, StreamStep, StreamStep | null][] = [
    [plan(), STREAM_STEP.cuts, STREAM_STEP.review],
    [plan(), STREAM_STEP.review, null],
    [plan({ face_crop_reviewed: false }), STREAM_STEP.review, STREAM_STEP.layout],
    [plan({ variant: 'streamer-fullframe-nocam', face_crop_reviewed: false }), STREAM_STEP.review, null],
    [plan({ clips: [] }), STREAM_STEP.music, STREAM_STEP.cuts],
  ];
  for (const [state, active, expected] of cases) assert.equal(streamCtaTarget(state, active), expected);
  assert.equal(streamPlanBlocker(plan()), null);
  assert.equal(streamPlanBlocker(plan({ face_crop_reviewed: false, clips: [] })), STREAM_STEP.layout);
});

test('the brief is approvable only with cuts and a confirmed facecam', () => {
  assert.equal(streamBriefCanBeApproved(plan()), true);
  assert.equal(streamBriefCanBeApproved(plan({ clips: [] })), false);
  assert.equal(streamBriefCanBeApproved(plan({ face_crop_reviewed: false })), false);
  assert.equal(
    streamBriefCanBeApproved(plan({ variant: 'streamer-fullframe-nocam', face_crop_reviewed: false })),
    true,
  );
});

test('the output summary lists one Short per cut and flags a stale render', () => {
  assert.equal(streamOutputSummary(plan(), false), '01 · 0:17 — 02 · 0:19');
  assert.equal(
    streamOutputSummary(plan(), true),
    '01 · 0:17 — 02 · 0:19 — plan cambiado desde el último render',
  );
  assert.match(streamOutputSummary(plan({ clips: [] }), false), /Añade un corte/);
});
