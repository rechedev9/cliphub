'use client';

import { ChevronDown, Expand, Upload } from 'lucide-react';
import { useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { CUSTOM_HUD_CAPTURE_PROFILE, CUSTOM_HUD_THEMES, NATIVE_HUD_CAPTURE_PROFILE, customHudTheme, type CustomHudTheme } from '@/lib/custom-hud';
import { uploadFullDemoPortrait, type FullDemoOptions, type FullDemoAssetRef } from '@/lib/full-demo-plan';
import { hudPreviewUrl, needsDrawnPreview, type HudOptions, type HudStyle, type HudTeamNames } from '@/lib/hud-themes';
import { mapShotUrl } from '@/lib/api/game-assets';
import { humanErrorText } from '@/lib/human-error';
import { Button } from '@/components/ui/button';
import { MapCover } from '@/components/brand/map-cover';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { CoverImage } from '@/components/studio/cover-image';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { FULL_DEMO_RADAR_LABEL, FULL_DEMO_TRUEVIEW_LABEL } from '@/lib/produce/full-demo-brief';
import { FullDemoGroup } from './full-demo-fields';
import { HudCompositionSwitches, HudTeamNameFields, HudThemePicker } from './full-demo-hud-look';
import { NativeHudArt } from './full-demo-native-hud';

type HudKind = 'custom' | 'native';
/** The preview is illustrative, so it always sits on the same map whatever the demo's. */
const PREVIEW_MAP = 'de_dust2';
const HUD_KINDS: readonly { value: HudKind; label: string; detail: string }[] = [
  { value: 'custom', label: 'Diseño de retransmisión', detail: `${CUSTOM_HUD_THEMES.length} diseños con los datos de la demo` },
  { value: 'native', label: 'Original de CS2', detail: 'El HUD nativo del juego' },
];

export function FullDemoHud({ options, onChange, onAssetBusy }: {
  options: FullDemoOptions; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void;
}): ReactNode {
  const theme = customHudTheme(options.overlays.hud_theme);
  // The design (and Focus portrait) set aside for the CS2 HUD, restored when switching back.
  const [setAside, setSetAside] = useState<{ theme?: string; portrait?: FullDemoAssetRef | null; style?: HudStyle; look?: HudOptions; names?: HudTeamNames }>({});
  const selected = theme ?? customHudTheme(setAside.theme) ?? CUSTOM_HUD_THEMES[0];
  const kind: HudKind = theme ? 'custom' : 'native';
  const portrait = options.overlays.hud_portrait;
  const id = useId();
  const [error, setError] = useState<string | null>(null);
  const [portraitName, setPortraitName] = useState<{ id: string; name: string } | null>(null);
  const request = useRef<AbortController | null>(null);
  const portraitInput = useRef<HTMLInputElement>(null);
  const kinds = useRef<HTMLFieldSetElement>(null);
  /** Viewport top of the HUD cards before a kind switch, to undo the scroll jump its collapse causes. */
  const kindsTop = useRef<number | null>(null);
  useEffect(() => () => request.current?.abort(), []);
  useLayoutEffect(() => {
    const before = kindsTop.current;
    kindsTop.current = null;
    const element = kinds.current;
    if (before === null || !element) return;
    const shift = element.getBoundingClientRect().top - before;
    if (Math.abs(shift) >= 1) scrollParent(element).scrollTop += shift;
  }, [kind]);

  async function upload(file: File): Promise<void> {
    const controller = new AbortController(); request.current = controller;
    setError(null); onAssetBusy(true);
    try {
      const ref = await uploadFullDemoPortrait(file, controller.signal);
      if (!controller.signal.aborted) { setPortraitName({ id: ref.id, name: file.name }); onChange({ ...options, overlays: { ...options.overlays, hud_portrait: ref } }); }
    } catch (failure) { if (!controller.signal.aborted) setError(humanErrorText({ error: failure, lead: 'No se pudo subir el retrato.' })); }
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

  function choose(id: string, kept = portrait, style?: HudStyle, restored?: { look?: HudOptions; names?: HudTeamNames }): void {
    const { hud_portrait: _portrait, hud_custom: _custom, ...current } = options.overlays;
    // The CS2 HUD set the switches and names aside; coming back restores them.
    const overlays = restored ? { ...current, ...(restored.look ? { hud_options: restored.look } : {}), ...(restored.names ? { team_names: restored.names } : {}) } : current;
    cancelUpload();
    setError(null);
    onChange({
      ...options,
      capture: { ...options.capture, hud_profile: CUSTOM_HUD_CAPTURE_PROFILE },
      overlays: { ...overlays, hud_theme: id, ...(id === 'focus' && kept ? { hud_portrait: kept } : {}), ...(style ? { hud_custom: style } : {}) },
    });
  }

  function chooseKind(next: HudKind): void {
    if (next === kind) return;
    kindsTop.current = kinds.current?.getBoundingClientRect().top ?? null;
    if (next === 'custom' && selected) { choose(selected.id, setAside.portrait, setAside.style, setAside); return; }
    const { hud_theme: _theme, hud_portrait: _portrait, hud_custom: style, hud_options: look, team_names: names, ...overlays } = options.overlays;
    cancelUpload();
    setError(null);
    setSetAside({ theme: theme?.id, portrait, style, look, names });
    const { cliphub_radar: _radar, ...capture } = options.capture;
    onChange({ ...options, capture: { ...capture, hud_profile: NATIVE_HUD_CAPTURE_PROFILE }, overlays });
  }

  function chooseTrueView(enabled: boolean): void {
    const { trueview: _trueview, ...capture } = options.capture;
    onChange({ ...options, capture: enabled ? { ...capture, trueview: true } : capture });
  }

  function chooseRadar(enabled: boolean): void {
    const { cliphub_radar: _radar, ...capture } = options.capture;
    onChange({ ...options, capture: enabled ? { ...capture, cliphub_radar: true } : capture });
  }

  return <FullDemoGroup title="HUD" note="Qué HUD aparece en el vídeo y desde qué vista se graba al jugador.">
    <div className="@container/hud grid min-w-0 gap-4">
      <fieldset ref={kinds} className="min-w-0">
        <legend className="sr-only">HUD</legend>
        <div className="grid min-w-0 gap-2.5 @min-[22rem]/hud:grid-cols-2">
          {HUD_KINDS.map((option) => <HudKindCard key={option.value} name={`${id}-hud`} kind={option.value} checked={kind === option.value} title={option.label} detail={option.detail} onChange={() => chooseKind(option.value)} />)}
        </div>
      </fieldset>
      {kind === 'custom' && selected ? <div className="grid min-w-0 gap-2">
        {/* The gallery folds away; while open, each design picked shows at once in the preview. */}
        <details className="group min-w-0 rounded-xl border-[1.5px] border-border-subtle open:border-format-full/60">
          <summary className="flex min-h-11 cursor-pointer list-none items-center justify-between gap-3 rounded-xl px-3 text-body-sm focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
            <span className="min-w-0 truncate"><span className="text-fg-2">Diseño · </span><span className="font-semibold text-fg-1">{options.overlays.hud_custom?.name ?? selected.name}</span></span>
            <ChevronDown aria-hidden className="size-4 shrink-0 text-fg-2 transition-transform group-open:rotate-180" />
          </summary>
          <div className="min-w-0 border-t border-border-subtle p-3">
            <HudThemePicker base={selected.id} style={options.overlays.hud_custom} backdrop={<PreviewBackdrop />} onSelect={(id, style) => choose(id, portrait, style)} />
          </div>
        </details>
        <p className="text-body-sm text-fg-2">{selected.description}</p>
        <p className="text-body-sm text-fg-3">Todos los diseños añaden bajo el marcador los jugadores vivos de cada bando, la racha de 4 o más rondas, el 3K, 4K o ACE de la ronda, la barra de desactivación con o sin kit y el MVP al cerrar cada ronda.</p>
        {selected.id === 'focus' ? <div className="min-w-0 space-y-2">
          <p id={id} className="text-body-sm text-fg-2">Retrato del jugador (opcional)</p>
          <input ref={portraitInput} type="file" accept="image/png,image/jpeg" className="sr-only" tabIndex={-1} aria-labelledby={id}
            onChange={(event) => { const file = event.target.files?.[0]; event.target.value = ''; if (file) void upload(file); }} />
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <Button type="button" variant="secondary" aria-describedby={id} onClick={() => portraitInput.current?.click()}>
              <Upload aria-hidden />{portrait ? 'Cambiar retrato' : 'Subir retrato'}
            </Button>
            {portrait ? <span className="min-w-0 truncate text-body-sm text-fg-1">{portraitName?.id === portrait.id ? portraitName.name : 'Retrato subido'}</span> : null}
            {portrait ? <Button type="button" variant="ghost" onClick={removePortrait}>Quitar retrato</Button> : null}
          </div>
          <p className="text-body-sm text-fg-3">PNG o JPG, hasta 10 MB. Un PNG sin fondo encaja como en una retransmisión.</p>
          {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
        </div> : null}
        <HudCompositionSwitches options={options.overlays.hud_options}
          onOptions={(hud_options) => onChange({ ...options, overlays: { ...options.overlays, hud_options } })} />
        <label className="flex min-w-0 cursor-pointer items-start gap-3 text-body-sm">
          <input type="checkbox" checked={options.capture.cliphub_radar === true} aria-label={FULL_DEMO_RADAR_LABEL} aria-describedby={`${id}-radar`}
            onChange={(event) => chooseRadar(event.target.checked)} className="mt-0.5 size-4 shrink-0 accent-format-full" />
          <span className="min-w-0">
            <span className="block font-semibold text-fg-1">{FULL_DEMO_RADAR_LABEL}</span>
            <span id={`${id}-radar`} className="mt-0.5 block text-fg-2">Arriba a la izquierda, en lugar del de CS2: el mapa del Replay 2D, cada jugador y hacia dónde mira, humos, fuegos, flashes, muertes y la bomba, sin nombres de zona. Se vuelve a grabar.</span>
          </span>
        </label>
        <HudTeamNameFields names={options.overlays.team_names}
          onNames={(team_names) => onChange({ ...options, overlays: { ...options.overlays, team_names } })} />
      </div> : <p className="text-body-sm text-fg-2">Vida, munición, radar, killfeed y la mira del jugador. Arriba queda la barra de equipos de CS2, con el dinero y la utilidad de ambos equipos.</p>}
      <fieldset className="min-w-0 border-t border-border-subtle pt-3.5">
        <legend className="sr-only">Vista</legend>
        <label className="flex min-w-0 cursor-pointer items-start gap-3 text-body-sm">
          <input type="checkbox" checked={options.capture.trueview === true} aria-label="POV original 1:1 (TrueView)" aria-describedby={`${id}-trueview`}
            onChange={(event) => chooseTrueView(event.target.checked)} className="mt-0.5 size-4 shrink-0 accent-format-full" />
          <span className="min-w-0">
            <span className="block font-semibold text-fg-1">{FULL_DEMO_TRUEVIEW_LABEL}</span>
            <span id={`${id}-trueview`} className="mt-0.5 block text-fg-2">TrueView de CS2 recrea la cámara con los movimientos del propio jugador, como la vio él.</span>
          </span>
        </label>
      </fieldset>
    </div>
  </FullDemoGroup>;
}

/** The large preview at the top of the editor; it opens enlarged in a dialog. */
export function FullDemoHudPreview({ options }: { options: FullDemoOptions }): ReactNode {
  const theme = customHudTheme(options.overlays.hud_theme);
  const portrait = options.overlays.hud_portrait;
  const previewName = theme ? `HUD ${options.overlays.hud_custom?.name ?? theme.name}` : 'HUD original de CS2';
  return <div className="mx-auto w-full max-w-[min(100%,110dvh)] min-w-0 space-y-2">
    <Dialog>
      <DialogTrigger asChild><button type="button" aria-label={`Ampliar ${previewName}`}
        className="relative block w-full min-w-0 overflow-hidden rounded-2xl text-left ring-2 ring-format-full/70 shadow-[var(--elev-2)] transition-shadow hover:ring-format-full focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring">
        <HudPreview theme={theme} portrait={portrait} overlays={options.overlays} />
      </button></DialogTrigger>
      <DialogContent className="max-h-[92dvh] gap-4 overflow-y-auto p-4 sm:max-w-[min(1540px,calc(100%-3rem))]">
        <DialogHeader><DialogTitle>{previewName}</DialogTitle></DialogHeader>
        <div className="min-w-0 overflow-hidden rounded border border-border-subtle"><HudPreview theme={theme} portrait={portrait} overlays={options.overlays} /></div>
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
  return <label className="flex min-w-0 cursor-pointer items-center gap-3 rounded-xl border-[1.5px] border-border-subtle bg-transparent p-2.5 transition-colors hover:border-border-strong has-[:checked]:border-format-full has-[:checked]:bg-format-full/12 has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring">
    <span aria-hidden className="relative block aspect-video w-16 shrink-0 overflow-hidden rounded-md bg-surface-3">
      <span className={`absolute top-1 left-1/2 h-1.5 -translate-x-1/2 rounded-[2px] ${kind === 'custom' ? 'w-3/5' : 'w-[30%]'} ${mark}`} />
      <span className={`absolute bottom-1 left-1 h-1.5 w-[30%] rounded-[2px] ${mark}`} />
    </span>
    <span className="flex min-w-0 flex-col gap-0.5">
      <span className="text-body-sm font-bold text-fg-1">{title}</span>
      <span id={detailId} className="text-meta text-fg-3">{detail}</span>
    </span>
    <input type="radio" name={name} checked={checked} aria-label={title} aria-describedby={detailId} onChange={onChange} className="sr-only" />
  </label>;
}

function HudPreview({ theme, portrait, overlays }: {
  theme?: CustomHudTheme; portrait?: FullDemoAssetRef | null; overlays: FullDemoOptions['overlays'];
}): ReactNode {
  const hasPortrait = theme?.id === 'focus' && Boolean(portrait);
  // The drawn preview is the video's own renderer; it has no portrait, so Focus with one keeps the catalog picture.
  const drawn = theme && !hasPortrait && needsDrawnPreview(overlays.hud_custom, overlays.hud_options, overlays.team_names)
    ? hudPreviewUrl({ base: theme.id, style: overlays.hud_custom, options: overlays.hud_options, names: overlays.team_names }) : null;
  const name = overlays.hud_custom?.name ?? theme?.name;
  return <div className="relative isolate aspect-video w-full overflow-hidden bg-surface-0">
    <PreviewBackdrop />
    {theme ? <img src={drawn ?? `/hud/${theme.id}${hasPortrait ? '-portrait' : ''}.webp`} alt={`Vista previa del HUD ${name}`} className="absolute inset-0 size-full object-contain" width={1920} height={1080} /> : <NativeHudArt />}
    {hasPortrait && portrait ? <img src={`/api/full-demo/overlay-images/${portrait.id}`} alt="Retrato del jugador" className="absolute object-contain object-bottom" style={{ left: `${654 / 1920 * 100}%`, top: `${898 / 1080 * 100}%`, width: `${116 / 1920 * 100}%`, height: `${116 / 1080 * 100}%` }} /> : null}
  </div>;
}

/** The map the HUD and intro pictures sit on, in the large preview and behind each design of a gallery. */
export function PreviewBackdrop(): ReactNode {
  return <>
    <MapCover map={PREVIEW_MAP} className="absolute inset-0 opacity-70" />
    <span className="absolute inset-0"><CoverImage src={mapShotUrl(PREVIEW_MAP)} className="brightness-[0.8]" /></span>
  </>;
}

/** The element that scrolls the page around `element`: the app shell's main pane, else the document. */
function scrollParent(element: HTMLElement): Element {
  for (let node = element.parentElement; node; node = node.parentElement) {
    const { overflowY } = getComputedStyle(node);
    if ((overflowY === 'auto' || overflowY === 'scroll') && node.scrollHeight > node.clientHeight) return node;
  }
  return document.scrollingElement ?? document.documentElement;
}
