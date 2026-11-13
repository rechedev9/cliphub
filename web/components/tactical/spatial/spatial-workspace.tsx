'use client';

import { useCallback, useState, type ReactNode } from 'react';
import type { SpatialScope } from '@/lib/api/library';
import type { SpatialResponse } from '@/lib/api/spatial';
import type { TacticalSide } from '@/lib/api/tactical';
import type { MapCounts } from '@/lib/analysis/map-counts';
import { ScopeMapSelect } from '@/components/analysis/scope-map-select';
import type { TramoControl } from '@/components/replay/tramo-bar';
import { HeatmapView } from './heatmap-view';
import type { HeatControl } from './heatmap-view';
import { OverlayView } from './overlay-view';
import { Segmented } from '@/components/ui/segmented';

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
  mapCounts,
  onMapChange,
  onLoaded: onAnswer,
  tramo,
  heat,
  inMatch = false,
}: {
  scope: SpatialScope;
  side?: TacticalSide | '';
  view?: SpatialView;
  hideViewSwitch?: boolean;
  /** Matches per map in the owner's scope; a map with 0 cannot be chosen. */
  mapCounts?: MapCounts | null;
  /** When set, the map select writes the owner's scope instead of local state. */
  onMapChange?: (map: string) => void;
  /** Every answer, so the owner can show its sample size. */
  onLoaded?: (response: SpatialResponse) => void;
  /** The owner's tramo, shared by both views. */
  tramo?: TramoControl;
  /** Inside a match, the map takes the Replay's viewer: its box, its size and its side column. */
  inMatch?: boolean;
  /** What the heatmap counts, when the owner keeps it in its URL. */
  heat?: HeatControl;
}): ReactNode {
  const [chosen, setView] = useState<SpatialView>(initialView);
  // With the switch hidden the owner picks the view, so the prop wins.
  const view = hideViewSwitch ? initialView : chosen;
  const [map, setMap] = useState<string | undefined>(undefined);
  const [maps, setMaps] = useState<{ map: string; matches: number }[]>([]);
  const [shownMap, setShownMap] = useState('');

  const onLoaded = useCallback(
    (response: SpatialResponse) => {
      setMaps(response.maps);
      setShownMap(response.map);
      onAnswer?.(response);
    },
    [onAnswer],
  );

  const mapNames = [...maps.map((entry) => entry.map), ...Object.keys(mapCounts ?? {}).filter((name) => !maps.some((entry) => entry.map === name))];

  const scoped: SpatialScope =
    scope.kind === 'team' && onMapChange === undefined ? { ...scope, query: { ...scope.query, map: map ?? scope.query.map } } : scope;

  return (
    <div className="flex flex-col gap-4">
      {hideViewSwitch && (scope.kind !== 'team' || mapNames.length < 2) ? null : (
        <div className="flex flex-wrap items-center gap-2">
          {hideViewSwitch ? null : <Segmented label="Vista" value={view} options={VIEWS} onChange={setView} />}
          {scope.kind === 'team' && mapNames.length > 0 ? (
            <ScopeMapSelect
              value={shownMap}
              maps={mapNames}
              required
              counts={mapCounts ?? null}
              onChange={onMapChange ?? setMap}
            />
          ) : null}
        </div>
      )}
      {view === 'overlay' ? (
        <OverlayView scope={scoped} onLoaded={onLoaded} tramo={tramo} inMatch={inMatch} />
      ) : (
        <HeatmapView scope={scoped} side={side} onLoaded={onLoaded} tramo={tramo} heat={heat} inMatch={inMatch} />
      )}
    </div>
  );
}
