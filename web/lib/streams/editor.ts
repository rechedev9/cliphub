import { STREAM_VARIANTS, type StreamEditPlan, type StreamRenderState } from '../api/streams.ts';
import { affiliateFamilyLabel } from '../api/types.ts';
import { clipOutputDuration, formatStreamClock } from './plan.ts';

/** Editor rail steps, in the order the rail lists them. */
export const STREAM_STEP = {
  layout: 'layout',
  banners: 'banners',
  cuts: 'cuts',
  music: 'music',
  results: 'results',
} as const;
export type StreamStep = (typeof STREAM_STEP)[keyof typeof STREAM_STEP];

export const STREAM_STEP_LABEL = {
  layout: 'Layout',
  banners: 'Banners',
  cuts: 'Cortes → Shorts',
  music: 'Música',
  results: 'Resultados',
} as const satisfies Record<StreamStep, string>;

export type StreamStepEntry = {
  key: StreamStep;
  number: string;
  label: string;
  detail: string;
  done: boolean;
  /** Skippable: an undone optional step never blocks the render. */
  optional: boolean;
};

export function streamVariantNeedsFaceCrop(plan: StreamEditPlan): boolean {
  return STREAM_VARIANTS.find((entry) => entry.value === plan.variant)?.needsFaceCrop ?? false;
}

export function streamVariantLabel(plan: StreamEditPlan): string {
  return STREAM_VARIANTS.find((entry) => entry.value === plan.variant)?.label ?? plan.variant;
}

function cutsWord(count: number): string {
  return count === 1 ? '1 corte' : `${count} cortes`;
}

export function shortsWord(count: number): string {
  return count === 1 ? '1 Short' : `${count} Shorts`;
}

/** Rail rows derived from the plan; `musicLabel` is the catalog title when known. */
export function streamEditorSteps({
  plan,
  musicLabel,
  renderState,
  stale,
  rendering = false,
}: {
  plan: StreamEditPlan;
  musicLabel: string;
  renderState: StreamRenderState | null;
  stale: boolean;
  rendering?: boolean;
}): StreamStepEntry[] {
  const needsFace = streamVariantNeedsFaceCrop(plan);
  const faceReviewed = plan.face_crop_reviewed === true;
  const nick = plan.streamer_banner?.nick?.trim() ?? '';
  const platform = plan.streamer_banner?.platform === 'kick' ? 'Kick' : 'Twitch';
  const affiliate = plan.keydrop_banner?.style?.trim()
    ? ` · ${affiliateFamilyLabel(plan.keydrop_banner?.family ?? '', plan.keydrop_banner?.style ?? '') || 'Afiliado'}`
    : '';
  const count = plan.clips.length;
  const grade = plan.effects?.grade ? ' · grade' : '';
  const hasMusic = (plan.music?.key?.trim() ?? '') !== '';

  const steps: StreamStepEntry[] = [
    {
      key: STREAM_STEP.layout,
      number: '01',
      label: STREAM_STEP_LABEL.layout,
      detail: needsFace
        ? `${streamVariantLabel(plan)} · ${faceReviewed ? 'recorte ✓' : 'sin confirmar'}`
        : streamVariantLabel(plan),
      done: !needsFace || faceReviewed,
      optional: false,
    },
    {
      key: STREAM_STEP.banners,
      number: '02',
      label: STREAM_STEP_LABEL.banners,
      detail: `${nick ? `@${nick} · ${platform}` : 'sin banner'}${affiliate}`,
      done: nick !== '' || affiliate !== '',
      optional: true,
    },
    {
      key: STREAM_STEP.cuts,
      number: '03',
      label: STREAM_STEP_LABEL.cuts,
      detail: `${cutsWord(count)} → ${shortsWord(count)}`,
      done: count > 0,
      optional: false,
    },
    {
      key: STREAM_STEP.music,
      number: '04',
      label: STREAM_STEP_LABEL.music,
      detail: `${hasMusic ? musicLabel : 'Sin música'}${grade}`,
      done: hasMusic,
      optional: true,
    },
  ];
  if (rendering) {
    steps.push({
      key: STREAM_STEP.results,
      number: '05',
      label: STREAM_STEP_LABEL.results,
      detail: 'Renderizando…',
      done: false,
      optional: false,
    });
  } else if (renderState && (renderState.status === 'rendered' || renderState.published)) {
    steps.push({
      key: STREAM_STEP.results,
      number: '05',
      label: STREAM_STEP_LABEL.results,
      detail: `${shortsWord(renderState.videos.length)}${stale ? ' · desactualizados' : ''}`,
      done: true,
      optional: false,
    });
  }
  return steps;
}

/** `01 · 0:17 — 02 · 0:19`, or the nudge to add a first cut. */
export function streamOutputSummary(plan: StreamEditPlan, stale: boolean): string {
  if (plan.clips.length === 0) return 'Añade un corte en la timeline · cada corte es un Short';
  const parts = plan.clips.map(
    (clip, index) => `${String(index + 1).padStart(2, '0')} · ${formatStreamClock(clipOutputDuration(clip))}`,
  );
  if (stale) parts.push('plan cambiado desde el último render');
  // A double-space separator collapses under HTML whitespace rules, so each
  // clip's group reads as one run with no visible boundary; use a real glyph.
  return parts.join(' — ');
}

export type StreamCtaState = {
  plan: StreamEditPlan;
  briefApproved: boolean;
  rendering: boolean;
  hasRender: boolean;
};

/** Footer CTA copy: the label names the next blocker before it names the action. */
export function streamCtaLabel({ plan, briefApproved, rendering, hasRender }: StreamCtaState): string {
  if (rendering) return 'Renderizando…';
  const blocker = streamPlanBlocker(plan);
  if (blocker === STREAM_STEP.layout) return 'Confirma el recorte primero';
  if (blocker === STREAM_STEP.cuts) return 'Añade un corte primero';
  if (!briefApproved) return 'Aprueba el brief';
  return hasRender ? 'Crear Shorts de nuevo →' : 'Crear Shorts →';
}

/** The brief can only be approved once the plan is renderable in principle. */
export function streamBriefCanBeApproved(plan: StreamEditPlan): boolean {
  return streamPlanBlocker(plan) === null;
}

/** Steps that can block a render; a new member forces every consumer to name its hint. */
export type StreamBlocker = typeof STREAM_STEP.layout | typeof STREAM_STEP.cuts;

/** The first step that still blocks a render, or null when the plan is renderable. */
export function streamPlanBlocker(plan: StreamEditPlan): StreamBlocker | null {
  if (streamVariantNeedsFaceCrop(plan) && plan.face_crop_reviewed !== true) return STREAM_STEP.layout;
  if (plan.clips.length === 0) return STREAM_STEP.cuts;
  return null;
}

/**
 * Where the footer CTA navigates when it names a blocker instead of acting,
 * so "Confirma el recorte primero" is a real link rather than a dead end;
 * null once the CTA's own action applies.
 */
export function streamCtaTarget(plan: StreamEditPlan): StreamBlocker | null {
  return streamPlanBlocker(plan);
}
