'use client';

import { Expand } from 'lucide-react';
import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { CUSTOM_HUD_CAPTURE_PROFILE, CUSTOM_HUD_THEMES, customHudTheme, type CustomHudTheme } from '@/lib/custom-hud';
import { uploadFullDemoPortrait, type FullDemoOptions, type FullDemoAssetRef } from '@/lib/full-demo-plan';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { MapCover } from '@/components/brand/map-cover';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { FullDemoChoice, FullDemoGroup } from './full-demo-fields';

export function FullDemoHud({ options, map, onChange, onAssetBusy }: {
  options: FullDemoOptions; map: string; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void;
}): ReactNode {
  const selected = customHudTheme(options.overlays.hud_theme) ?? CUSTOM_HUD_THEMES[0];
  const portrait = options.overlays.hud_portrait;
  const id = useId();
  const [error, setError] = useState<string | null>(null);
  const request = useRef<AbortController | null>(null);
  useEffect(() => () => request.current?.abort(), []);

  async function upload(file: File): Promise<void> {
    const controller = new AbortController(); request.current = controller;
    setError(null); onAssetBusy(true);
    try {
      const ref = await uploadFullDemoPortrait(file, controller.signal);
      if (!controller.signal.aborted) onChange({ ...options, overlays: { ...options.overlays, hud_portrait: ref } });
    } catch (failure) { if (!controller.signal.aborted) setError(failure instanceof Error ? failure.message : 'No se pudo subir el retrato.'); }
    finally { if (!controller.signal.aborted) onAssetBusy(false); }
  }

  function removePortrait(): void {
    const { hud_portrait: _portrait, ...overlays } = options.overlays;
    onChange({ ...options, overlays });
  }

  function choose(id: string): void {
    const { hud_portrait: _portrait, ...overlays } = options.overlays;
    setError(null);
    onChange({
      ...options,
      capture: { ...options.capture, hud_profile: CUSTOM_HUD_CAPTURE_PROFILE },
      overlays: { ...overlays, hud_theme: id, ...(id === 'focus' && portrait ? { hud_portrait: portrait } : {}) },
    });
  }

  if (!selected) return null;
  return <FullDemoGroup title="HUD de la partida" note="Un HUD de retransmisión acompaña al POV del jugador.">
    <div className="@container/hud min-w-0">
      <div className="grid min-w-0 items-start gap-3 @min-[40rem]/hud:grid-cols-[minmax(0,1.6fr)_minmax(190px,1fr)]">
        <Dialog>
        <DialogTrigger asChild><button type="button" className="group relative mx-auto w-full max-w-[min(100%,46dvh)] min-w-0 overflow-hidden rounded border border-border-subtle text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary" aria-label={`Ampliar HUD ${selected.name}`}><HudPreview theme={selected} map={map} portrait={portrait} /><span className="absolute right-2 top-2 flex h-8 items-center gap-1.5 rounded bg-black/80 px-2.5 text-meta text-white"><Expand className="size-3.5" aria-hidden />Ampliar</span></button></DialogTrigger>
          <DialogContent className="max-h-[92dvh] gap-4 overflow-y-auto p-4 sm:max-w-[min(1540px,calc(100%-3rem))]">
            <DialogHeader><DialogTitle>HUD {selected.name}</DialogTitle></DialogHeader>
            <div className="min-w-0 overflow-hidden rounded border border-border-subtle"><HudPreview theme={selected} map={map} portrait={portrait} /></div>
          </DialogContent>
        </Dialog>
        <div className="grid min-w-0 items-end gap-3 @min-[25rem]/hud:grid-cols-[minmax(140px,0.8fr)_minmax(0,1fr)] @min-[40rem]/hud:grid-cols-1">
          <FullDemoChoice label="Diseño" value={selected.id} options={CUSTOM_HUD_THEMES.map((theme) => ({ value: theme.id, label: theme.name }))} onChange={choose} />
          <p className="text-body-sm text-fg-2">{selected.description}</p>
          {selected.id === 'focus' ? <div className="min-w-0 space-y-2">
            <label htmlFor={id} className="text-body-sm text-fg-2">Retrato del jugador (opcional)</label>
            <Input id={id} type="file" accept="image/png,image/jpeg" onChange={(event) => { const file = event.target.files?.[0]; event.target.value = ''; if (file) void upload(file); }} />
            <p className="text-meta text-fg-3">PNG o JPG, hasta 10 MB. Un PNG sin fondo encaja como en una retransmisión.</p>
            {portrait ? <Button type="button" variant="secondary" onClick={removePortrait}>Quitar retrato</Button> : null}
            {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
          </div> : null}
        </div>
      </div>
    </div>
  </FullDemoGroup>;
}

function HudPreview({ theme, map, portrait }: {
  theme: CustomHudTheme; map: string; portrait?: FullDemoAssetRef | null;
}): ReactNode {
  const hasPortrait = theme.id === 'focus' && Boolean(portrait);
  return <div className="relative isolate aspect-video w-full overflow-hidden bg-surface-0">
    <MapCover map={map} className="absolute inset-0 opacity-70" />
    <img src={`/hud/${theme.id}${hasPortrait ? '-portrait' : ''}.webp`} alt={`Vista previa del HUD ${theme.name}`} className="absolute inset-0 size-full object-contain" width={1920} height={1080} />
    {hasPortrait && portrait ? <img src={`/api/full-demo/overlay-images/${portrait.id}`} alt="Retrato del jugador" className="absolute object-contain object-bottom" style={{ left: `${654 / 1920 * 100}%`, top: `${898 / 1080 * 100}%`, width: `${116 / 1920 * 100}%`, height: `${116 / 1080 * 100}%` }} /> : null}
  </div>;
}
