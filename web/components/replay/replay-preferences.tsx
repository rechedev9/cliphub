'use client';

import type { ReactNode, RefObject } from 'react';
import { CircleDot, Keyboard, SlidersHorizontal } from 'lucide-react';
import {
  DOT_SIZES,
  DOT_SIZE_LABELS,
  FRAME_RATES,
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
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Segmented } from '@/components/ui/segmented';
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { ShortcutTable } from './shortcut-table';

export type PreferencesTab = 'general' | 'players' | 'keys';

const PREFERENCES_TABS: readonly PreferencesTab[] = ['general', 'players', 'keys'];

function isPreferencesTab(value: string): value is PreferencesTab {
  return PREFERENCES_TABS.some((tab) => tab === value);
}

const PANEL = 'flex min-h-0 flex-col gap-5 overflow-y-auto overscroll-contain p-5 [scrollbar-width:thin]';
const TRIGGER = 'h-9 flex-none px-2.5';

function Section({ title, children }: { title: string; children: ReactNode }): ReactNode {
  return (
    <section aria-label={title} className="flex flex-col gap-2">
      <h3 className="text-body-sm font-semibold text-fg-1">{title}</h3>
      <div className="flex flex-col divide-y divide-border-subtle rounded-md border border-border-subtle bg-surface-1">{children}</div>
    </section>
  );
}

function Text({ label, hint }: { label: string; hint?: string }): ReactNode {
  return (
    <span className="flex min-w-0 flex-col">
      <span className="text-body-sm text-fg-1">{label}</span>
      {hint === undefined ? null : <span className="text-meta text-fg-3">{hint}</span>}
    </span>
  );
}

function Toggle({ label, hint, checked, onChange }: { label: string; hint?: string; checked: boolean; onChange: (checked: boolean) => void }): ReactNode {
  return (
    <label className="flex min-h-11 cursor-pointer items-center justify-between gap-4 px-3 py-2 transition-colors hover:bg-surface-2">
      <Text label={label} hint={hint} />
      <Switch checked={checked} onCheckedChange={onChange} aria-label={label} />
    </label>
  );
}

function Choice({ label, hint, children }: { label: string; hint?: string; children: ReactNode }): ReactNode {
  return (
    <div className="flex min-h-11 items-center justify-between gap-4 px-3 py-2">
      <Text label={label} hint={hint} />
      <div className="shrink-0">{children}</div>
    </div>
  );
}

/**
 * Preferencias del replay, with the settings of EDGE's replayer in three tabs: what the map and
 * timeline draw, how each player is drawn, and every keyboard shortcut. They apply at once.
 */
export function ReplayPreferences({
  open,
  onOpenChange,
  tab,
  onTabChange,
  settings,
  onSettings,
  twoFloors,
  drawing,
  container,
  returnFocus,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  tab: PreferencesTab;
  onTabChange: (tab: PreferencesTab) => void;
  settings: MapSettings;
  onSettings: (settings: MapSettings) => void;
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
    <Toggle
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
        className="h-[min(42rem,calc(100dvh-2rem))] grid-rows-[auto_minmax(0,1fr)] gap-0 p-0 sm:max-w-2xl"
      >
        <DialogHeader className="border-b border-border-subtle px-5 py-4 pr-12">
          <DialogTitle>Preferencias del replay</DialogTitle>
          <DialogDescription>Se aplican al momento y se guardan para todas las partidas.</DialogDescription>
        </DialogHeader>
        <Tabs
          orientation="vertical"
          value={tab}
          onValueChange={(value) => {
            if (isPreferencesTab(value)) onTabChange(value);
          }}
          className="min-h-0 flex-row gap-0"
        >
          <TabsList variant="line" className="w-40 shrink-0 justify-start gap-0.5 self-stretch border-r border-border-subtle py-2 pr-1 pl-2">
            <TabsTrigger value="general" className={TRIGGER}>
              <SlidersHorizontal aria-hidden />
              General
            </TabsTrigger>
            <TabsTrigger value="players" className={TRIGGER}>
              <CircleDot aria-hidden />
              Jugadores
            </TabsTrigger>
            <TabsTrigger value="keys" className={TRIGGER}>
              <Keyboard aria-hidden />
              Atajos
            </TabsTrigger>
          </TabsList>
          <TabsContent value="general" className={PANEL}>
            <Section title="Mapa">
              {show('killfeed')}
              {show('scoreline')}
              {twoFloors ? (
                <Toggle
                  label={SEPARATE_FLOORS_LABEL}
                  hint={SEPARATE_FLOORS_HINT}
                  checked={floorsSeparated(settings)}
                  onChange={(checked) => onSettings({ ...settings, floors: checked ? 'both' : 'one' })}
                />
              ) : null}
              <Choice label="Zoom con la rueda">
                <Segmented
                  label="Zoom con la rueda"
                  value={settings.zoom}
                  options={MAP_SIZES.map((size) => ({ value: size, label: ZOOM_SPEED_LABELS[size] }))}
                  onChange={(zoom) => onSettings({ ...settings, zoom })}
                />
              </Choice>
            </Section>
            <Section title="Línea de tiempo">
              {show('tlEvents')}
              {show('tlUtility')}
              {show('tlKills')}
            </Section>
            <Section title="Reproducción">
              {show('freezetime')}
              <Choice label="Imágenes por segundo" hint="Menos imágenes descargan el PC mientras se reproduce">
                <Segmented
                  label="Límite de imágenes por segundo"
                  value={String(settings.fps)}
                  options={FRAME_RATES.map((rate) => ({ value: String(rate), label: frameRateLabel(rate) }))}
                  onChange={(value) => onSettings({ ...settings, fps: FRAME_RATES.find((rate) => String(rate) === value) ?? 0 })}
                />
              </Choice>
            </Section>
          </TabsContent>
          <TabsContent value="players" className={PANEL}>
            <Section title="Puntos">
              <Choice label="Tamaño">
                <Segmented
                  label="Tamaño de los puntos"
                  value={settings.dots}
                  options={DOT_SIZES.map((size) => ({ value: size, label: DOT_SIZE_LABELS[size] }))}
                  onChange={(dots) => onSettings({ ...settings, dots })}
                />
              </Choice>
            </Section>
            <Section title="Nombres">
              <Toggle label="En una ronda" checked={settings.namesOne} onChange={(checked) => onSettings({ ...settings, namesOne: checked })} />
              <Toggle label="En varias rondas" checked={settings.namesMulti} onChange={(checked) => onSettings({ ...settings, namesMulti: checked })} />
              <Choice label="Tamaño">
                <Segmented
                  label="Tamaño de los nombres"
                  value={settings.names}
                  options={MAP_SIZES.map((size) => ({ value: size, label: MAP_SIZE_LABELS[size] }))}
                  onChange={(names) => onSettings({ ...settings, names })}
                />
              </Choice>
            </Section>
            <Section title="Efectos">
              {show('flashed')}
              {show('damage')}
            </Section>
            <Section title="Líneas">
              {show('trajectories')}
              {show('bullets')}
            </Section>
            <Section title="Inventario">
              {show('held')}
              {show('bomb')}
              {show('kit')}
            </Section>
          </TabsContent>
          <TabsContent value="keys" className={PANEL}>
            <p className="text-meta text-fg-3">Funcionan mientras el Replay está en pantalla y no escribes en un campo.</p>
            <ShortcutTable drawing={drawing} />
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
