'use client';

import { Expand } from 'lucide-react';
import type { ReactNode } from 'react';
import { CUSTOM_HUD_CAPTURE_PROFILE, CUSTOM_HUD_THEMES, customHudTheme, type CustomHudTheme } from '@/lib/custom-hud';
import type { FullDemoOptions } from '@/lib/full-demo-plan';
import { MapCover } from '@/components/brand/map-cover';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { FullDemoChoice, FullDemoGroup } from './full-demo-fields';

export function FullDemoHud({ options, map, onChange }: {
  options: FullDemoOptions; map: string; onChange: (options: FullDemoOptions) => void;
}): ReactNode {
  const selected = customHudTheme(options.overlays.hud_theme) ?? CUSTOM_HUD_THEMES[0];

  function choose(id: string): void {
    onChange({
      ...options,
      capture: { ...options.capture, hud_profile: CUSTOM_HUD_CAPTURE_PROFILE },
      overlays: { ...options.overlays, hud_theme: id },
    });
  }

  if (!selected) return null;
  return <FullDemoGroup title="HUD de la partida" note="Un HUD de retransmisión acompaña al POV del jugador.">
    <div className="@container/hud min-w-0">
      <div className="grid min-w-0 items-start gap-3 @min-[40rem]/hud:grid-cols-[minmax(0,1.6fr)_minmax(190px,1fr)]">
        <Dialog>
        <DialogTrigger asChild><button type="button" className="group relative mx-auto w-full max-w-[min(100%,46dvh)] min-w-0 overflow-hidden rounded border border-border-subtle text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary" aria-label={`Ampliar HUD ${selected.name}`}><HudPreview theme={selected} map={map} /><span className="absolute bottom-2 left-1/2 flex h-8 -translate-x-1/2 items-center gap-1.5 rounded bg-black/80 px-2.5 text-meta text-white"><Expand className="size-3.5" aria-hidden />Ampliar</span></button></DialogTrigger>
          <DialogContent className="max-h-[92dvh] gap-4 overflow-y-auto p-4 sm:max-w-[min(1540px,calc(100%-3rem))]">
            <DialogHeader><DialogTitle>HUD {selected.name}</DialogTitle></DialogHeader>
            <div className="min-w-0 overflow-hidden rounded border border-border-subtle"><HudPreview theme={selected} map={map} /></div>
          </DialogContent>
        </Dialog>
        <div className="grid min-w-0 items-end gap-3 @min-[25rem]/hud:grid-cols-[minmax(140px,0.8fr)_minmax(0,1fr)] @min-[40rem]/hud:grid-cols-1">
          <FullDemoChoice label="Diseño" value={selected.id} options={CUSTOM_HUD_THEMES.map((theme) => ({ value: theme.id, label: theme.name }))} onChange={choose} />
          <p className="text-body-sm text-fg-2">{selected.description}</p>
        </div>
      </div>
    </div>
  </FullDemoGroup>;
}

function HudPreview({ theme, map }: {
  theme: CustomHudTheme; map: string;
}): ReactNode {
  return <div className="relative isolate aspect-video w-full overflow-hidden bg-surface-0">
    <MapCover map={map} className="absolute inset-0 opacity-70" />
    <img src={`/hud/${theme.id}.webp`} alt={`Vista previa del HUD ${theme.name}`} className="absolute inset-0 size-full object-contain" width={1920} height={1080} />
  </div>;
}
