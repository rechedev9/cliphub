'use client';

import { useState, type ReactNode } from 'react';
import { ArrowUpRight, Bomb, Brush, Eraser, Eye, Film, Flag, Flame, Footprints, Hand, Settings2, Tag, Timer, Trash2, Undo2, X, Zap } from 'lucide-react';
import { STROKE_COLORS } from '@/lib/api/spatial';
import type { HitLineKind } from '@/lib/replay/hit-lines';
import { LAYER_LABELS, TRAIL_LENGTHS } from '@/lib/replay/layers';
import type { LayerToggle, ReplayLayers, TrailSeconds } from '@/lib/replay/layers';
import {
  FRAME_RATES,
  MAP_ARTS,
  MAP_ART_LABELS,
  MAP_FLOORS,
  MAP_FLOORS_LABELS,
  MAP_SHOWS,
  MAP_SHOW_HINTS,
  MAP_SHOW_LABELS,
  MAP_SIZES,
  MAP_SIZE_LABELS,
  ZOOM_SPEED_LABELS,
  frameRateLabel,
} from '@/lib/replay/map-settings';
import type { MapSettings } from '@/lib/replay/map-settings';
import { HIT_TOKENS } from '@/components/tactical/radar-style';
import { STROKE_LABEL, STROKE_VAR, chooseDrawMode, drawMode } from '@/components/tactical/spatial/telestrator';
import type { DrawMode, TelestratorDraft } from '@/components/tactical/spatial/telestrator';
import { FOCUS_RING } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { cn } from '@/lib/utils';
import { ReplayChip } from './replay-chip';
import { ShortcutTable } from './shortcut-table';

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

const HIT_LEGEND: readonly [HitLineKind, string][] = [
  ['bullet', 'Bala'],
  ['he', 'HE'],
  ['fire', 'Fuego'],
  ['flash', 'Flash'],
];

/** The chips name themselves where the bar has room; in a narrow one the icon and its tooltip do. */
const WIDE_LABEL = 'hidden @5xl:inline';

const DRAW_MODES: readonly { mode: DrawMode; label: string; hint: string; icon: ReactNode }[] = [
  { mode: 'move', label: 'Mover', hint: 'Mover el mapa (H)', icon: <Hand aria-hidden /> },
  { mode: 'draw', label: 'Dibujar', hint: 'Dibujar (D)', icon: <Brush aria-hidden /> },
  { mode: 'arrow', label: 'Flecha', hint: 'Flecha (A)', icon: <ArrowUpRight aria-hidden /> },
  { mode: 'erase', label: 'Borrar trazo', hint: 'Borrar trazo: pulsa o arrastra sobre un trazo del dibujo para quitarlo (E)', icon: <Eraser aria-hidden /> },
];

function IconButton({ label, disabled = false, onClick, children }: { label: string; disabled?: boolean; onClick: () => void; children: ReactNode }): ReactNode {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className={cn('grid size-7 place-items-center rounded-sm text-fg-2 outline-none hover:bg-surface-3 hover:text-fg-1 disabled:pointer-events-none disabled:text-fg-4 [&_svg]:size-3.5', FOCUS_RING)}
    >
      {children}
    </button>
  );
}

/** Mover, Dibujar and Flecha, the stroke colours and undo: the one drawing bar of the Replay. */
function DrawTools({ draft }: { draft: TelestratorDraft }): ReactNode {
  const mode = drawMode(draft);
  return (
    <div role="toolbar" aria-label="Dibujo sobre el mapa" className="flex items-center gap-1">
      {DRAW_MODES.map((entry) => (
        <ReplayChip key={entry.mode} pressed={mode === entry.mode} onClick={() => chooseDrawMode(draft, entry.mode)} title={entry.hint} aria-label={entry.label}>
          {entry.icon}
          <span className={WIDE_LABEL}>{entry.label}</span>
        </ReplayChip>
      ))}
      <ReplayChip
        pressed={draft.tool.timed}
        onClick={() => draft.setTool({ ...draft.tool, timed: !draft.tool.timed })}
        title="Temporal: cada trazo se desvanece solo a los 4 s y no entra en la nota (T)"
        aria-label="Trazo temporal"
      >
        <Timer aria-hidden />
        <span className={WIDE_LABEL}>Temporal</span>
      </ReplayChip>
      <div className="flex items-center gap-1 px-1" role="group" aria-label="Color del trazo">
        {STROKE_COLORS.map((color) => (
          <button
            key={color}
            type="button"
            aria-label={STROKE_LABEL[color]}
            title={STROKE_LABEL[color]}
            aria-pressed={draft.tool.color === color}
            onClick={() => draft.setTool({ ...draft.tool, color })}
            className={cn('size-5 rounded-full border-2 outline-none', FOCUS_RING, draft.tool.color === color ? 'border-fg-1' : 'border-transparent')}
            style={{ backgroundColor: STROKE_VAR[color] }}
          />
        ))}
      </div>
      <IconButton label="Deshacer el último trazo (Z)" disabled={draft.strokes.length === 0} onClick={draft.undo}>
        <Undo2 aria-hidden />
      </IconButton>
      <IconButton label="Borrar todo el dibujo" disabled={draft.strokes.length === 0} onClick={draft.clear}>
        <Trash2 aria-hidden />
      </IconButton>
    </div>
  );
}

/**
 * The bar over the map: how the map is drawn, its layers as chips, the map's
 * settings and the drawing tools. Nothing of this floats over the map itself.
 */
export function MapBar({
  settings,
  onSettings,
  twoFloors,
  layers,
  onToggleLayer,
  onTrailSeconds,
  draft,
  gif,
  follow,
  zoomSlot,
}: {
  settings: MapSettings;
  onSettings: (settings: MapSettings) => void;
  /** The map has a lower floor (Nuke, Vertigo), so it can show both at once. */
  twoFloors: boolean;
  layers: ReplayLayers;
  onToggleLayer: (key: LayerToggle) => void;
  onTrailSeconds: (seconds: TrailSeconds) => void;
  /** The drawing of the note being made; null where notes cannot be saved. */
  draft: TelestratorDraft | null;
  /** The GIF row under the bar, opened with this chip or G. */
  gif: { open: boolean; onToggle: () => void };
  /** The player the map keeps centred. */
  follow: { name: string; onStop: () => void } | null;
  /** Where the map puts its zoom buttons. */
  zoomSlot: (element: HTMLDivElement | null) => void;
}): ReactNode {
  const [open, setOpen] = useState(false);
  return (
    <div className="@container flex shrink-0 flex-col gap-1.5 border-b border-border-subtle bg-surface-1 px-2 py-1.5">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1.5">
        <Segmented
          label="Dibujo del mapa"
          value={settings.art}
          options={MAP_ARTS.map((art) => ({ value: art, label: MAP_ART_LABELS[art] }))}
          onChange={(art) => onSettings({ ...settings, art })}
        />
        {twoFloors ? (
          <Segmented
            label="Pisos del mapa"
            value={settings.floors}
            options={MAP_FLOORS.map((floors) => ({ value: floors, label: MAP_FLOORS_LABELS[floors] }))}
            onChange={(floors) => onSettings({ ...settings, floors })}
          />
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
          <ReplayChip pressed={open} aria-expanded={open} onClick={() => setOpen((value) => !value)} title="Ajustes del mapa" aria-label="Ajustes del mapa">
            <Settings2 aria-hidden />
            <span className={WIDE_LABEL}>Ajustes del mapa</span>
          </ReplayChip>
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
        <div className="ml-auto flex flex-wrap items-center gap-x-2 gap-y-1.5">
          {draft === null ? null : <DrawTools draft={draft} />}
          <ReplayChip pressed={gif.open} aria-expanded={gif.open} onClick={gif.onToggle} title="GIF del tramo (G)" aria-label="GIF del tramo">
            <Film aria-hidden />
            GIF
          </ReplayChip>
          <div ref={zoomSlot} />
        </div>
      </div>
      {open ? (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 border-t border-border-subtle pt-1.5">
          <div className="flex items-center gap-2 text-meta text-fg-3">
            Puntos
            <Segmented
              label="Tamaño de los puntos"
              value={settings.dots}
              options={MAP_SIZES.map((size) => ({ value: size, label: MAP_SIZE_LABELS[size] }))}
              onChange={(dots) => onSettings({ ...settings, dots })}
            />
          </div>
          <div className="flex items-center gap-2 text-meta text-fg-3">
            Nombres
            <Segmented
              label="Tamaño de los nombres"
              value={settings.names}
              options={MAP_SIZES.map((size) => ({ value: size, label: MAP_SIZE_LABELS[size] }))}
              onChange={(names) => onSettings({ ...settings, names })}
            />
          </div>
          <div className="flex items-center gap-2 text-meta text-fg-3">
            Zoom
            <Segmented
              label="Velocidad del zoom con la rueda"
              value={settings.zoom}
              options={MAP_SIZES.map((size) => ({ value: size, label: ZOOM_SPEED_LABELS[size] }))}
              onChange={(zoom) => onSettings({ ...settings, zoom })}
            />
          </div>
          <div className="flex items-center gap-2 text-meta text-fg-3" title="Menos imágenes por segundo descargan el PC mientras se reproduce">
            Imágenes por segundo
            <Segmented
              label="Límite de imágenes por segundo"
              value={String(settings.fps)}
              options={FRAME_RATES.map((rate) => ({ value: String(rate), label: frameRateLabel(rate) }))}
              onChange={(value) => onSettings({ ...settings, fps: FRAME_RATES.find((rate) => String(rate) === value) ?? 0 })}
            />
          </div>
          <div className="flex flex-wrap items-center gap-1" role="group" aria-label="Qué dibuja el mapa">
            {MAP_SHOWS.map((key) => (
              <ReplayChip
                key={key}
                pressed={settings.show[key]}
                onClick={() => onSettings({ ...settings, show: { ...settings.show, [key]: !settings.show[key] } })}
                title={`${MAP_SHOW_LABELS[key]}. ${MAP_SHOW_HINTS[key]}`}
              >
                {MAP_SHOW_LABELS[key]}
              </ReplayChip>
            ))}
          </div>
          <ul className="flex flex-wrap items-center gap-x-2.5 gap-y-1 text-meta text-fg-3" aria-label="Colores del daño">
            {HIT_LEGEND.map(([kind, label]) => (
              <li key={kind} className="flex items-center gap-1">
                <span className="h-0.5 w-3 rounded-full" style={{ backgroundColor: `var(${HIT_TOKENS[kind]})` }} aria-hidden />
                {label}
              </li>
            ))}
          </ul>
          <section aria-label="Atajos de teclado" className="flex basis-full flex-col gap-1.5 border-t border-border-subtle pt-1.5">
            <h3 className="text-meta font-semibold text-fg-2">Atajos de teclado</h3>
            <ShortcutTable drawing={draft !== null} className="@3xl:flex-row @3xl:*:flex-1" />
          </section>
        </div>
      ) : null}
    </div>
  );
}
