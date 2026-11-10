import { sidesSwapped } from '../tactical-filter.ts';

/** True when the teams change sides between two rounds: regulation's half break and every overtime half, not an overtime's first half. */
export function isHalfBreak(info: { half: number } | null, previous: { half: number } | null): boolean {
  return info !== null && previous !== null && sidesSwapped(info.half) !== sidesSwapped(previous.half);
}
