import type { StreamClipEdit, StreamClipRange, StreamEditPlan, StreamTextOverlay } from '../api/streams.ts';
import { streamPlanShorts } from './groups.ts';
import { pruneClipEdit } from './plan.ts';

export type ShortMoment = { clip: StreamClipRange; name: string };

export function shortMoments(plan: StreamEditPlan, shortId: string): ShortMoment[] {
  const short = streamPlanShorts(plan).find((s) => s.id === shortId);
  if (!short) return [];
  return short.clips.map((clip) => ({
    clip,
    name: clip.title?.trim() || `Momento ${plan.clips.indexOf(clip) + 1}`,
  }));
}

function withEdit(clip: StreamClipRange, patch: Partial<StreamClipEdit>): StreamClipRange {
  const { edit: previous, ...range } = clip;
  const edit = pruneClipEdit({ ...previous, ...patch });
  return edit ? { ...range, edit } : range;
}

const FADE_SECONDS = [0, 0.5, 1, 2] as const;
const SECONDS_FORMAT = new Intl.NumberFormat('es-ES', { maximumFractionDigits: 2 });

export type FadeChoice = { seconds: number; label: string; fits: boolean };

export function fadeChoices(fade: { current: number; other: number; outputSeconds: number }): FadeChoice[] {
  const lengths: number[] = [...FADE_SECONDS];
  if (!lengths.includes(fade.current)) lengths.push(fade.current);
  return lengths
    .sort((a, b) => a - b)
    .map((seconds) => ({
      seconds,
      label: seconds === 0 ? 'Sin fundido' : `${SECONDS_FORMAT.format(seconds)} s`,
      fits: seconds === 0 || seconds + fade.other <= fade.outputSeconds,
    }));
}

const MIN_TEXT_SECONDS = 0.5;

export function textWindow(overlay: StreamTextOverlay, clipDuration: number): { start: number; end: number } {
  return { start: overlay.start_seconds ?? 0, end: overlay.end_seconds ?? clipDuration };
}

export function textTimingPatch(drag: {
  overlay: StreamTextOverlay;
  edge: 'start' | 'end';
  seconds: number;
  clipDuration: number;
}): Partial<StreamTextOverlay> {
  const { start, end } = textWindow(drag.overlay, drag.clipDuration);
  const tenths = (value: number) => Math.round(value * 10) / 10;
  if (drag.edge === 'start') {
    const next = tenths(Math.max(0, Math.min(drag.seconds, end - MIN_TEXT_SECONDS)));
    return { start_seconds: next > 0 ? next : undefined };
  }
  const next = tenths(Math.min(drag.clipDuration, Math.max(drag.seconds, start + MIN_TEXT_SECONDS)));
  return { end_seconds: next < drag.clipDuration ? next : undefined };
}

export function withMomentEdit(
  plan: StreamEditPlan,
  change: { clipId: string; patch: Partial<StreamClipEdit> },
): StreamEditPlan {
  return { ...plan, clips: plan.clips.map((clip) => (clip.id === change.clipId ? withEdit(clip, change.patch) : clip)) };
}
