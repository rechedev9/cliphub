'use client';

import type { ReactNode } from 'react';
import { NO_MATCHES_TEXT, mapCountText, type MapCounts } from '@/lib/analysis/map-counts';
import { prettyMapName } from '@/lib/format';
import { MapThumb } from '@/components/analysis/map-thumb';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

/** Radix Select reserves the empty string, so "every map" travels as this. */
const ALL = '__all__';

export type ScopeMapSelectProps = {
  value: string;
  maps: readonly string[];
  /** The view reads one map at a time, so the empty choice asks for one instead of offering all. */
  required: boolean;
  /** Matches per map in the scope; null leaves the maps uncounted and all choosable. */
  counts: MapCounts | null;
  onChange: (map: string) => void;
};

function MapName({ map }: { map: string }): ReactNode {
  return (
    <span className="flex items-center gap-2">
      <MapThumb map={map} className="h-4 w-6" crop={1.6} />
      {prettyMapName(map)}
    </span>
  );
}

/** The map control of the scope bar: each map says how many matches it has in the scope and a map with none cannot be chosen. */
export function ScopeMapSelect({ value, maps, required, counts, onChange }: ScopeMapSelectProps): ReactNode {
  const allLabel = required ? 'Elige un mapa' : 'Todos los mapas';
  // A map reached through the URL, a chip or a changed scope can still have none: say so on the trigger.
  const emptyChoice = value !== '' && counts?.[value] === 0;
  return (
    <Select value={value === '' ? ALL : value} onValueChange={(next) => onChange(next === ALL ? '' : next)}>
      <SelectTrigger
        aria-label="Mapa"
        title={emptyChoice ? NO_MATCHES_TEXT : undefined}
        className={cn('h-8 gap-1.5 px-2.5 text-body-sm', value !== '' && 'border-fg-3 text-fg-1')}
      >
        <SelectValue>
          <span className="flex items-center gap-2">
            {value === '' ? allLabel : <MapName map={value} />}
            {emptyChoice ? <span className="text-meta text-fg-3 tabular-nums">{mapCountText(0)}</span> : null}
          </span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>{allLabel}</SelectItem>
        {maps.map((map) => {
          const count = counts?.[map];
          const empty = count === 0;
          return (
            <SelectItem
              key={map}
              value={map}
              disabled={empty}
              title={empty ? NO_MATCHES_TEXT : undefined}
              className="data-[disabled]:pointer-events-auto data-[disabled]:cursor-not-allowed data-[disabled]:text-fg-3"
            >
              <span className="flex w-full items-center justify-between gap-4">
                <MapName map={map} />
                {count === undefined ? null : <span className="text-meta text-fg-3 tabular-nums">{mapCountText(count)}</span>}
              </span>
            </SelectItem>
          );
        })}
      </SelectContent>
    </Select>
  );
}
