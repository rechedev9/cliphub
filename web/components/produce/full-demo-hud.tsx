'use client';

import { Check, Palette } from 'lucide-react';
import type { ReactNode } from 'react';
import { CUSTOM_HUD_CAPTURE_PROFILE, CUSTOM_HUD_THEMES, customHudTheme } from '@/lib/custom-hud';
import type { FullDemoOptions } from '@/lib/full-demo-plan';
import { cn } from '@/lib/utils';
import { MapCover } from '@/components/brand/map-cover';
import { FullDemoGroup, FullDemoToggle } from './full-demo-fields';

export function FullDemoHud({ options, map, onChange }: {
  options: FullDemoOptions; map: string; onChange: (options: FullDemoOptions) => void;
}): ReactNode {
  const selected = customHudTheme(options.overlays.hud_theme);

  function choose(id: string | null): void {
    const { hud_theme: _previous, ...native } = options.overlays;
    onChange({
      ...options,
      capture: { ...options.capture, hud_profile: id ? CUSTOM_HUD_CAPTURE_PROFILE : 'native-clean-spectator' },
      overlays: id ? { ...native, hud_theme: id } : native,
    });
  }

  return <FullDemoGroup title="HUD de la partida" note="Elige el aspecto del marcador y las fichas de jugadores. El radar, el killfeed y la mira se conservan.">
    <FullDemoToggle label="Utilizar un custom HUD" value={Boolean(selected)} onChange={(enabled) => choose(enabled ? CUSTOM_HUD_THEMES[0].id : null)} />
    {selected ? <div className="space-y-5">
      <div className="grid min-w-0 items-start gap-5 @[56rem]/content:grid-cols-[minmax(0,1.65fr)_minmax(220px,1fr)]">
        <div className="min-w-0 space-y-2">
          <div className="relative isolate aspect-video overflow-hidden border border-border-subtle bg-surface-0" data-testid="custom-hud-preview">
            <MapCover map={map} className="absolute inset-0 opacity-55" />
            <img src={`/hud/${selected.id}.webp`} alt={`Vista previa del HUD ${selected.name}`} className="absolute inset-0 size-full object-contain" width={1920} height={1080} />
          </div>
          <p className="text-meta text-fg-3">Vista previa de diseño · datos de ejemplo</p>
        </div>
        <div className="min-w-0 space-y-3">
          <p className="flex items-center gap-2 font-mono text-meta uppercase tracking-widest text-primary"><Palette className="size-4" aria-hidden /> Custom HUD · 16:9</p>
          <h3 className="break-words font-display text-display-sm font-bold uppercase text-fg-1">{selected.name}</h3>
          <p className="text-body-sm text-fg-2">{selected.description}</p>
          <div className="flex flex-wrap gap-3 text-meta text-fg-2">
            <TeamColor label="CT" color={selected.ct} /><TeamColor label="T" color={selected.t} />
          </div>
          <p className="border-t border-border-subtle pt-3 text-body-sm text-fg-2">La primera vez se captura una base compatible. Después puedes cambiar de estilo reutilizando esa captura.</p>
        </div>
      </div>
      <fieldset className="min-w-0">
        <legend className="mb-3 font-mono text-meta uppercase tracking-widest text-fg-3">10 diseños</legend>
        <div className="grid min-w-0 grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5">
          {CUSTOM_HUD_THEMES.map((theme, index) => <label key={theme.id} className={cn(
            'relative min-w-0 cursor-pointer border bg-surface-1 transition-colors hover:border-fg-3 focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-primary',
            selected.id === theme.id ? 'border-primary' : 'border-border-subtle',
          )}>
            <input type="radio" name="custom-hud-theme" value={theme.id} checked={selected.id === theme.id} onChange={() => choose(theme.id)} aria-label={`HUD ${theme.name}`} className="sr-only" />
            <div className="relative aspect-video overflow-hidden bg-surface-0">
              <div className="absolute inset-0 opacity-25"><MapCover map={map} /></div>
              <img src={`/hud/${theme.id}.webp`} alt="" loading="lazy" className="relative size-full object-contain" width={384} height={216} />
            </div>
            <div className="flex min-w-0 items-center justify-between gap-2 px-3 py-2.5">
              <span className="min-w-0 break-words font-display text-body-sm font-semibold text-fg-1"><span className="mr-2 font-mono text-meta text-fg-3">{String(index + 1).padStart(2, '0')}</span>{theme.name}</span>
              {selected.id === theme.id ? <Check className="size-4 shrink-0 text-primary" aria-hidden /> : <span className="flex shrink-0 gap-1" aria-hidden><span className="size-2.5 rounded-full" style={{ backgroundColor: `#${theme.ct}` }} /><span className="size-2.5 rounded-full" style={{ backgroundColor: `#${theme.t}` }} /></span>}
            </div>
          </label>)}
        </div>
      </fieldset>
    </div> : <p className="text-body-sm text-fg-2">Se utilizará el HUD nativo de CS2. Activa los diseños para ver y elegir uno de los diez estilos.</p>}
  </FullDemoGroup>;
}

function TeamColor({ label, color }: { label: string; color: string }): ReactNode {
  return <span className="inline-flex items-center gap-2"><span className="size-3 rounded-full border border-white/15" style={{ backgroundColor: `#${color}` }} aria-hidden />{label}</span>;
}
