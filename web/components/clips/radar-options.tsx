'use client';

import { Radar } from 'lucide-react';
import { useId, type ReactNode } from 'react';
import type { EditConfig } from '@/lib/api/types';
import {
  RADAR_DEFAULT_OPACITY,
  RADAR_MIN_OPACITY,
  RADAR_POSITIONS,
  RADAR_POSITION_LABEL,
  RADAR_SIZES,
  RADAR_SIZE_LABEL,
  isRadarPosition,
  isRadarSize,
  radarAutoPosition,
} from '@/lib/radar-overlay';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

const AUTO = 'auto';

export type RadarOptionsProps = {
  value: EditConfig;
  onChange: (next: EditConfig) => void;
  disabled: boolean;
  /** False when the chosen style records CS2's own HUD, whose radar is already in the video. */
  allowed: boolean;
};

/** ClipHub's radar of a Short: the switch, its corner, its size and its opacity. */
export function RadarOptions({ value, onChange, disabled, allowed }: RadarOptionsProps): ReactNode {
  const id = useId();
  const on = allowed && value.radar === true;
  const opacityPercent = Math.round((value.radarOpacity ?? RADAR_DEFAULT_OPACITY) * 100);

  function toggle(items: string[]): void {
    const next: EditConfig = { ...value, radar: items.includes('radar') };
    if (!next.radar) {
      delete next.radarPosition;
      delete next.radarSize;
      delete next.radarOpacity;
    }
    onChange(next);
  }

  function choosePosition(picked: string): void {
    const next: EditConfig = { ...value };
    if (isRadarPosition(picked)) next.radarPosition = picked;
    else delete next.radarPosition;
    onChange(next);
  }

  return (
    <>
      <ToggleGroup
        type="multiple"
        value={on ? ['radar'] : []}
        onValueChange={toggle}
        disabled={disabled || !allowed}
        variant="outline"
        className="flex-wrap"
      >
        <ToggleGroupItem value="radar" aria-label="Radar de ClipHub">
          <Radar className="size-4" />
          Radar de ClipHub
        </ToggleGroupItem>
      </ToggleGroup>
      {allowed ? null : (
        <p className="text-body-sm text-fg-3">
          Este estilo ya graba el radar de CS2 en el vídeo. Elige otro estilo para usar el radar de ClipHub.
        </p>
      )}
      {on ? (
        <div className="grid min-w-0 gap-3 sm:grid-cols-2">
          <div className="grid min-w-0 gap-2">
            <label htmlFor={`${id}-position`} className="text-body-sm text-fg-2">Posición</label>
            <Select value={value.radarPosition ?? AUTO} onValueChange={choosePosition} disabled={disabled}>
              <SelectTrigger id={`${id}-position`} className="h-10 w-full max-w-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={AUTO}>Automática · {RADAR_POSITION_LABEL[radarAutoPosition(value.format)].toLowerCase()}</SelectItem>
                {RADAR_POSITIONS.map((position) => (
                  <SelectItem key={position} value={position}>{RADAR_POSITION_LABEL[position]}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid min-w-0 gap-2">
            <span className="text-body-sm text-fg-2">Tamaño</span>
            <ToggleGroup
              type="single"
              value={value.radarSize ?? 'medium'}
              onValueChange={(size) => { if (isRadarSize(size)) onChange({ ...value, radarSize: size }); }}
              disabled={disabled}
              variant="outline"
              className="flex-wrap justify-start"
            >
              {RADAR_SIZES.map((size) => (
                <ToggleGroupItem key={size} value={size} aria-label={RADAR_SIZE_LABEL[size]}>{RADAR_SIZE_LABEL[size]}</ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
          <div className="grid min-w-0 gap-2 sm:col-span-2">
            <label htmlFor={`${id}-opacity`} className="text-body-sm text-fg-2">
              Opacidad <span className="text-fg-1">· {opacityPercent} %</span>
            </label>
            <input
              id={`${id}-opacity`}
              type="range"
              min={Math.round(RADAR_MIN_OPACITY * 100)}
              max={100}
              step={5}
              value={opacityPercent}
              disabled={disabled}
              onChange={(event) => onChange({ ...value, radarOpacity: Number(event.target.value) / 100 })}
              className="h-1 w-full max-w-md cursor-pointer appearance-none rounded-full bg-border-strong accent-primary disabled:cursor-not-allowed disabled:opacity-50"
            />
          </div>
          <p className="text-body-sm text-fg-3 sm:col-span-2">
            Se lee de la demo: cada jugador vivo y hacia dónde mira, humos, molotovs, flashes, granadas, las muertes de la ronda y la bomba. Se oculta durante el título, la repetición desde la víctima y la tarjeta final, y se aparta de tu marca de agua y de los banners.
          </p>
        </div>
      ) : null}
    </>
  );
}
