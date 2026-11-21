import { STREAM_VARIANTS, type StreamEditPlan, type StreamRenderState } from '../api/streams.ts';
import { CREATE_FLOW_STEPS } from '../produce/create-flow.ts';
import { formatStreamClock } from './plan.ts';
import { momentsWord, streamPlanShorts, streamShortCount, type StreamShort } from './groups.ts';
import { momentsFooterHint, momentsFooterText, momentsRangeIssue } from './moments.ts';
import type { StreamPlaybackMode } from '../stream-playback.ts';

export const STREAM_STEP = { cuts: 'cuts', layout: 'layout', review: 'review', results: 'results' } as const;
export type StreamStep = (typeof STREAM_STEP)[keyof typeof STREAM_STEP];
export const STREAM_STEP_LABEL = {
  cuts: 'Elegir momentos',
  layout: 'Ajustar aspecto',
  review: 'Revisar y exportar',
  results: 'Guardar vídeos',
} as const satisfies Record<StreamStep, string>;
/** The /streams landing rail and the editor rail share these names; import is step 1. */
export const STREAM_WORKFLOW_STEPS: readonly string[] = CREATE_FLOW_STEPS.stream;
const STREAM_STEP_ORDER: readonly StreamStep[] = ['cuts', 'layout', 'review', 'results'];
function workflowEntry(step: StreamStep): { number: string; label: string } {
  const index = STREAM_STEP_ORDER.indexOf(step) + 1;
  return { number: String(index + 1), label: STREAM_WORKFLOW_STEPS[index] ?? STREAM_STEP_LABEL[step] };
}
export type StreamStepEntry = {
  key: StreamStep;
  number: string;
  label: string;
  detail: string;
  done: boolean;
  optional: boolean;
  disabled: boolean;
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
/** No worker has claimed the render yet: renders wait while a capture runs. */
export function streamRenderQueued(renderState: StreamRenderState | null): boolean {
  return renderState === null || renderState.status === 'queued';
}
function streamRenderBusyLabel(queued: boolean): string {
  return queued ? 'En cola…' : 'Creando vídeos…';
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
      ...workflowEntry('cuts'),
      detail: momentsWord(plan.clips.length),
      done: plan.clips.length > 0,
      optional: false,
      disabled: false,
    },
    {
      key: 'layout',
      ...workflowEntry('layout'),
      detail: faceReady ? streamVariantLabel(plan) : 'Ajusta la cámara',
      done: faceReady,
      optional: false,
      disabled: false,
    },
    {
      key: 'review',
      ...workflowEntry('review'),
      detail: shortsWord(streamShortCount(plan)),
      done: videoCount > 0 && !stale,
      optional: false,
      disabled: false,
    },
  ];
  const hasResults = rendering || renderState?.status === 'rendered' || renderState?.published === true;
  let resultDetail = shortsWord(videoCount);
  if (!hasResults) resultDetail = 'Después de exportar';
  else if (rendering) resultDetail = streamRenderBusyLabel(streamRenderQueued(renderState));
  else if (!videoCount) resultDetail = 'Sin vídeos · vuelve a exportar';
  else if (stale) resultDetail = 'Hay cambios sin exportar';
  steps.push({
    key: 'results',
    ...workflowEntry('results'),
    detail: resultDetail,
    done: hasResults && videoCount > 0 && !rendering && !stale,
    optional: false,
    disabled: !hasResults,
  });
  return steps;
}
export function streamOutputSummary(plan: StreamEditPlan, stale: boolean): string {
  if (!plan.clips.length) return 'Vuelve a Momentos y elige tu primer momento';
  return (
    streamPlanShorts(plan)
      .map((short, index) => `${String(index + 1).padStart(2, '0')} · ${formatStreamClock(short.durationSeconds)}`)
      .join(', ') + (stale ? ' · Cambios sin exportar' : '')
  );
}
export function streamFooterLabels({
  step,
  plan,
  stale,
  sourceDuration,
}: {
  step: StreamStep;
  plan: StreamEditPlan;
  stale: boolean;
  sourceDuration: number;
}): { count: string; summary: string } {
  if (step === STREAM_STEP.cuts) {
    const issue = momentsRangeIssue({ clips: plan.clips, sourceDuration });
    return { count: momentsFooterText(plan.clips), summary: issue ?? momentsFooterHint(plan.clips.length) };
  }
  if (step === STREAM_STEP.layout) {
    return { count: momentsFooterText(plan.clips), summary: 'En el siguiente paso revisas tus Shorts antes de exportar.' };
  }
  return { count: shortsWord(streamShortCount(plan)), summary: streamOutputSummary(plan, stale) };
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
  queued = false,
  hasRender,
  activeStep = 'review',
  stale = false,
}: {
  plan: StreamEditPlan;
  rendering: boolean;
  queued?: boolean;
  hasRender: boolean;
  activeStep?: StreamStep;
  stale?: boolean;
}): string {
  if (rendering) return streamRenderBusyLabel(queued);
  if (activeStep === 'cuts') return 'Continuar al aspecto →';
  if (activeStep === 'layout')
    return streamVariantNeedsFaceCrop(plan) && !plan.face_crop_reviewed
      ? 'Confirmar cámara y continuar →'
      : 'Revisar Shorts →';
  if (hasRender && !stale) return 'Ver vídeos terminados →';
  if (streamPlanBlocker(plan) === 'cuts') return 'Elegir momentos →';
  if (streamPlanBlocker(plan) === 'layout') return 'Ajustar cámara →';
  return `Exportar ${shortsWord(streamShortCount(plan))} →`;
}

/** The source timeline's heading: what the monitor plays and, for a Short, which one of how many. */
export function streamTimelineLabel({
  mode,
  shorts,
  clipId,
}: {
  mode: StreamPlaybackMode;
  shorts: readonly StreamShort[];
  clipId: string | null;
}): string {
  const index = shorts.findIndex((short) => short.clips.some((clip) => clip.id === clipId));
  if (mode === 'source' || index < 0) return 'Recorrer el vídeo original';
  const position = `Short ${index + 1} de ${shorts.length}`;
  return mode === 'sequence' ? `Todos los Shorts · ahora el ${position}` : `Short seleccionado · ${position}`;
}
