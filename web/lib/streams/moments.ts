import type { StreamClipGroup, StreamClipRange, StreamEditPlan, StreamPlay } from '../api/streams.ts';
import { momentsWord, pruneStreamGroups, restoreStreamGroup } from './groups.ts';
import { clipOutputDuration, formatStreamClock, insertClipSorted } from './plan.ts';
import { isVoiceOnly, playCoveringClips, playIsCovered, rankPlays } from './plays.ts';

export type PlayRowAction = { kind: 'add' } | { kind: 'jump'; clip: StreamClipRange };

export function playRowAction(clips: readonly StreamClipRange[], play: StreamPlay): PlayRowAction {
  const [clip] = playCoveringClips(clips, play);
  return clip ? { kind: 'jump', clip } : { kind: 'add' };
}

export function bulkAddPlays(clips: readonly StreamClipRange[], plays: readonly StreamPlay[]): StreamPlay[] {
  const taken: StreamClipRange[] = [...clips];
  const picked: StreamPlay[] = [];
  for (const play of plays) {
    if (isVoiceOnly(play) || playIsCovered(taken, play)) continue;
    picked.push(play);
    taken.push({ id: play.id, start_seconds: play.start_seconds, end_seconds: play.end_seconds });
  }
  return picked;
}

export function bulkAddLabel(count: number): string {
  if (count === 0) return 'Todas las de kill están añadidas';
  return count === 1 ? 'Añadir la que queda con kill' : `Añadir las ${count} con kill`;
}

const TOP_PLAY_COUNT = 3;

export function topPlayIds(plays: readonly StreamPlay[]): Set<string> {
  return new Set(rankPlays(plays).slice(0, TOP_PLAY_COUNT).map((play) => play.id));
}

export type NumberedMoment = { clip: StreamClipRange; number: number };

export function momentsInOrder(clips: readonly StreamClipRange[]): NumberedMoment[] {
  return [...clips]
    .sort((a, b) => a.start_seconds - b.start_seconds)
    .map((clip, index) => ({ clip, number: index + 1 }));
}

export function momentsFooterText(clips: readonly StreamClipRange[]): string {
  if (clips.length === 0) return 'Aún no has elegido ningún momento';
  const total = clips.reduce((sum, clip) => sum + clipOutputDuration(clip), 0);
  return `${momentsWord(clips.length)} · ${formatStreamClock(total)} en total`;
}

export function momentsFooterHint(count: number): string {
  return count === 0
    ? 'Necesitas al menos un momento para continuar.'
    : 'En el siguiente paso eliges cómo se ve en vertical.';
}

function momentEdgeIssue(clip: StreamClipRange, sourceDuration: number): string | null {
  if (!Number.isFinite(clip.start_seconds) || clip.start_seconds < 0) return 'empieza antes que el vídeo. Mueve su inicio más adelante.';
  if (!Number.isFinite(clip.end_seconds) || clip.end_seconds <= clip.start_seconds) return 'acaba antes de empezar. Mueve su final más adelante.';
  if (sourceDuration > 0 && clip.end_seconds > sourceDuration) return 'sigue después de que acabe el vídeo. Acerca su final.';
  return null;
}

export function momentsRangeIssue({ clips, sourceDuration }: { clips: readonly StreamClipRange[]; sourceDuration: number }): string | null {
  const numbered = momentsInOrder(clips);
  for (const { clip, number } of numbered) {
    const issue = momentEdgeIssue(clip, sourceDuration);
    if (issue !== null) return `El momento ${number} ${issue}`;
  }
  for (const [index, first] of numbered.entries()) {
    const second = numbered.slice(index + 1).find(({ clip }) => clip.start_seconds < first.clip.end_seconds && first.clip.start_seconds < clip.end_seconds);
    if (second) return `Los momentos ${first.number} y ${second.number} usan el mismo trozo del vídeo. Mueve sus bordes para que no se pisen.`;
  }
  return null;
}

export function momentLengthLabel(seconds: number): string {
  return Math.round(seconds) < 60 ? `${Math.max(0, Math.round(seconds))} s` : formatStreamClock(seconds);
}

export function playTimeLabel(play: StreamPlay): string {
  return `${formatStreamClock(play.start_seconds)} · ${momentLengthLabel(play.end_seconds - play.start_seconds)}`;
}

export function withoutMoments(clips: readonly StreamClipRange[], ids: readonly string[]): StreamClipRange[] {
  const removed = new Set(ids);
  return clips.filter((clip) => !removed.has(clip.id));
}

export type MomentRemoval = { plan: StreamEditPlan; removed: StreamClipRange[]; groups: StreamClipGroup[] };

export function removeMoments(plan: StreamEditPlan, ids: readonly string[]): MomentRemoval {
  const gone = new Set(ids);
  return {
    plan: pruneStreamGroups({ ...plan, clips: withoutMoments(plan.clips, ids) }),
    removed: plan.clips.filter((clip) => gone.has(clip.id)),
    groups: plan.groups?.filter((group) => group.clip_ids.some((id) => gone.has(id))) ?? [],
  };
}

function overlapsAny(clips: readonly StreamClipRange[], clip: StreamClipRange): boolean {
  return clips.some((other) => other.id === clip.id || (other.start_seconds < clip.end_seconds && clip.start_seconds < other.end_seconds));
}

export function undoRemoval(current: StreamEditPlan, removal: MomentRemoval): StreamEditPlan {
  const clips = removal.removed.reduce(
    (next, clip) => (overlapsAny(next, clip) ? next : insertClipSorted(next, clip)),
    current.clips,
  );
  return removal.groups.reduce(restoreStreamGroup, { ...current, clips });
}
