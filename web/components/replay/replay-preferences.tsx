'use client';

import type { ReactNode, RefObject } from 'react';
import type { HitLineKind } from '@/lib/replay/hit-lines';
import { LAYER_LABELS, TRAIL_LENGTHS } from '@/lib/replay/layers';
import type { LayerToggle, ReplayLayers, TrailSeconds } from '@/lib/replay/layers';
import {
  DOT_SIZES,
  DOT_SIZE_LABELS,
  FRAME_RATES,
  MAP_ARTS,
  MAP_ART_LABELS,
  MAP_SHOW_HINTS,
  MAP_SHOW_LABELS,
  MAP_SIZES,
  MAP_SIZE_LABELS,
  SEPARATE_FLOORS_HINT,
  SEPARATE_FLOORS_LABEL,
  ZOOM_SPEED_LABELS,
  floorsSeparated,
  frameRateLabel,
} from '@/lib/replay/map-settings';
import type { MapSettings, MapShow } from '@/lib/replay/map-settings';
import { HIT_TOKENS } from '@/components/tactical/radar-style';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Segmented } from '@/components/ui/segmented';
import { cn } from '@/lib/utils';
import { ShortcutTable } from './shortcut-table';

const HIT_LEGEND: readonly [HitLineKind, string][] = [
  ['bullet', 'Bala'],
  ['he', 'HE'],
  ['fire', 'Fuego'],
  ['flash', 'Flash'],
];

const LAYER_HINTS: Partial<Record<LayerToggle, string>> = {
  sight: 'Une a los rivales que se ven, con las paredes y los humos reales del mapa, y recorta los conos de visión en las paredes',
  hits: 'Una línea del atacante a la víctima en cada impacto y cada flash a un rival',
  moments: 'Primera baja, intercambios, plant, desactivaciones y clutch en la línea de tiempo',
};

/** The layers the map draws, in the order the dialog lists them; Nombres has its own rows under Jugador. */
const LAYER_ORDER: readonly Exclude<LayerToggle, 'names'>[] = ['utility', 'bomb', 'trails', 'hits', 'sight', 'moments'];

function Group({ title, children }: { title: string; children: ReactNode }): ReactNode {
  return (
    <section aria-label={title} className="flex flex-col gap-3 border-t border-border-subtle pt-4 first:border-t-0 first:pt-0">
      <h3 className="border-b border-border-subtle pb-1.5 text-body-sm font-bold text-fg-1">{title}</h3>
      <div className="grid gap-x-6 gap-y-4 sm:grid-cols-3">{children}</div>
    </section>
  );
}

function Setting({ label, hint, className, children }: { label: string; hint?: string; className?: string; children: ReactNode }): ReactNode {
  return (
    <div className={cn('flex flex-col gap-1.5', className)}>
      <span className="text-meta font-semibold text-fg-1">{label}</span>
      {children}
      {hint === undefined ? null : <span className="text-meta text-fg-3">{hint}</span>}
    </div>
  );
}

function Check({ label, hint, checked, onChange }: { label: string; hint?: string; checked: boolean; onChange: (checked: boolean) => void }): ReactNode {
  return (
    <label title={hint} className="flex cursor-pointer items-center gap-2 text-body-sm text-fg-2 has-[:focus-visible]:text-fg-1">
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} className="size-3.5 cursor-pointer accent-primary" />
      {label}
    </label>
  );
}

/**
 * Preferencias del replay: how the Replay draws the map and its players, which
 * layers it shows, how fast it plays, and every keyboard shortcut. They apply
 * at once and stay for every match.
 */
export function ReplayPreferences({
  open,
  onOpenChange,
  settings,
  onSettings,
  layers,
  onToggleLayer,
  onTrailSeconds,
  twoFloors,
  drawing,
  container,
  returnFocus,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  settings: MapSettings;
  onSettings: (settings: MapSettings) => void;
  layers: ReplayLayers;
  onToggleLayer: (key: LayerToggle) => void;
  onTrailSeconds: (seconds: TrailSeconds) => void;
  /** The map has a lower floor (Nuke, Vertigo), so it can split the floors. */
  twoFloors: boolean;
  /** The drawing shortcuts work here, so the table lists them. */
  drawing: boolean;
  /** The full-screen replay, which sits in the top layer: the dialog opens inside it to stay visible. */
  container: HTMLElement | null;
  /** The button that opened the dialog, which gets the focus back when it closes. */
  returnFocus: RefObject<HTMLButtonElement | null>;
}): ReactNode {
  const show = (key: MapShow): ReactNode => (
    <Check
      key={key}
      label={MAP_SHOW_LABELS[key]}
      hint={MAP_SHOW_HINTS[key]}
      checked={settings.show[key]}
      onChange={(checked) => onSettings({ ...settings, show: { ...settings.show, [key]: checked } })}
    />
  );
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        data-replay-preferences=""
        container={container ?? undefined}
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          returnFocus.current?.focus();
        }}
        className="max-h-[calc(100dvh-2rem)] grid-rows-[auto_minmax(0,1fr)_auto] gap-0 p-0 sm:max-w-2xl"
      >
        <DialogHeader className="border-b border-border-subtle p-4 pr-12">
          <DialogTitle>Preferencias del replay</DialogTitle>
          <DialogDescription>Se aplican al momento y se guardan para todas las partidas.</DialogDescription>
        </DialogHeader>
        <div className="flex min-h-0 flex-col gap-5 overflow-y-auto overscroll-contain p-4 [scrollbar-width:thin]">
          <Group title="Interfaz">
            <Setting label="Dibujo del mapa" className="sm:col-span-2">
              <Segmented
                label="Dibujo del mapa"
                value={settings.art}
                options={MAP_ARTS.map((art) => ({ value: art, label: MAP_ART_LABELS[art] }))}
                onChange={(art) => onSettings({ ...settings, art })}
              />
            </Setting>
            <Setting label="Velocidad del zoom con la rueda">
              <Segmented
                label="Velocidad del zoom con la rueda"
                value={settings.zoom}
                options={MAP_SIZES.map((size) => ({ value: size, label: ZOOM_SPEED_LABELS[size] }))}
                onChange={(zoom) => onSettings({ ...settings, zoom })}
              />
            </Setting>
            <Setting label="Mapa">
              {twoFloors ? (
                <Check
                  label={SEPARATE_FLOORS_LABEL}
                  hint={SEPARATE_FLOORS_HINT}
                  checked={floorsSeparated(settings)}
                  onChange={(checked) => onSettings({ ...settings, floors: checked ? 'both' : 'one' })}
                />
              ) : null}
              {show('zones')}
              {show('scoreline')}
              {show('killfeed')}
            </Setting>
            <Setting label="Capas" className="sm:col-span-2">
              <div className="grid gap-x-6 gap-y-1.5 sm:grid-cols-2">
                {LAYER_ORDER.map((key) => (
                  <div key={key} className="flex flex-wrap items-center gap-x-2 gap-y-1">
                    <Check label={LAYER_LABELS[key]} hint={LAYER_HINTS[key]} checked={layers[key]} onChange={() => onToggleLayer(key)} />
                    {key === 'trails' && layers.trails ? (
                      <Segmented
                        label="Largo de los rastros"
                        value={String(layers.trailSeconds)}
                        options={TRAIL_LENGTHS.map((seconds) => ({ value: String(seconds), label: `${seconds} s` }))}
                        onChange={(value) => {
                          const seconds = TRAIL_LENGTHS.find((length) => String(length) === value);
                          if (seconds !== undefined) onTrailSeconds(seconds);
                        }}
                      />
                    ) : null}
                  </div>
                ))}
              </div>
            </Setting>
          </Group>
          <Group title="Jugador">
            <Setting label="Efectos">
              {show('flashed')}
              {show('damage')}
            </Setting>
            <Setting label="Líneas">{show('trajectories')}</Setting>
            <Setting label="Inventario">{show('held')}</Setting>
            <Setting label="Tamaño de los puntos">
              <Segmented
                label="Tamaño de los puntos"
                value={settings.dots}
                options={DOT_SIZES.map((size) => ({ value: size, label: DOT_SIZE_LABELS[size] }))}
                onChange={(dots) => onSettings({ ...settings, dots })}
              />
            </Setting>
            <Setting label="Mostrar nombres">
              <Check label="En una ronda" checked={settings.namesOne} onChange={(checked) => onSettings({ ...settings, namesOne: checked })} />
              <Check label="En varias rondas" checked={settings.namesMulti} onChange={(checked) => onSettings({ ...settings, namesMulti: checked })} />
            </Setting>
            <Setting label="Tamaño de los nombres">
              <Segmented
                label="Tamaño de los nombres"
                value={settings.names}
                options={MAP_SIZES.map((size) => ({ value: size, label: MAP_SIZE_LABELS[size] }))}
                onChange={(names) => onSettings({ ...settings, names })}
              />
            </Setting>
            <Setting label="Colores del daño">
              <ul className="flex flex-wrap items-center gap-x-3 gap-y-1 text-meta text-fg-2" aria-label="Colores del daño">
                {HIT_LEGEND.map(([kind, label]) => (
                  <li key={kind} className="flex items-center gap-1.5">
                    <span className="h-0.5 w-4 rounded-full" style={{ backgroundColor: `var(${HIT_TOKENS[kind]})` }} aria-hidden />
                    {label}
                  </li>
                ))}
              </ul>
            </Setting>
          </Group>
          <Group title="Rendimiento">
            <Setting label="Imágenes por segundo" hint="Menos imágenes por segundo descargan el PC mientras se reproduce." className="sm:col-span-3">
              <Segmented
                label="Límite de imágenes por segundo"
                value={String(settings.fps)}
                options={FRAME_RATES.map((rate) => ({ value: String(rate), label: frameRateLabel(rate) }))}
                onChange={(value) => onSettings({ ...settings, fps: FRAME_RATES.find((rate) => String(rate) === value) ?? 0 })}
              />
            </Setting>
          </Group>
          <section aria-label="Atajos de teclado" className="flex flex-col gap-3 border-t border-border-subtle pt-4">
            <h3 className="border-b border-border-subtle pb-1.5 text-body-sm font-bold text-fg-1">Atajos de teclado</h3>
            <ShortcutTable drawing={drawing} />
          </section>
        </div>
        <DialogFooter className="border-t border-border-subtle p-3">
          <Button type="button" variant="outline" size="sm" onClick={() => onOpenChange(false)}>
            Cerrar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
