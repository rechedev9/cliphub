'use client';

import { Expand } from 'lucide-react';
import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { CUSTOM_HUD_CAPTURE_PROFILE, CUSTOM_HUD_THEMES, NATIVE_HUD_CAPTURE_PROFILE, customHudTheme, type CustomHudTheme } from '@/lib/custom-hud';
import { uploadFullDemoPortrait, type FullDemoOptions, type FullDemoAssetRef } from '@/lib/full-demo-plan';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { MapCover } from '@/components/brand/map-cover';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { FullDemoChoice, FullDemoGroup } from './full-demo-fields';
import { NativeHudArt } from './full-demo-native-hud';

type HudKind = 'custom' | 'native';
const HUD_KINDS: readonly { value: HudKind; label: string; detail: string }[] = [
  { value: 'custom', label: 'Diseño de retransmisión', detail: `${CUSTOM_HUD_THEMES.length} diseños con los datos de la demo` },
  { value: 'native', label: 'Original de CS2', detail: 'El HUD nativo del juego' },
];

export function FullDemoHud({ options, onChange, onAssetBusy }: {
  options: FullDemoOptions; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void;
}): ReactNode {
  const theme = customHudTheme(options.overlays.hud_theme);
  // The design (and Focus portrait) set aside for the CS2 HUD, restored when switching back.
  const [setAside, setSetAside] = useState<{ theme?: string; portrait?: FullDemoAssetRef | null }>({});
  const selected = theme ?? customHudTheme(setAside.theme) ?? CUSTOM_HUD_THEMES[0];
  const kind: HudKind = theme ? 'custom' : 'native';
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

  return <FullDemoGroup title="HUD" note="Qué HUD aparece en el vídeo y desde qué vista se graba al jugador.">
    <div className="@container/hud grid min-w-0 gap-4">
      <fieldset className="min-w-0">
        <legend className="sr-only">HUD</legend>
        <div className="grid min-w-0 gap-2.5 @min-[22rem]/hud:grid-cols-2">
          {HUD_KINDS.map((option) => <HudKindCard key={option.value} name={`${id}-hud`} kind={option.value} checked={kind === option.value} title={option.label} detail={option.detail} onChange={() => chooseKind(option.value)} />)}
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
      <fieldset className="min-w-0 border-t border-border-subtle pt-3.5">
        <legend className="sr-only">Vista</legend>
        <label className="flex min-w-0 cursor-pointer items-start gap-3 text-body-sm">
          <input type="checkbox" checked={options.capture.trueview === true} aria-label="POV original 1:1 (TrueView)" aria-describedby={`${id}-trueview`}
            onChange={(event) => chooseTrueView(event.target.checked)} className="mt-0.5 size-4 shrink-0 accent-format-full" />
          <span className="min-w-0">
            <span className="block font-semibold text-fg-1">POV original 1:1</span>
            <span id={`${id}-trueview`} className="mt-0.5 block text-fg-2">TrueView de CS2 recrea la cámara con los movimientos del propio jugador, como la vio él.</span>
          </span>
        </label>
      </fieldset>
    </div>
  </FullDemoGroup>;
}

/** The large preview at the top of the editor; it opens enlarged in a dialog. */
export function FullDemoHudPreview({ options, map }: { options: FullDemoOptions; map: string }): ReactNode {
  const theme = customHudTheme(options.overlays.hud_theme);
  const portrait = options.overlays.hud_portrait;
  const previewName = theme ? `HUD ${theme.name}` : 'HUD original de CS2';
  return <div className="mx-auto w-full max-w-[min(100%,110dvh)] min-w-0 space-y-2">
    <Dialog>
      <DialogTrigger asChild><button type="button" aria-label={`Ampliar ${previewName}`}
        className="relative block w-full min-w-0 overflow-hidden rounded-2xl text-left ring-2 ring-format-full/70 shadow-[var(--elev-2)] transition-shadow hover:ring-format-full focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring">
        <HudPreview theme={theme} map={map} portrait={portrait} />
      </button></DialogTrigger>
      <DialogContent className="max-h-[92dvh] gap-4 overflow-y-auto p-4 sm:max-w-[min(1540px,calc(100%-3rem))]">
        <DialogHeader><DialogTitle>{previewName}</DialogTitle></DialogHeader>
        <div className="min-w-0 overflow-hidden rounded border border-border-subtle"><HudPreview theme={theme} map={map} portrait={portrait} /></div>
      </DialogContent>
    </Dialog>
    <p className="flex items-center gap-1.5 text-body-sm text-fg-3"><Expand className="size-3.5 shrink-0" aria-hidden />Ejemplo con datos ilustrativos. Pulsa la imagen para ampliarla.</p>
  </div>;
}

/** A HUD radio card with a small drawing of its layout; the native radio keeps keyboard and screen reader behaviour. */
function HudKindCard({ name, kind, checked, title, detail, onChange }: {
  name: string; kind: HudKind; checked: boolean; title: string; detail: string; onChange: () => void;
}): ReactNode {
  const detailId = useId();
  const mark = kind === 'custom' ? FORMAT_TONE.full.bg : 'bg-fg-4';
  return <label className="flex min-w-0 cursor-pointer flex-col gap-2 rounded-xl border-[1.5px] border-border-subtle bg-transparent p-3 transition-colors hover:border-border-strong has-[:checked]:border-format-full has-[:checked]:bg-format-full/12 has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring">
    <span aria-hidden className="relative block aspect-video overflow-hidden rounded-lg bg-surface-3">
      <span className={`absolute top-1.5 left-1/2 h-2 -translate-x-1/2 rounded-[3px] ${kind === 'custom' ? 'w-3/5' : 'w-[30%]'} ${mark}`} />
      <span className={`absolute bottom-1.5 left-1.5 h-2.5 w-[30%] rounded-[3px] ${mark}`} />
    </span>
    <span className="text-body-sm font-bold text-fg-1">{title}</span>
    <span id={detailId} className="text-meta text-fg-3">{detail}</span>
    <input type="radio" name={name} checked={checked} aria-label={title} aria-describedby={detailId} onChange={onChange} className="sr-only" />
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
