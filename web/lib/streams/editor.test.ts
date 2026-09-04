import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamEditPlan } from '../api/streams.ts';
import { EDIT_PLAN_SCHEMA_VERSION } from './plan.ts';
import {
  STREAM_STEP,
  streamBlockerHint,
  streamCtaLabel,
  streamEditorSteps,
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
      [STREAM_STEP.banners, 'sin banner', false],
      [STREAM_STEP.cuts, '2 cortes → 2 Shorts', true],
      [STREAM_STEP.music, 'Sin música', false],
    ],
  );
  assert.deepEqual(
    steps.map((step) => step.optional),
    [false, true, false, true],
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
  assert.deepEqual([withRender[4]?.number, withRender[4]?.detail], ['05', '1 Short · desactualizados']);
  const inFlight = streamEditorSteps({ plan: plan(), musicLabel: '', renderState: null, stale: false, rendering: true });
  assert.deepEqual([inFlight[4]?.detail, inFlight[4]?.done], ['Renderizando…', false]);
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

test('the CTA names the first blocker, then the approval, then the action', () => {
  const base = { briefApproved: false, rendering: false, hasRender: false };
  const cases: [Parameters<typeof streamCtaLabel>[0], string][] = [
    [{ ...base, plan: plan(), rendering: true }, 'Renderizando…'],
    [{ ...base, plan: plan({ face_crop_reviewed: false }), briefApproved: true }, 'Confirma el recorte primero'],
    [{ ...base, plan: plan({ clips: [] }) }, 'Añade un corte primero'],
    [{ ...base, plan: plan() }, 'Aprueba el brief'],
    [{ ...base, plan: plan(), briefApproved: true }, 'Crear Shorts →'],
    [{ ...base, plan: plan(), briefApproved: true, hasRender: true }, 'Crear Shorts de nuevo →'],
  ];
  for (const [state, expected] of cases) assert.equal(streamCtaLabel(state), expected);
});

test('the blocker is the first unmet requirement, and names its hint', () => {
  const cases: [StreamEditPlan, StreamStep | null][] = [
    [plan(), null],
    [plan({ face_crop_reviewed: false }), STREAM_STEP.layout],
    [plan({ variant: 'streamer-fullframe-nocam', face_crop_reviewed: false }), null],
    [plan({ clips: [] }), STREAM_STEP.cuts],
    [plan({ face_crop_reviewed: false, clips: [] }), STREAM_STEP.layout],
  ];
  for (const [state, expected] of cases) assert.equal(streamPlanBlocker(state), expected);
  assert.equal(streamBlockerHint(plan()), null);
  assert.match(streamBlockerHint(plan({ face_crop_reviewed: false })) ?? '', /paso 01/);
  assert.match(streamBlockerHint(plan({ clips: [] })) ?? '', /corte/);
});

test('the output summary lists one Short per cut and flags a stale render', () => {
  assert.equal(streamOutputSummary(plan(), false), '01 · 0:17 — 02 · 0:19');
  assert.equal(
    streamOutputSummary(plan(), true),
    '01 · 0:17 — 02 · 0:19 — plan cambiado desde el último render',
  );
  assert.match(streamOutputSummary(plan({ clips: [] }), false), /Añade un corte/);
});
