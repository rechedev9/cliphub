'use client';

import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

/** The library's aspect-ratio views: everything, or one render format. */
export type VideoFormatFilter = 'all' | 'short-9x16' | 'landscape-16x9' | 'full-demo';

/** No magic strings at the call site: the value round-trips through this map. */
const FORMAT_FILTERS = [
  { value: 'all', label: 'Todos', description: 'Todos los formatos' },
  { value: 'short-9x16', label: '9:16', description: 'Formato vertical 9:16' },
  { value: 'landscape-16x9', label: '16:9', description: 'Formato horizontal 16:9' },
  { value: 'full-demo', label: 'Partidas completas', description: 'Partidas completas 16:9 con resumen' },
] as const satisfies readonly { value: VideoFormatFilter; label: string; description: string }[];

function isFormatFilter(value: string): value is VideoFormatFilter {
  return FORMAT_FILTERS.some((entry) => entry.value === value);
}

export type VideoFiltersProps = {
  filter: VideoFormatFilter;
  onFilterChange: (filter: VideoFormatFilter) => void;
};

/**
 * Aspect-ratio filter chips for the Library grid. Filters over
 * `editConfig.format`, a field every reel already carries — no new data, just a
 * client-side view over it.
 *
 * `variant="filter"` at the group level, with the default `spacing={0}`, so the
 * three chips render as one joined segmented plate. The old call site pasted
 * `STUDIO_FILTER_CHIP_CLASS` onto separated items while `ToggleGroupItem` was
 * still applying the *joined* radii, which left chip 1 rounded on the left, chip
 * 2 fully square and chip 3 rounded on the right, with 8px of air between them.
 *
 * Radix hands back a bare `string`; `isFormatFilter` narrows it instead of
 * casting, which also drops the empty-string deselect without a second guard.
 */
export function VideoFilters({ filter, onFilterChange }: VideoFiltersProps) {
  return (
    <ToggleGroup
      type="single"
      variant="filter"
      value={filter}
      onValueChange={(value) => {
        if (isFormatFilter(value)) onFilterChange(value);
      }}
      className="w-fit max-w-full"
      aria-label="Filtrar reels por formato"
    >
      {FORMAT_FILTERS.map((entry) => (
        <ToggleGroupItem key={entry.value} value={entry.value} aria-label={entry.description}>
          {entry.label}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  );
}
