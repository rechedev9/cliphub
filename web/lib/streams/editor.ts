import { STREAM_VARIANTS, type StreamEditPlan, type StreamRenderState } from '../api/streams.ts';
import { clipOutputDuration, formatStreamClock } from './plan.ts';

export const STREAM_STEP = { cuts: 'cuts', layout: 'layout', review: 'review', results: 'results' } as const;
export type StreamStep = (typeof STREAM_STEP)[keyof typeof STREAM_STEP];
export const STREAM_STEP_LABEL = {
  cuts: 'Elegir momentos',
  layout: 'Ajustar aspecto',
  review: 'Revisar y exportar',
  results: 'Guardar vídeos',
} as const satisfies Record<StreamStep, string>;
export const STREAM_IMPORT_STEP_LABEL = 'Importar vídeo';
/**
 * The whole stream workflow as the /streams landing numbers it. Import is step
 * 1 there, so the editor rail continues at 2 with the same names.
 */
export const STREAM_WORKFLOW_STEPS: readonly string[] = [
  STREAM_IMPORT_STEP_LABEL,
  STREAM_STEP_LABEL.cuts,
  STREAM_STEP_LABEL.layout,
  STREAM_STEP_LABEL.review,
  STREAM_STEP_LABEL.results,
];
function workflowNumber(step: StreamStep): string {
  return String(STREAM_WORKFLOW_STEPS.indexOf(STREAM_STEP_LABEL[step]) + 1);
}
export type StreamStepEntry = {
  key: StreamStep;
  number: string;
  label: string;
  detail: string;
  done: boolean;
  optional: boolean;
};
export function streamVariantNeedsFaceCrop(plan: StreamEditPlan): boolean {
  return STREAM_VARIANTS.find((entry) => entry.value === plan.variant)?.needsFaceCrop ?? false;
}
export function streamVariantLabel(plan: StreamEditPlan): string {
  return STREAM_VARIANTS.find((entry) => entry.value === plan.variant)?.label ?? plan.variant;
}
export function shortsWord(count: number): string {
  return count === 1 ? '1 Short' : `${count} Shorts`;
}

export function streamEditorSteps({
  plan,
  renderState,
  stale,
  rendering = false,
}: {
  plan: StreamEditPlan;
  musicLabel?: string;
  renderState: StreamRenderState | null;
  stale: boolean;
  rendering?: boolean;
}): StreamStepEntry[] {
  const faceReady = !streamVariantNeedsFaceCrop(plan) || plan.face_crop_reviewed === true;
  const videoCount = renderState?.videos?.length ?? 0;
  const steps: StreamStepEntry[] = [
    {
      key: 'cuts',
      number: workflowNumber('cuts'),
      label: STREAM_STEP_LABEL.cuts,
      detail: shortsWord(plan.clips.length),
      done: plan.clips.length > 0,
      optional: false,
    },
    {
      key: 'layout',
      number: workflowNumber('layout'),
      label: STREAM_STEP_LABEL.layout,
      detail: faceReady ? streamVariantLabel(plan) : 'Ajusta la cámara',
      done: faceReady,
      optional: false,
    },
    {
      key: 'review',
      number: workflowNumber('review'),
      label: STREAM_STEP_LABEL.review,
      detail: 'Un vídeo por momento',
      done: videoCount > 0 && !stale,
      optional: false,
    },
  ];
  if (rendering || renderState?.status === 'rendered' || renderState?.published) {
    let resultDetail = shortsWord(videoCount);
    if (!videoCount) resultDetail = 'Sin vídeos · vuelve a exportar';
    else if (stale) resultDetail = 'Hay cambios sin exportar';
    steps.push({
      key: 'results',
      number: workflowNumber('results'),
      label: STREAM_STEP_LABEL.results,
      detail: rendering ? 'Creando vídeos…' : resultDetail,
      done: videoCount > 0 && !rendering && !stale,
      optional: false,
    });
  }
  return steps;
}
export function streamOutputSummary(plan: StreamEditPlan, stale: boolean): string {
  if (!plan.clips.length) return 'Marca el inicio y el final de tu primer momento';
  return (
    plan.clips
      .map((clip, index) => `${String(index + 1).padStart(2, '0')} · ${formatStreamClock(clipOutputDuration(clip))}`)
      .join(' — ') + (stale ? ' · Cambios sin exportar' : '')
  );
}
export type StreamBlocker = 'cuts' | 'layout';
export function streamPlanBlocker(plan: StreamEditPlan): StreamBlocker | null {
  if (!plan.clips.length) return 'cuts';
  if (streamVariantNeedsFaceCrop(plan) && plan.face_crop_reviewed !== true) return 'layout';
  return null;
}
export function streamBlockerHint(plan: StreamEditPlan): string | null {
  const blocker = streamPlanBlocker(plan);
  if (blocker === 'cuts') return 'Añade un momento para continuar';
  if (blocker === 'layout') return 'Ajusta y confirma la cámara antes de exportar';
  return null;
}
export function streamCtaLabel({
  plan,
  rendering,
  hasRender,
  activeStep = 'review',
  stale = false,
}: {
  plan: StreamEditPlan;
  rendering: boolean;
  hasRender: boolean;
  activeStep?: StreamStep;
  stale?: boolean;
}): string {
  if (rendering) return 'Creando vídeos…';
  if (activeStep === 'cuts') return 'Continuar al aspecto →';
  if (activeStep === 'layout')
    return streamVariantNeedsFaceCrop(plan) && !plan.face_crop_reviewed
      ? 'Confirmar cámara y continuar →'
      : 'Revisar Shorts →';
  if (hasRender && !stale) return 'Ver vídeos terminados →';
  if (streamPlanBlocker(plan) === 'cuts') return 'Elegir momentos →';
  if (streamPlanBlocker(plan) === 'layout') return 'Ajustar cámara →';
  return `Exportar ${shortsWord(plan.clips.length)} →`;
}
