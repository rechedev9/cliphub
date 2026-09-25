'use client';

import { Crosshair, Expand, PanelsTopLeft, ScanEye, type LucideIcon } from 'lucide-react';
import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { CUSTOM_HUD_CAPTURE_PROFILE, CUSTOM_HUD_THEMES, NATIVE_HUD_CAPTURE_PROFILE, customHudTheme, type CustomHudTheme } from '@/lib/custom-hud';
import { uploadFullDemoPortrait, type FullDemoOptions, type FullDemoAssetRef } from '@/lib/full-demo-plan';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { MapCover } from '@/components/brand/map-cover';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { FullDemoChoice, FullDemoGroup } from './full-demo-fields';
import { NativeHudArt } from './full-demo-native-hud';

type HudKind = 'custom' | 'native';
const HUD_KINDS: readonly { value: HudKind; label: string; detail: string; icon: LucideIcon }[] = [
  { value: 'custom', label: 'Diseño de retransmisión', detail: `${CUSTOM_HUD_THEMES.length} diseños con los datos de la demo`, icon: PanelsTopLeft },
  { value: 'native', label: 'Original de CS2', detail: 'El HUD nativo del juego', icon: Crosshair },
];

export function FullDemoHud({ options, map, onChange, onAssetBusy }: {
  options: FullDemoOptions; map: string; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void;
}): ReactNode {
  const theme = customHudTheme(options.overlays.hud_theme);
  // The design (and Focus portrait) set aside for the CS2 HUD, restored when switching back.
  const [setAside, setSetAside] = useState<{ theme?: string; portrait?: FullDemoAssetRef | null }>({});
  const selected = theme ?? customHudTheme(setAside.theme) ?? CUSTOM_HUD_THEMES[0];
  const kind: HudKind = theme ? 'custom' : 'native';
  const portrait = options.overlays.hud_portrait;
  const previewName = kind === 'custom' && selected ? `HUD ${selected.name}` : 'HUD original de CS2';
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
    finally {
      if (!controller.signal.aborted) onAssetBusy(false);
      if (request.current === controller) request.current = null;
    }
  }

  // A pending upload resolves against the options it started with; changing the
  // HUD first must drop it, or its result would bring the old design back.
  function cancelUpload(): void {
    const pending = request.current;
    if (!pending) return;
    request.current = null;
    pending.abort();
    onAssetBusy(false);
  }

  function removePortrait(): void {
    const { hud_portrait: _portrait, ...overlays } = options.overlays;
    onChange({ ...options, overlays });
  }

  function choose(id: string, kept = portrait): void {
    const { hud_portrait: _portrait, ...overlays } = options.overlays;
    cancelUpload();
    setError(null);
    onChange({
      ...options,
      capture: { ...options.capture, hud_profile: CUSTOM_HUD_CAPTURE_PROFILE },
      overlays: { ...overlays, hud_theme: id, ...(id === 'focus' && kept ? { hud_portrait: kept } : {}) },
    });
  }

  function chooseKind(next: HudKind): void {
    if (next === kind) return;
    if (next === 'custom' && selected) { choose(selected.id, setAside.portrait); return; }
    const { hud_theme: _theme, hud_portrait: _portrait, ...overlays } = options.overlays;
    cancelUpload();
    setError(null);
    setSetAside({ theme: theme?.id, portrait });
    onChange({ ...options, capture: { ...options.capture, hud_profile: NATIVE_HUD_CAPTURE_PROFILE }, overlays });
  }

  function chooseTrueView(enabled: boolean): void {
    const { trueview: _trueview, ...capture } = options.capture;
    onChange({ ...options, capture: enabled ? { ...capture, trueview: true } : capture });
  }

  return <FullDemoGroup title="HUD y POV" note="Qué HUD aparece en el vídeo y desde qué vista se graba al jugador.">
    <div className="@container/hud min-w-0">
      <div className="grid min-w-0 items-start gap-4 @min-[40rem]/hud:grid-cols-[minmax(0,1.5fr)_minmax(240px,1fr)]">
        <div className="mx-auto w-full max-w-[min(100%,46dvh)] min-w-0 space-y-1.5 @min-[40rem]/hud:max-w-none">
          <Dialog>
            <DialogTrigger asChild><button type="button" className="relative block w-full min-w-0 overflow-hidden rounded border border-border-subtle text-left transition-colors hover:border-border-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary" aria-label={`Ampliar ${previewName}`}>
              <HudPreview theme={kind === 'custom' ? selected : undefined} map={map} portrait={portrait} />
            </button></DialogTrigger>
            <DialogContent className="max-h-[92dvh] gap-4 overflow-y-auto p-4 sm:max-w-[min(1540px,calc(100%-3rem))]">
              <DialogHeader><DialogTitle>{previewName}</DialogTitle></DialogHeader>
              <div className="min-w-0 overflow-hidden rounded border border-border-subtle"><HudPreview theme={kind === 'custom' ? selected : undefined} map={map} portrait={portrait} /></div>
            </DialogContent>
          </Dialog>
          <p className="flex items-center gap-1.5 text-body-sm text-fg-3"><Expand className="size-3.5 shrink-0" aria-hidden />Ejemplo con datos ilustrativos. Pulsa la imagen para ampliarla.</p>
        </div>
        <div className="grid min-w-0 content-start gap-4 @min-[40rem]/hud:col-start-2 @min-[40rem]/hud:row-span-2 @min-[40rem]/hud:row-start-1">
          <fieldset className="min-w-0">
            <legend className="mb-2 text-body-sm text-fg-2">HUD</legend>
            <div className="grid min-w-0 gap-2 @min-[22rem]/hud:grid-cols-2 @min-[40rem]/hud:grid-cols-1">
              {HUD_KINDS.map((option) => <OptionCard key={option.value} type="radio" name={`${id}-hud`} checked={kind === option.value} icon={option.icon} title={option.label} detail={option.detail} onChange={() => chooseKind(option.value)} />)}
            </div>
          </fieldset>
          {kind === 'custom' && selected ? <div className="grid min-w-0 gap-2">
            <FullDemoChoice label="Diseño" value={selected.id} options={CUSTOM_HUD_THEMES.map((theme) => ({ value: theme.id, label: theme.name }))} onChange={choose} />
            <p className="text-body-sm text-fg-2">{selected.description}</p>
            {selected.id === 'focus' ? <div className="min-w-0 space-y-2">
              <label htmlFor={id} className="text-body-sm text-fg-2">Retrato del jugador (opcional)</label>
              <Input id={id} type="file" accept="image/png,image/jpeg" onChange={(event) => { const file = event.target.files?.[0]; event.target.value = ''; if (file) void upload(file); }} />
              <p className="text-body-sm text-fg-3">PNG o JPG, hasta 10 MB. Un PNG sin fondo encaja como en una retransmisión.</p>
              {portrait ? <Button type="button" variant="secondary" onClick={removePortrait}>Quitar retrato</Button> : null}
              {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
            </div> : null}
          </div> : <p className="text-body-sm text-fg-2">Vida, munición, radar, killfeed y la mira del jugador. Arriba queda la barra de equipos de CS2, con el dinero y la utilidad de ambos equipos.</p>}
        </div>
        <fieldset className="min-w-0 @min-[40rem]/hud:col-start-1 @min-[40rem]/hud:row-start-2">
          <legend className="sr-only">Vista</legend>
          <OptionCard type="checkbox" checked={options.capture.trueview === true} icon={ScanEye} title="POV original 1:1" label="POV original 1:1 (TrueView)"
            detail="TrueView de CS2 recrea la cámara con los movimientos del propio jugador, como la vio él."
            onChange={chooseTrueView} />
        </fieldset>
      </div>
    </div>
  </FullDemoGroup>;
}

/** A labelled radio or checkbox card; the native input keeps keyboard and screen reader behaviour. */
function OptionCard({ type, name, checked, icon: Icon, title, label, detail, onChange }: {
  type: 'radio' | 'checkbox'; name?: string; checked: boolean; icon: LucideIcon; title: string; label?: string; detail: string; onChange: (checked: boolean) => void;
}): ReactNode {
  const detailId = useId();
  return <label className="group flex min-w-0 cursor-pointer items-start gap-3 rounded-md border border-border-subtle bg-surface-1 p-3 transition-colors hover:border-border-strong has-[:checked]:border-primary/60 has-[:checked]:bg-primary/8 has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-primary">
    <span className="grid size-9 shrink-0 place-items-center rounded-md border border-border-subtle bg-surface-2 text-fg-2 group-has-[:checked]:border-primary/40 group-has-[:checked]:text-primary"><Icon className="size-4.5" aria-hidden /></span>
    <span className="min-w-0 flex-1">
      <span className="block text-body-sm font-semibold text-fg-1">{title}</span>
      <span id={detailId} className="mt-1 block text-body-sm text-fg-2">{detail}</span>
    </span>
    <input type={type} name={name} checked={checked} aria-label={label ?? title} aria-describedby={detailId} onChange={(event) => onChange(event.target.checked)}
      className={type === 'radio' ? 'sr-only' : 'mt-0.5 size-4 shrink-0 accent-primary'} />
  </label>;
}

function HudPreview({ theme, map, portrait }: {
  theme?: CustomHudTheme; map: string; portrait?: FullDemoAssetRef | null;
}): ReactNode {
  const hasPortrait = theme?.id === 'focus' && Boolean(portrait);
  return <div className="relative isolate aspect-video w-full overflow-hidden bg-surface-0">
    <MapCover map={map} className="absolute inset-0 opacity-70" />
    {theme ? <img src={`/hud/${theme.id}${hasPortrait ? '-portrait' : ''}.webp`} alt={`Vista previa del HUD ${theme.name}`} className="absolute inset-0 size-full object-contain" width={1920} height={1080} /> : <NativeHudArt />}
    {hasPortrait && portrait ? <img src={`/api/full-demo/overlay-images/${portrait.id}`} alt="Retrato del jugador" className="absolute object-contain object-bottom" style={{ left: `${654 / 1920 * 100}%`, top: `${898 / 1080 * 100}%`, width: `${116 / 1920 * 100}%`, height: `${116 / 1080 * 100}%` }} /> : null}
  </div>;
}
