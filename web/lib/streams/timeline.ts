import type { StreamClipRange, StreamPlay } from '../api/streams.ts';
import { STREAM_PLAYBACK_MODE, streamClipIndexAt, type StreamPlaybackMode } from '../stream-playback.ts';
import { momentsInOrder } from './moments.ts';
import { formatStreamClock } from './plan.ts';
import { isVoiceOnly, playIsCovered } from './plays.ts';

export type TimelineWindow = { start: number; end: number };

export type PlayMarkState = 'open' | 'covered' | 'selected';

export type PlayMark = { play: StreamPlay; number: number; voice: boolean; covered: boolean; state: PlayMarkState; leftPercent: number };

export type MomentSpan = { clip: StreamClipRange; number: number; leftPercent: number; widthPercent: number; selected: boolean };

export function fullTimelineWindow(sourceDuration: number): TimelineWindow {
  return { start: 0, end: Number.isFinite(sourceDuration) && sourceDuration > 0 ? sourceDuration : 0 };
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export function timelinePercent(seconds: number, window: TimelineWindow): number {
  const span = window.end - window.start;
  if (span <= 0) return 0;
  return clamp(((seconds - window.start) / span) * 100, 0, 100);
}

export type TimelinePlayhead = { leftPercent: number | null; valueMin: number; valueMax: number; valueNow: number };

export function timelinePlayhead({ seconds, window }: { seconds: number; window: TimelineWindow }): TimelinePlayhead {
  const valueMin = Math.round(window.start);
  const valueMax = Math.max(valueMin, Math.round(window.end));
  const shown = window.end > window.start && seconds >= window.start && seconds <= window.end;
  return {
    leftPercent: shown ? timelinePercent(seconds, window) : null,
    valueMin,
    valueMax,
    valueNow: clamp(Math.round(seconds), valueMin, valueMax),
  };
}

function inWindow(start: number, end: number, window: TimelineWindow): boolean {
  return end >= window.start && start <= window.end;
}

function byStart<T extends { start_seconds: number }>(items: readonly T[]): T[] {
  return [...items].sort((a, b) => a.start_seconds - b.start_seconds);
}

export function playNumbers(plays: readonly StreamPlay[]): Map<string, number> {
  return new Map(byStart(plays).map((play, index) => [play.id, index + 1]));
}

export function timelinePlayMarks({
  plays,
  clips,
  activePlayId,
  window,
}: {
  plays: readonly StreamPlay[];
  clips: readonly StreamClipRange[];
  activePlayId: string | null;
  window: TimelineWindow;
}): PlayMark[] {
  return byStart(plays).flatMap((play, index): PlayMark[] => {
    if (!inWindow(play.start_seconds, play.start_seconds, window)) return [];
    const covered = playIsCovered(clips, play);
    let state: PlayMarkState = covered ? 'covered' : 'open';
    if (play.id === activePlayId) state = 'selected';
    return [{ play, number: index + 1, voice: isVoiceOnly(play), covered, state, leftPercent: timelinePercent(play.start_seconds, window) }];
  });
}

export function timelineMomentRanges({
  clips,
  selectedClipIds,
  window,
}: {
  clips: readonly StreamClipRange[];
  selectedClipIds: readonly string[];
  window: TimelineWindow;
}): MomentSpan[] {
  return momentsInOrder(clips).flatMap(({ clip, number }): MomentSpan[] => {
    if (clip.end_seconds <= window.start || clip.start_seconds >= window.end) return [];
    const leftPercent = timelinePercent(clip.start_seconds, window);
    const widthPercent = timelinePercent(clip.end_seconds, window) - leftPercent;
    return [{ clip, number, leftPercent, widthPercent, selected: selectedClipIds.includes(clip.id) }];
  });
}

export function playMarkLabel(mark: PlayMark): string {
  const kind = mark.voice ? 'pico de voz' : 'kill';
  const kept = mark.covered ? ', en tus momentos' : '';
  return `Jugada ${mark.number}, ${kind}: ${mark.play.label}, ${formatStreamClock(mark.play.start_seconds)}${kept}`;
}

export function momentRangeLabel(range: MomentSpan): string {
  const title = range.clip.title?.trim();
  const span = `${formatStreamClock(range.clip.start_seconds)} a ${formatStreamClock(range.clip.end_seconds)}`;
  return `Momento ${range.number}: ${title ? `${title}, ` : ''}${span}`;
}

const KEY_STEP_SECONDS = 5;
const PAGE_STEP_SECONDS = 60;

export function timelineKeySeek({ key, seconds, window }: { key: string; seconds: number; window: TimelineWindow }): number | null {
  const within = (value: number) => clamp(value, window.start, Math.max(window.start, window.end));
  switch (key) {
    case 'ArrowRight':
    case 'ArrowUp':
      return within(seconds + KEY_STEP_SECONDS);
    case 'ArrowLeft':
    case 'ArrowDown':
      return within(seconds - KEY_STEP_SECONDS);
    case 'PageUp':
      return within(seconds + PAGE_STEP_SECONDS);
    case 'PageDown':
      return within(seconds - PAGE_STEP_SECONDS);
    case 'Home':
      return window.start;
    case 'End':
      return window.end;
    default:
      return null;
  }
}

export function timelineSeekKeepsPlayback({
  mode,
  seconds,
  preview,
  selected,
  clips,
}: {
  mode: StreamPlaybackMode;
  seconds: number;
  preview: StreamClipRange | null;
  selected: StreamClipRange | undefined;
  clips: readonly StreamClipRange[];
}): boolean {
  if (mode === STREAM_PLAYBACK_MODE.sequence) return streamClipIndexAt(clips, seconds) >= 0;
  if (mode !== STREAM_PLAYBACK_MODE.selected) return false;
  const playing = preview ?? selected;
  return playing !== undefined && seconds >= playing.start_seconds && seconds <= playing.end_seconds;
}

export function secondsAtFraction(fraction: number, window: TimelineWindow): number {
  const span = Math.max(0, window.end - window.start);
  return window.start + clamp(fraction, 0, 1) * span;
}
