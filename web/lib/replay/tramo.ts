/**
 * The tramo: the span of the round the map counts, apart from the cursor that
 * plays. Seconds run from the end of freeze time, as the server reads them;
 * the user reads the CS2 round clock, which counts down from 1:55.
 */

/** A CS2 round lasts 1:55 until the bomb or the time decides it. */
export const ROUND_CLOCK_SECONDS = 115;
/** The shortest tramo a drag can leave. */
export const TRAMO_MIN_SECONDS = 5;

/** `fromPlant` starts the span at each round's plant and ignores `from`; `to` 0 runs to the end of the round. */
export type Tramo = { from: number; to: number; fromPlant?: boolean };

/** The whole round: nothing is cut. */
export const WHOLE_ROUND: Tramo = { from: 0, to: 0 };

/** The quick tramos, the same list in the Replay and in Equipos > Preparar. */
export const QUICK_TRAMOS = [
  { id: 'all', label: 'Toda la ronda', tramo: WHOLE_ROUND },
  { id: 'early', label: 'Hasta 1:00', tramo: { from: 0, to: 55 } },
  { id: 'mid', label: '1:00 a 0:40', tramo: { from: 55, to: 75 } },
  { id: 'plant', label: 'Desde el plant', tramo: { from: 0, to: 0, fromPlant: true } },
] as const satisfies readonly { id: string; label: string; tramo: Tramo }[];

export type QuickTramoId = (typeof QUICK_TRAMOS)[number]['id'];

/** The tramos of "Posiciones hasta la primera muerte": 15 s each from 1:55, then the rest of the round. */
export const FIRST_DEATH_TRAMOS = [
  { id: 'opening', label: '1:55-1:40', tramo: { from: 0, to: 15 } },
  { id: 'early', label: '1:40-1:25', tramo: { from: 15, to: 30 } },
  { id: 'mid', label: '1:25-1:10', tramo: { from: 30, to: 45 } },
  { id: 'late', label: '1:10-0:00', tramo: { from: 45, to: 0 } },
] as const satisfies readonly { id: string; label: string; tramo: Tramo }[];

export type FirstDeathTramoId = (typeof FIRST_DEATH_TRAMOS)[number]['id'];

/** Seconds since freeze end as the round clock shows them: 15 s is "1:40". */
export function roundClock(seconds: number): string {
  const left = Math.max(0, Math.round(ROUND_CLOCK_SECONDS - seconds));
  return `${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`;
}

export function isWholeRound(tramo: Tramo): boolean {
  return tramo.fromPlant !== true && tramo.from <= 0 && (tramo.to <= 0 || tramo.to >= ROUND_CLOCK_SECONDS);
}

/** Where the tramo ends on the clock; an open end is the end of the round. */
export function tramoEnd(tramo: Tramo): number {
  return tramo.to <= 0 ? ROUND_CLOCK_SECONDS : Math.min(tramo.to, ROUND_CLOCK_SECONDS);
}

/** "Tramo 1:40 a 1:10", always with the word, so it is never taken for the cursor. */
export function tramoLabel(tramo: Tramo): string {
  if (tramo.fromPlant === true) return 'Tramo desde el plant';
  if (isWholeRound(tramo)) return 'Tramo: toda la ronda';
  return `Tramo ${roundClock(tramo.from)} a ${roundClock(tramoEnd(tramo))}`;
}

export function quickTramoOf(tramo: Tramo): QuickTramoId | null {
  if (tramo.fromPlant === true) return 'plant';
  if (isWholeRound(tramo)) return 'all';
  return QUICK_TRAMOS.find((quick) => quick.tramo.from === tramo.from && quick.tramo.to === tramo.to && quick.id !== 'all' && quick.id !== 'plant')?.id ?? null;
}

/** Whether a moment of a round falls in the tramo; `plantSeconds` is null when the bomb was not planted. */
export function inTramo(tramo: Tramo, seconds: number, plantSeconds: number | null): boolean {
  if (tramo.fromPlant === true) return plantSeconds !== null && seconds >= plantSeconds;
  return seconds >= tramo.from && seconds <= tramoEnd(tramo);
}

/** One handle dragged to `seconds`: whole seconds, inside the round, never closer than the minimum to the other. */
export function moveTramoEdge(tramo: Tramo, edge: 'from' | 'to', seconds: number): Tramo {
  const value = Math.round(Math.min(ROUND_CLOCK_SECONDS, Math.max(0, seconds)));
  if (edge === 'from') return { from: Math.min(value, tramoEnd(tramo) - TRAMO_MIN_SECONDS), to: tramoEnd(tramo) };
  return { from: tramo.from, to: Math.max(value, tramo.from + TRAMO_MIN_SECONDS) };
}

/** Query key of the tramo: `15-45` in seconds, or `plant`. The whole round is left out. */
export const TRAMO_QUERY = 'tramo';

export function tramoFromQuery(query: Pick<URLSearchParams, 'get'>): Tramo {
  const raw = query.get(TRAMO_QUERY);
  if (raw === 'plant') return { from: 0, to: 0, fromPlant: true };
  const match = raw === null ? null : /^(\d{1,3})-(\d{1,3})$/.exec(raw);
  if (match === null) return WHOLE_ROUND;
  const from = Number(match[1]);
  const to = Number(match[2]);
  if (from >= to || to > ROUND_CLOCK_SECONDS) return WHOLE_ROUND;
  return { from, to };
}

/** The query with the tramo written over it, every other key kept. */
export function writeTramo(current: string, tramo: Tramo): string {
  const params = new URLSearchParams(current);
  if (tramo.fromPlant === true) params.set(TRAMO_QUERY, 'plant');
  else if (isWholeRound(tramo)) params.delete(TRAMO_QUERY);
  else params.set(TRAMO_QUERY, `${tramo.from}-${tramoEnd(tramo)}`);
  return params.toString();
}
