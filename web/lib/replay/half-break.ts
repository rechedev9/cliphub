/** True when a round opens a new half (or overtime period) after the previous round of the same match. */
export function isHalfBreak(info: { half: number } | null, previous: { half: number } | null): boolean {
  return info !== null && previous !== null && info.half !== previous.half;
}
