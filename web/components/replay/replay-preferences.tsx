'use client';

import type { ReactNode } from 'react';
import type { HitLineKind } from '@/lib/replay/hit-lines';
import { FRAME_RATES, MAP_SHOWS, MAP_SHOW_HINTS, MAP_SHOW_LABELS, MAP_SIZES, MAP_SIZE_LABELS, ZOOM_SPEED_LABELS, frameRateLabel } from '@/lib/replay/map-settings';
import type { MapSettings } from '@/lib/replay/map-settings';
import { HIT_TOKENS } from '@/components/tactical/radar-style';
import { Segmented } from '@/components/ui/segmented';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { ReplayChip } from './replay-chip';
import { ShortcutTable } from './shortcut-table';

const HIT_LEGEND: readonly [HitLineKind, string][] = [
  ['bullet', 'Bala'],
  ['he', 'HE'],
  ['fire', 'Fuego'],
  ['flash', 'Flash'],
];

function Group({ title, children }: { title: string; children: ReactNode }): ReactNode {
  return (
    <section aria-label={title} className="flex flex-col gap-3 border-t border-border-subtle pt-4 first:border-t-0 first:pt-0">
      <h3 className="text-body-sm font-semibold text-fg-1">{title}</h3>
      {children}
    </section>
  );
}

function Setting({ label, hint, children }: { label: string; hint?: string; children: ReactNode }): ReactNode {
  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-meta text-fg-2">{label}</span>
      {children}
      {hint === undefined ? null : <span className="text-meta text-fg-3">{hint}</span>}
    </div>
  );
}

/**
 * Ajustes del mapa: how the Replay draws the map and its players, how fast it
 * plays, and every keyboard shortcut. They apply at once and stay for every match.
 */
export function ReplayPreferences({
  open,
  onOpenChange,
  settings,
  onSettings,
  drawing,
  container,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  settings: MapSettings;
  onSettings: (settings: MapSettings) => void;
  /** The drawing shortcuts work here, so the table lists them. */
  drawing: boolean;
  /** The full-screen replay, which sits in the top layer: the sheet opens inside it to stay visible. */
  container: HTMLElement | null;
}): ReactNode {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent container={container ?? undefined} className="gap-0 sm:max-w-md">
        <SheetHeader className="border-b border-border-subtle pr-12">
          <SheetTitle>Ajustes del mapa</SheetTitle>
          <SheetDescription>Se aplican al momento y se guardan para todas las partidas.</SheetDescription>
        </SheetHeader>
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto overscroll-contain p-4 [scrollbar-width:thin]">
          <Group title="Interfaz">
            <Setting label="Velocidad del zoom con la rueda">
              <Segmented
                label="Velocidad del zoom con la rueda"
                value={settings.zoom}
                options={MAP_SIZES.map((size) => ({ value: size, label: ZOOM_SPEED_LABELS[size] }))}
                onChange={(zoom) => onSettings({ ...settings, zoom })}
              />
            </Setting>
            <Setting label="Qué dibuja el mapa">
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
          <Group title="Jugador">
            <Setting label="Tamaño de los puntos">
              <Segmented
                label="Tamaño de los puntos"
                value={settings.dots}
                options={MAP_SIZES.map((size) => ({ value: size, label: MAP_SIZE_LABELS[size] }))}
                onChange={(dots) => onSettings({ ...settings, dots })}
              />
            </Setting>
            <Setting label="Tamaño de los nombres">
              <Segmented
                label="Tamaño de los nombres"
                value={settings.names}
                options={MAP_SIZES.map((size) => ({ value: size, label: MAP_SIZE_LABELS[size] }))}
                onChange={(names) => onSettings({ ...settings, names })}
              />
            </Setting>
          </Group>
          <Group title="Rendimiento">
            <Setting label="Imágenes por segundo" hint="Menos imágenes por segundo descargan el PC mientras se reproduce.">
              <Segmented
                label="Límite de imágenes por segundo"
                value={String(settings.fps)}
                options={FRAME_RATES.map((rate) => ({ value: String(rate), label: frameRateLabel(rate) }))}
                onChange={(value) => onSettings({ ...settings, fps: FRAME_RATES.find((rate) => String(rate) === value) ?? 0 })}
              />
            </Setting>
          </Group>
          <Group title="Atajos de teclado">
            <ShortcutTable drawing={drawing} />
          </Group>
        </div>
      </SheetContent>
    </Sheet>
  );
}
