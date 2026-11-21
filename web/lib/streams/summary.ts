import type { StreamPlay } from '../api/streams.ts';
import { formatStreamClock } from './plan.ts';
import { isVoiceOnly } from './plays.ts';

export function summaryDefaultIds(plays: readonly StreamPlay[]): string[] {
  return plays.filter((play) => !isVoiceOnly(play)).map((play) => play.id);
}

function countWord({ count, one, many }: { count: number; one: string; many: string }): string {
  return `${count} ${count === 1 ? one : many}`;
}

export function summaryChoiceText(selected: readonly StreamPlay[]): string {
  if (selected.length === 0) return 'Elige al menos una jugada.';
  const voice = selected.filter(isVoiceOnly).length;
  const kills = selected.length - voice;
  const parts = [
    kills > 0 ? countWord({ count: kills, one: 'jugada con kill', many: 'jugadas con kill' }) : null,
    voice > 0 ? countWord({ count: voice, one: 'pico de voz', many: 'picos de voz' }) : null,
  ].filter((part) => part !== null);
  const seconds = selected.reduce((total, play) => total + play.end_seconds - play.start_seconds, 0);
  return `Un vídeo 16:9 con ${parts.join(' y ')}, una tras otra, unos ${formatStreamClock(seconds)}.`;
}

export type SummaryBulkChoice = { label: string; next: ReadonlySet<string> };

export function summaryBulkChoice(selected: ReadonlySet<string>, plays: readonly StreamPlay[]): SummaryBulkChoice {
  const kills = summaryDefaultIds(plays);
  if (kills.length > 0 && kills.every((id) => selected.has(id))) return { label: 'Quitar todas', next: new Set() };
  return {
    label: kills.length === 1 ? 'Elegir la jugada con kill' : `Elegir las ${kills.length} con kill`,
    next: new Set([...selected, ...kills]),
  };
}
