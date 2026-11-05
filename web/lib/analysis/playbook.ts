import type { PlaybookCategoryName, PlaybookRound, PlaybookThresholds } from '../api/playbook.ts';
import type { ScoutEvidence } from '../api/scout.ts';
import type { TacticalSide, TacticalSite } from '../api/tactical.ts';
import type { ReplayRef } from './replay-link.ts';

/** Spanish names of the playbook categories; `default` and `save` read the same on both sides. */
const CATEGORY_LABELS: Record<PlaybookCategoryName, string> = {
  save: 'Save',
  fake: 'Fake',
  rush: 'Rush',
  execute: 'Ejecución',
  map_control: 'Control de mapa',
  stack: 'Stack',
  aggression: 'Agresión',
  retake: 'Retoma',
  default: 'Default',
};

export function playbookCategoryLabel(category: PlaybookCategoryName): string {
  return CATEGORY_LABELS[category];
}

/** The rule that put a round in the category, with the thresholds the server applied. */
export function playbookCategoryRule(category: PlaybookCategoryName, side: TacticalSide, rules: PlaybookThresholds): string {
  switch (category) {
    case 'save':
      return side === 'T' ? 'Guardan armas, o van de eco sin plantar ni pisar un sitio.' : 'Guardan armas sin defender.';
    case 'fake':
      return `Se dejan ver en un sitio, o tiran ${rules.fake_utility} o más granadas allí, y juegan el otro.`;
    case 'rush':
      return `Llegan a un sitio antes de ${rules.rush_seconds} s, o entran rápido con todo el grupo.`;
    case 'execute':
      return 'Entran a un sitio con granadas, juntos o por dos lados.';
    case 'map_control':
      return `Sujetan una zona fuera de los sitios con ${rules.map_control_players} o más durante ${rules.map_control_seconds} s antes de ${rules.map_control_window_seconds} s.`;
    case 'stack':
      return 'Cargan un sitio con más jugadores de lo normal.';
    case 'aggression':
      return 'Salen a buscar el duelo fuera de su sitio.';
    case 'retake':
      return 'Ceden el sitio y lo retoman tras la planta.';
    case 'default':
      return side === 'T' ? 'Ninguna de las anteriores: juego lento o ronda sin forma clara.' : 'Defienden en su posición habitual.';
    default:
      return '';
  }
}

/** The narrowing a person builds from the zone chips: every zone held and, optionally, where the round ended. */
export type ZoneFilter = { readonly zones: readonly string[]; readonly site: TacticalSite | '' };

export const EMPTY_ZONE_FILTER: ZoneFilter = { zones: [], site: '' };

export function toggleZone(filter: ZoneFilter, zone: string): ZoneFilter {
  const zones = filter.zones.includes(zone) ? filter.zones.filter((entry) => entry !== zone) : [...filter.zones, zone];
  return { ...filter, zones };
}

/** The rounds that held every zone of the filter and ended on its site. */
export function filterRoundsByZones(rounds: readonly PlaybookRound[], filter: ZoneFilter): PlaybookRound[] {
  return rounds.filter((round) => {
    if (filter.site !== '' && round.site !== filter.site) return false;
    return filter.zones.every((zone) => round.zones.some((hold) => hold.zone === zone));
  });
}

/** How many of these rounds held each zone, for the counts on the chips. */
export function zoneRoundCounts(rounds: readonly PlaybookRound[]): Map<string, number> {
  const counts = new Map<string, number>();
  for (const round of rounds) {
    for (const zone of new Set(round.zones.map((hold) => hold.zone))) counts.set(zone, (counts.get(zone) ?? 0) + 1);
  }
  return counts;
}

function joinWords(words: readonly string[]): string {
  if (words.length <= 1) return words.join('');
  return `${words.slice(0, -1).join(', ')} y ${words[words.length - 1] ?? ''}`;
}

/** "Controla Mid y Banana y acaba en B", the filter read as a sentence; empty for no filter. */
export function zoneFilterSentence(labels: readonly string[], siteName: string): string {
  const parts: string[] = [];
  if (labels.length > 0) parts.push(`controla ${joinWords(labels)}`);
  if (siteName !== '') parts.push(`acaba en ${siteName}`);
  const text = parts.join(' y ');
  return text === '' ? '' : `${text.charAt(0).toUpperCase()}${text.slice(1)}`;
}

/** Playbook rows as replay refs, opening each round at freeze end. */
export function roundRefs(rounds: readonly Pick<PlaybookRound, 'match_id' | 'round' | 'tick'>[]): ReplayRef[] {
  return rounds.map((round) => ({ matchId: round.match_id, round: round.round, tick: round.tick }));
}

export function evidenceRefs(refs: readonly ScoutEvidence[]): ReplayRef[] {
  return refs.map((ref) => ({ matchId: ref.match_id, round: ref.round, tick: ref.tick }));
}

/** Won over played, as a whole percent; null for no rounds. */
export function winPct(rounds: readonly Pick<PlaybookRound, 'won'>[]): number | null {
  if (rounds.length === 0) return null;
  return Math.round((rounds.filter((round) => round.won).length / rounds.length) * 100);
}

export type TempoBucket<T> = { readonly fromSeconds: number; readonly items: readonly T[] };

/**
 * Rounds bucketed by their tempo, every bucket from zero to the last one with
 * a round, so gaps show as empty bars. Rounds with no tempo are left out.
 */
export function tempoBuckets<T extends { tempo_seconds: number | null }>(rounds: readonly T[], bucketSeconds: number): TempoBucket<T>[] {
  if (!(bucketSeconds > 0)) return [];
  const byIndex = new Map<number, T[]>();
  let last = -1;
  for (const round of rounds) {
    if (round.tempo_seconds === null || !(round.tempo_seconds >= 0)) continue;
    const index = Math.floor(round.tempo_seconds / bucketSeconds);
    byIndex.set(index, [...(byIndex.get(index) ?? []), round]);
    last = Math.max(last, index);
  }
  const out: TempoBucket<T>[] = [];
  for (let index = 0; index <= last; index += 1) out.push({ fromSeconds: index * bucketSeconds, items: byIndex.get(index) ?? [] });
  return out;
}

/**
 * Each value as a percent of the largest, for bar lengths. A value above zero
 * never drops under `minPct`, so a single round still shows a bar.
 */
export function scaleBars(values: readonly number[], minPct = 4): number[] {
  const peak = values.reduce((max, value) => Math.max(max, value), 0);
  if (!(peak > 0)) return values.map(() => 0);
  return values.map((value) => (value > 0 ? Math.max(minPct, Math.round((value / peak) * 1000) / 10) : 0));
}

/** "0-5 s", the span of a bucket. */
export function bucketLabel(fromSeconds: number, bucketSeconds: number): string {
  return `${fromSeconds}-${fromSeconds + bucketSeconds} s`;
}
