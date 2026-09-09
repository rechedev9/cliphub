'use client';

import { Check, Expand, Palette, Sun } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { CUSTOM_HUD_CAPTURE_PROFILE, CUSTOM_HUD_THEMES, customHudTheme, type CustomHudTheme } from '@/lib/custom-hud';
import type { FullDemoOptions } from '@/lib/full-demo-plan';
import { cn } from '@/lib/utils';
import { MapCover } from '@/components/brand/map-cover';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { FullDemoGroup, FullDemoToggle } from './full-demo-fields';

export function FullDemoHud({ options, map, onChange }: {
  options: FullDemoOptions; map: string; onChange: (options: FullDemoOptions) => void;
}): ReactNode {
  const selected = customHudTheme(options.overlays.hud_theme);
  const [detail, setDetail] = useState<'full' | 'score' | 'player' | 'loadout'>('full');
  const [light, setLight] = useState(false);

  function choose(id: string | null): void {
    const { hud_theme: _previous, ...native } = options.overlays;
    onChange({
      ...options,
      capture: { ...options.capture, hud_profile: id ? CUSTOM_HUD_CAPTURE_PROFILE : 'native-clean-spectator' },
      overlays: id ? { ...native, hud_theme: id } : native,
    });
  }

  return <FullDemoGroup title="HUD de la partida" note="Marcador y jugadores arriba, vida a la izquierda y arma a la derecha. El POV permanece en el jugador elegido.">
    <FullDemoToggle label="Utilizar un custom HUD" value={Boolean(selected)} onChange={(enabled) => choose(enabled ? CUSTOM_HUD_THEMES[0].id : null)} />
    {selected ? <div className="space-y-5">
      <div className="grid min-w-0 items-start gap-5 @[56rem]/content:grid-cols-[minmax(0,2.4fr)_minmax(200px,1fr)]">
        <div className="min-w-0 space-y-2">
          <Dialog>
            <DialogTrigger asChild>
              <button type="button" className="group relative block w-full min-w-0 cursor-zoom-in border border-border-subtle text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary" aria-label={`Ampliar HUD ${selected.name}`} data-testid="custom-hud-preview">
                <HudPreview theme={selected} map={map} />
                <span className="absolute bottom-3 left-1/2 flex -translate-x-1/2 items-center gap-2 rounded-md border border-white/15 bg-black/80 px-3 py-1.5 text-meta text-white transition-colors group-hover:bg-black"><Expand className="size-3.5" aria-hidden />Ampliar</span>
              </button>
            </DialogTrigger>
            <DialogContent className="max-h-[92dvh] gap-4 overflow-y-auto p-4 sm:max-w-[min(1540px,calc(100%-3rem))] sm:p-6">
              <DialogHeader>
                <DialogTitle>HUD {selected.name}</DialogTitle>
                <DialogDescription>{selected.description}</DialogDescription>
              </DialogHeader>
              <div className="flex flex-wrap items-center gap-2">
                <div className="flex flex-wrap gap-1" role="group" aria-label="Detalle de la vista previa">
                  {([['full', 'Vista completa'], ['score', 'Marcador'], ['player', 'Jugador'], ['loadout', 'Arma']] as const).map(([value, label]) => <Button key={value} type="button" size="sm" variant={detail === value ? 'secondary' : 'ghost'} aria-pressed={detail === value} onClick={() => setDetail(value)}>{label}</Button>)}
                </div>
                <Button type="button" size="sm" variant="outline" aria-pressed={light} onClick={() => setLight(!light)}><Sun aria-hidden />Fondo claro</Button>
                <label className="ml-auto flex min-w-0 items-center gap-2 text-body-sm text-fg-2">Diseño
                  <select className="h-10 min-w-0 rounded-md border border-border-strong bg-surface-3 px-3 text-fg-1 focus-visible:outline-2 focus-visible:outline-primary" value={selected.id} onChange={(event) => choose(event.target.value)} aria-label="Diseño en vista ampliada">
                    {CUSTOM_HUD_THEMES.map((theme) => <option key={theme.id} value={theme.id}>{theme.name}</option>)}
                  </select>
                </label>
              </div>
              <div className="min-w-0 overflow-hidden rounded-lg border border-border-subtle" data-testid="custom-hud-expanded">
                <HudPreview theme={selected} map={map} detail={detail} light={light} />
              </div>
              <p className="text-meta text-fg-3">Datos de ejemplo · El vídeo utilizará los nombres, armas y estadísticas de tu partida.</p>
            </DialogContent>
          </Dialog>
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
              <div className="absolute inset-0 opacity-60"><MapCover map={map} /></div>
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

function HudPreview({ theme, map, detail = 'full', light = false }: {
  theme: CustomHudTheme; map: string; detail?: 'full' | 'score' | 'player' | 'loadout'; light?: boolean;
}): ReactNode {
  let viewBox = '0 0 1920 1080';
  if (detail === 'score') viewBox = `${(1920 - theme.score_width) / 2 - 16} 12 ${theme.score_width + 32} 100`;
  if (detail === 'player') viewBox = `${theme.focus_x - 16} ${theme.focus_y - 16} ${theme.focus_width + 32} 134`;
  if (detail === 'loadout') viewBox = `${theme.loadout_x - 16} ${theme.loadout_y - 16} ${theme.loadout_width + 32} 122`;
  return <div className={cn('relative isolate aspect-video w-full overflow-hidden', light ? 'bg-[#d4dce2]' : 'bg-surface-0')}>
    <MapCover map={map} className={cn('absolute inset-0', light ? 'opacity-30' : 'opacity-70')} />
    {detail === 'full' ? <img src={`/hud/${theme.id}.webp`} alt={`Vista previa del HUD ${theme.name}`} className="absolute inset-0 size-full object-contain" width={1920} height={1080} /> : <svg viewBox={viewBox} className="absolute inset-0 size-full" role="img" aria-label={`Vista previa del HUD ${theme.name}`}>
      <image href={`/hud/${theme.id}.webp`} width={1920} height={1080} />
    </svg>}
  </div>;
}

function TeamColor({ label, color }: { label: string; color: string }): ReactNode {
  return <span className="inline-flex items-center gap-2"><span className="size-3 rounded-full border border-white/15" style={{ backgroundColor: `#${color}` }} aria-hidden />{label}</span>;
}
