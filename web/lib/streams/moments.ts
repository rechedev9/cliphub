import type { StreamClipRange, StreamPlay } from '../api/streams.ts';
import { clipOutputDuration, formatStreamClock } from './plan.ts';
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
  return count === 1 ? 'Añadir 1 con kill' : `Añadir las ${count} con kill`;
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
  const count = clips.length === 1 ? '1 momento' : `${clips.length} momentos`;
  return `${count} · ${formatStreamClock(total)} en total`;
}

export function momentsFooterHint(count: number): string {
  return count === 0
    ? 'Necesitas al menos un momento para continuar.'
    : 'En el siguiente paso eliges cómo se ve en vertical.';
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
