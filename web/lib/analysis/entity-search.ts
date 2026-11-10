function fold(text: string): string {
  return text.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase().trim();
}

/** The options whose label holds the query, ignoring case and accents; an empty query keeps all. */
export function searchOptions<T extends { readonly label: string }>(options: readonly T[], query: string): T[] {
  const needle = fold(query);
  return needle === '' ? [...options] : options.filter((option) => fold(option.label).includes(needle));
}
