import { SCOPE_TEXTS_MAX } from './scope-replay-filters.ts';

/** The demo tags and series events of the library, each once whatever its case, in Spanish order. */
export type LibraryTexts = { readonly tags: readonly string[]; readonly events: readonly string[] };

function distinct(values: readonly string[]): string[] {
  const seen = new Map<string, string>();
  for (const raw of values) {
    const value = raw.trim();
    const key = value.toLowerCase();
    if (value !== '' && !seen.has(key)) seen.set(key, value);
  }
  return [...seen.values()].sort((a, b) => a.localeCompare(b, 'es'));
}

export function libraryTexts(input: { demos: readonly { tags: readonly string[] }[]; series: readonly { event: string }[] }): LibraryTexts {
  return { tags: distinct(input.demos.flatMap((demo) => demo.tags)), events: distinct(input.series.map((series) => series.event)) };
}

/** The list with `value` added or taken out; a full list takes no more, as the server would refuse it. */
export function toggledText(list: readonly string[], value: string): string[] {
  if (list.includes(value)) return list.filter((entry) => entry !== value);
  return list.length >= SCOPE_TEXTS_MAX ? [...list] : [...list, value];
}
