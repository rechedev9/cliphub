export function certifiedRoundId(candidates: readonly string[], current: string): string | undefined {
  if (current !== '' && candidates.includes(current)) return current;
  return candidates[0];
}

export function isPrepareAbort(failure: unknown): boolean {
  return failure instanceof DOMException && failure.name === 'AbortError';
}
