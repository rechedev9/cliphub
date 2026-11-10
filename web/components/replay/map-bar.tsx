'use client';

import type { ReactNode, RefObject } from 'react';
import { Bomb, Eye, Flag, Flame, Footprints, Settings2, Tag, X, Zap } from 'lucide-react';
import { LAYER_LABELS, TRAIL_LENGTHS } from '@/lib/replay/layers';
import type { LayerToggle, ReplayLayers, TrailSeconds } from '@/lib/replay/layers';
import { MAP_ARTS, MAP_ART_LABELS, SEPARATE_FLOORS_HINT, SEPARATE_FLOORS_LABEL, floorsSeparated } from '@/lib/replay/map-settings';
import type { MapSettings } from '@/lib/replay/map-settings';
import { FOCUS_RING } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { cn } from '@/lib/utils';
import { ReplayChip } from './replay-chip';

const LAYER_ICONS: Record<LayerToggle, ReactNode> = {
  utility: <Flame aria-hidden />,
  bomb: <Bomb aria-hidden />,
  trails: <Footprints aria-hidden />,
  names: <Tag aria-hidden />,
  sight: <Eye aria-hidden />,
  hits: <Zap aria-hidden />,
  moments: <Flag aria-hidden />,
};

const LAYER_ORDER: readonly LayerToggle[] = ['utility', 'bomb', 'trails', 'hits', 'sight', 'moments', 'names'];

const LAYER_HINTS: Partial<Record<LayerToggle, string>> = {
  sight: 'Une a los rivales que se ven, con las paredes y los humos reales del mapa, y recorta los conos de visión en las paredes',
  hits: 'Una línea del atacante a la víctima en cada impacto y cada flash a un rival',
  moments: 'Primera baja, intercambios, plant, desactivaciones y clutch en la línea de tiempo',
};

/** The chips name themselves where the bar has room; in a narrow one the icon and its tooltip do. */
const WIDE_LABEL = 'hidden @5xl:inline';

/**
 * The bar over the map: how the map is drawn, its layers as chips, the player
 * it follows and the button that opens Ajustes del mapa. Nothing of this floats
 * over the map itself.
 */
export function MapBar({
  settings,
  onSettings,
  twoFloors,
  layers,
  onToggleLayer,
  onTrailSeconds,
  follow,
  preferences,
}: {
  settings: MapSettings;
  onSettings: (settings: MapSettings) => void;
  /** The map has a lower floor (Nuke, Vertigo), so it can split the floors. */
  twoFloors: boolean;
  layers: ReplayLayers;
  onToggleLayer: (key: LayerToggle) => void;
  onTrailSeconds: (seconds: TrailSeconds) => void;
  /** The player the map keeps centred. */
  follow: { name: string; onStop: () => void } | null;
  /** Ajustes del mapa, a sheet beside the map. */
  preferences: { open: boolean; onOpen: () => void; triggerRef: RefObject<HTMLButtonElement | null> };
}): ReactNode {
  return (
    <div className="@container flex shrink-0 flex-wrap items-center gap-x-2 gap-y-1.5 border-b border-border-subtle bg-surface-1 px-2 py-1.5">
      <Segmented
        label="Dibujo del mapa"
        value={settings.art}
        options={MAP_ARTS.map((art) => ({ value: art, label: MAP_ART_LABELS[art] }))}
        onChange={(art) => onSettings({ ...settings, art })}
      />
      {twoFloors ? (
        <label title={SEPARATE_FLOORS_HINT} className="flex h-7 cursor-pointer items-center gap-1.5 rounded-sm border border-border-strong bg-surface-1 px-2 text-meta text-fg-1 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring">
          <input
            type="checkbox"
            checked={floorsSeparated(settings)}
            onChange={(event) => onSettings({ ...settings, floors: event.target.checked ? 'both' : 'one' })}
            className="size-3.5 cursor-pointer accent-primary"
          />
          {SEPARATE_FLOORS_LABEL}
        </label>
      ) : null}
      <div className="flex flex-wrap items-center gap-1" role="group" aria-label="Capas del mapa">
        {LAYER_ORDER.map((key) => (
          <span key={key} className="flex items-center gap-1">
            <ReplayChip pressed={layers[key]} onClick={() => onToggleLayer(key)} title={LAYER_HINTS[key] === undefined ? LAYER_LABELS[key] : `${LAYER_LABELS[key]}. ${LAYER_HINTS[key]}`} aria-label={LAYER_LABELS[key]}>
              {LAYER_ICONS[key]}
              <span className={WIDE_LABEL}>{LAYER_LABELS[key]}</span>
            </ReplayChip>
            {key === 'trails' && layers.trails ? (
              <span className="flex items-center gap-0.5" role="group" aria-label="Largo de los rastros">
                {TRAIL_LENGTHS.map((seconds) => (
                  <ReplayChip key={seconds} pressed={layers.trailSeconds === seconds} onClick={() => onTrailSeconds(seconds)} className="px-1.5 tabular-nums">
                    {seconds} s
                  </ReplayChip>
                ))}
              </span>
            ) : null}
          </span>
        ))}
      </div>
      {follow === null ? null : (
        <button
          type="button"
          onClick={follow.onStop}
          className={cn('flex h-7 items-center gap-1.5 rounded-sm border border-border-strong bg-surface-1 px-2 text-meta text-fg-1 outline-none hover:bg-surface-3', FOCUS_RING)}
        >
          Siguiendo a <span className="max-w-32 truncate font-semibold">{follow.name}</span>
          <X className="size-3.5 text-fg-3" aria-label="Dejar de seguir" />
        </button>
      )}
      <ReplayChip
        ref={preferences.triggerRef}
        pressed={preferences.open}
        aria-haspopup="dialog"
        aria-expanded={preferences.open}
        onClick={preferences.onOpen}
        title="Ajustes del mapa: interfaz, jugador, rendimiento y atajos de teclado"
        aria-label="Ajustes del mapa"
        className="ml-auto"
      >
        <Settings2 aria-hidden />
        <span className={WIDE_LABEL}>Ajustes del mapa</span>
      </ReplayChip>
    </div>
  );
}
