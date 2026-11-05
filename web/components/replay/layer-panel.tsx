'use client';

import type { ReactNode } from 'react';
import { LAYER_LABELS } from '@/lib/replay/layers';
import type { LayerToggle, ReplayLayers } from '@/lib/replay/layers';
import { Button } from '@/components/ui/button';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

const TOGGLES: readonly LayerToggle[] = ['utility', 'bomb', 'trails', 'names'];

/** What the map draws. Players are switched one by one from their cards. */
export function LayerPanel({
  layers,
  onToggle,
  onShowAll,
}: {
  layers: ReplayLayers;
  onToggle: (key: LayerToggle) => void;
  onShowAll: () => void;
}): ReactNode {
  const on = TOGGLES.filter((key) => layers[key]);
  return (
    <div className="flex flex-col gap-2">
      <ToggleGroup
        type="multiple"
        variant="filter"
        spacing={2}
        value={[...on]}
        onValueChange={(next: string[]) => {
          for (const key of TOGGLES) if (next.includes(key) !== layers[key]) onToggle(key);
        }}
        aria-label="Capas del mapa"
        className="w-full flex-wrap"
      >
        {TOGGLES.map((key) => (
          <ToggleGroupItem key={key} value={key}>
            {LAYER_LABELS[key]}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      {layers.hiddenSlots.length === 0 ? null : (
        <p className="flex items-center justify-between gap-2 text-meta text-fg-2">
          {layers.hiddenSlots.length} {layers.hiddenSlots.length === 1 ? 'jugador oculto' : 'jugadores ocultos'}
          <Button type="button" size="xs" variant="ghost" onClick={onShowAll}>
            Mostrar todos
          </Button>
        </p>
      )}
    </div>
  );
}
