'use client';

import { useCallback, useState, type ReactNode } from 'react';
import type { SpatialScope } from '@/lib/api/library';
import type { SpatialResponse } from '@/lib/api/spatial';
import type { TacticalSide } from '@/lib/api/tactical';
import { prettyMapName } from '@/lib/format';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { HeatmapView } from './heatmap-view';
import { OverlayView } from './overlay-view';
import { Segmented } from './spatial-controls';

export type SpatialView = 'overlay' | 'heatmap';

const VIEWS: readonly { value: SpatialView; label: string }[] = [
  { value: 'overlay', label: 'Superponer rondas' },
  { value: 'heatmap', label: 'Mapas de calor' },
];

/**
 * The multi-round replay of a scope: its rounds overlaid on one clock, or its
 * heatmaps. A team scope reads one map at a time; the map list comes back with
 * the first answer, most played first.
 */
export function SpatialWorkspace({
  scope,
  side = '',
  view: initialView = 'overlay',
  hideViewSwitch = false,
}: {
  scope: SpatialScope;
  side?: TacticalSide | '';
  view?: SpatialView;
  hideViewSwitch?: boolean;
}): ReactNode {
  const [chosen, setView] = useState<SpatialView>(initialView);
  // With the switch hidden the owner picks the view, so the prop wins.
  const view = hideViewSwitch ? initialView : chosen;
  const [map, setMap] = useState<string | undefined>(undefined);
  const [maps, setMaps] = useState<{ map: string; matches: number }[]>([]);
  const [shownMap, setShownMap] = useState('');

  const onLoaded = useCallback((response: SpatialResponse) => {
    setMaps(response.maps);
    setShownMap(response.map);
  }, []);

  const scoped: SpatialScope = scope.kind === 'team' ? { ...scope, query: { ...scope.query, map: map ?? scope.query.map } } : scope;

  return (
    <div className="flex flex-col gap-4">
      {hideViewSwitch && (scope.kind !== 'team' || maps.length < 2) ? null : (
        <div className="flex flex-wrap items-center gap-2">
          {hideViewSwitch ? null : <Segmented label="Vista" value={view} options={VIEWS} onChange={setView} />}
          {scope.kind === 'team' && maps.length > 0 ? (
            <Select value={shownMap} onValueChange={setMap}>
              <SelectTrigger aria-label="Mapa" className="h-9 min-w-40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {maps.map((entry) => (
                  <SelectItem key={entry.map} value={entry.map}>
                    {prettyMapName(entry.map)} ({entry.matches})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : null}
        </div>
      )}
      {view === 'overlay' ? (
        <OverlayView scope={scoped} onLoaded={onLoaded} />
      ) : (
        <HeatmapView scope={scoped} side={side} onLoaded={onLoaded} />
      )}
    </div>
  );
}
