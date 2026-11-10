'use client';

import type { ReactNode } from 'react';
import { CircleDot } from 'lucide-react';
import { DOT_SIZES, DOT_SIZE_LABELS, MAP_SIZES, MAP_SIZE_LABELS, namesShown, toggleNames, type MapSettings } from '@/lib/replay/map-settings';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Segmented } from '@/components/ui/segmented';
import { Switch } from '@/components/ui/switch';

/**
 * Under the map, beside the zoom: the size of the player dots and their names, at hand while watching.
 * The same settings as Preferencias del replay, which keeps them for every match.
 */
export function PlayerMarksMenu({ settings, multi, onSettings }: { settings: MapSettings; multi: boolean; onSettings: (settings: MapSettings) => void }): ReactNode {
  const names = namesShown(settings, multi);
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button type="button" variant="ghost" size="xs" aria-label="Jugadores en el mapa: tamaño y nombres" title="Tamaño de los jugadores y nombres">
          <CircleDot aria-hidden />
          Jugadores
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" side="top" className="flex w-96 max-w-[92vw] flex-col gap-3 p-3">
        <div className="flex flex-col gap-1.5">
          <span className="text-meta font-semibold text-fg-1">Tamaño de los puntos</span>
          <Segmented
            label="Tamaño de los puntos"
            value={settings.dots}
            options={DOT_SIZES.map((size) => ({ value: size, label: DOT_SIZE_LABELS[size] }))}
            onChange={(dots) => onSettings({ ...settings, dots })}
          />
        </div>
        <label className="flex cursor-pointer items-center justify-between gap-2 text-body-sm text-fg-1">
          {multi ? 'Nombres en varias rondas' : 'Nombres en la ronda'}
          <Switch checked={names} onCheckedChange={() => onSettings(toggleNames(settings, multi))} />
        </label>
        <div className={names ? 'flex flex-col gap-1.5' : 'pointer-events-none flex flex-col gap-1.5 opacity-45'} aria-disabled={!names}>
          <span className="text-meta font-semibold text-fg-1">Tamaño de los nombres</span>
          <Segmented
            label="Tamaño de los nombres"
            value={settings.names}
            options={MAP_SIZES.map((size) => ({ value: size, label: MAP_SIZE_LABELS[size] }))}
            onChange={(next) => onSettings({ ...settings, names: next })}
          />
        </div>
      </PopoverContent>
    </Popover>
  );
}
