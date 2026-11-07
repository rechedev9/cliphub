import type { CallSide, LibraryCall } from '../api/calls.ts';

export const CALL_SIDE_LABEL: Record<CallSide, string> = { T: 'T', CT: 'CT', both: 'Ambos' };

/** What each filter of the Llamadas list reads: a side also keeps the calls of both sides, as the server does. */
export const CALL_FILTER_FIELDS = {
  side: (call: LibraryCall): readonly string[] => (call.side === 'both' ? ['T', 'CT', 'both'] : [call.side]),
  tag: (call: LibraryCall): readonly string[] => call.tags,
};

export function callSearchText(call: LibraryCall): string {
  return [call.name, call.meaning, call.tags.join(' ')].join(' ');
}

/** Every tag of these entries once, by name, as filter options. */
export function tagOptions(rows: readonly { tags: readonly string[] }[]): { value: string; label: string }[] {
  return [...new Set(rows.flatMap((row) => row.tags))]
    .sort((a, b) => a.localeCompare(b, 'es'))
    .map((tag) => ({ value: tag, label: tag }));
}
