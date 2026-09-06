'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { ChevronRight, Sparkles } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { EditConfig, Match, Play, Preset } from '@/lib/api/types';
import { GAME_VOLUME_DEFAULT_PERCENT } from '@/lib/api/reel-music';
import { hubHref, seriesHref } from '@/lib/clips/routes';
import { forgeHint } from '@/lib/forge-hint';
import { PRODUCE_SHORT_DRAFT_RESET, PRODUCE_SHORT_DRAFT_RESTORED, PRODUCE_SHORT_EMPTY_HINT, PRODUCE_SHORT_TITLE } from '@/lib/produce/copy';
import { defaultShortSettings } from '@/lib/produce/short-draft';
import { useShortDraft } from '@/hooks/use-short-draft';
import {
  autoPickBestPlays,
  estimatedSelectionSeconds,
  formatClock,
  roundsSummary,
  selectionTimeline,
  SHORT_TARGET_SECONDS,
} from '@/lib/produce/short-selection';
import { canForgeReel, constrainEditConfig, reelCreativeBrief, type MusicBrief } from '@/lib/reel-brief';
import { selectShortsFormat, selectShortsPreset, shortsPresetsForFormat } from '@/lib/reel-format';
import { cn } from '@/lib/utils';
import { presetDescription } from '@/lib/preset-copy';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { EditOptions } from '@/components/clips/edit-options';
import { PlayList } from '@/components/clips/play-list';
import { SongPickerDialog } from '@/components/clips/song-picker-dialog';
import { MusicCard, MUSIC_VOLUME } from './music-card';
import { ProduceFooter } from './produce-footer';
import { ShortStoryboard } from './short-storyboard';
import { PresetPreview } from './preset-preview';

const SHORT_FORMAT: EditConfig['format'] = 'short-9x16';
const CLOCK_TARGET = formatClock(SHORT_TARGET_SECONDS);
const AUTO_PICK_LABEL = 'Auto: mejores 60 s';

/** Step eyebrows: the list is 01, the aside walks 02 → 04, the footer brief closes. */
const STEP = {
  highlights: '01 · Elige las jugadas',
  preset: '02 · Estilo del vídeo',
  music: '03 · Música',
  overlays: '04 · Textos y gráficos',
} as const;

export type ShortProducerProps = {
  matchId: string;
  match: Match;
  plays: Play[];
  /** From a series map card: a finished render returns to the series. */
  seriesId: string | null;
};

/** The Short constructor: the best minute is preselected, then preset, music and overlays, render. */
export function ShortProducer({ matchId, match, plays, seriesId }: ShortProducerProps): ReactNode {
  const router = useRouter();
  const [presets, setPresets] = useState<Preset[] | null>(null);
  const { settings, updateSettings, resetSettings, discardDraft, loaded, restored } = useShortDraft(matchId, plays);
  const { variant, songId, songTitle, musicDecided, musicVolume, gameVolume, editConfig } = settings;
  const selectedIds = useMemo(() => new Set(settings.selectedIds), [settings.selectedIds]);
  const [songOpen, setSongOpen] = useState(false);
  const [stylesOpen, setStylesOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const busy = creating || !loaded;

  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const list = await api.listPresets();
        if (!active) return;
        setPresets(list);
        // An unavailable catalog must not overwrite a saved preset with null.
        if (list.length > 0) updateSettings((cur) => ({ variant: selectShortsFormat(SHORT_FORMAT, cur.variant, list).variant }), false);
      } catch {
        if (active) setPresets([]);
      }
    })();
    return () => {
      active = false;
    };
  }, [updateSettings]);

  const selectedPlays = plays.filter((play) => selectedIds.has(play.id));
  const cues = selectionTimeline(plays, selectedIds);
  const estimatedSeconds = estimatedSelectionSeconds(selectedPlays);
  const overTarget = estimatedSeconds > SHORT_TARGET_SECONDS;
  // No tilde: under uppercase mono it reads as a minus sign ("-0:00").
  const clock = `${formatClock(estimatedSeconds)} / ${CLOCK_TARGET} aprox.`;
  const visiblePresets = presets === null ? null : shortsPresetsForFormat(presets, SHORT_FORMAT);
  const selectedPreset = visiblePresets?.find((preset) => preset.name === variant) ?? null;
  const presetLabel = selectedPreset?.label ?? null;
  const briefItems = reelCreativeBrief(editConfig, selectedPreset, musicBriefFor(musicDecided, songTitle, musicVolume, gameVolume));
  const ready = canForgeReel({
    creating: busy,
    hasPreset: selectedPreset !== null,
    selectionCount: selectedPlays.length,
    musicDecided,
  });

  function toggleSelect(playId: string): void {
    if (busy) return;
    updateSettings((prev) => {
      const next = new Set(prev.selectedIds);
      if (next.has(playId)) next.delete(playId);
      else next.add(playId);
      return { selectedIds: [...next] };
    });
  }

  function changeEditConfig(next: EditConfig): void {
    if (busy) return;
    updateSettings({ editConfig: constrainEditConfig({ ...next, format: SHORT_FORMAT }) });
  }

  function chooseVariant(nextVariant: string): void {
    // A landscape preset is never offered here, so only the variant travels.
    if (busy) return;
    updateSettings({ variant: selectShortsPreset(nextVariant, SHORT_FORMAT, presets ?? []).variant });
  }

  function onChooseSong(chosenId: string, chosenTitle: string): void {
    if (busy) return;
    updateSettings({ songId: chosenId, songTitle: chosenTitle, musicDecided: true });
    setSongOpen(false);
  }

  function resetMusic(decided: boolean): void {
    if (busy) return;
    updateSettings({ songId: null, songTitle: null, musicVolume: MUSIC_VOLUME.default, gameVolume: GAME_VOLUME_DEFAULT_PERCENT, musicDecided: decided });
  }

  function startOver(): void {
    if (busy) return;
    resetSettings({ ...defaultShortSettings(plays), variant: selectShortsFormat(SHORT_FORMAT, null, presets ?? []).variant });
    setCreateError(null);
    setSongOpen(false);
  }

  async function onCreate(): Promise<void> {
    if (!ready) return;
    setCreating(true);
    setCreateError(null);
    try {
      await api.createVideo({
        matchId,
        playIds: selectedPlays.map((play) => play.id),
        mode: songId ? 'music' : 'clean',
        songId: songId ?? undefined,
        // Only a reduced volume travels; full volume stays the legacy default.
        musicVolume: songId && musicVolume < MUSIC_VOLUME.max ? musicVolume / 100 : undefined,
        gameVolume: songId ? gameVolume / 100 : undefined,
        variant: variant ?? undefined,
        editConfig: constrainEditConfig({ ...editConfig, format: SHORT_FORMAT }),
      });
      discardDraft();
      toast('Short en preparación', { description: 'Sigue la grabación y el montaje en Clips y vídeos.' });
      router.push(seriesId ? seriesHref(seriesId) : hubHref({ open: matchId }));
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'No se pudo crear el short.');
      setCreating(false);
    }
  }

  const musicSummary = songTitle ? `♪ ${songTitle}` : 'Sin música';
  const summary = selectedPlays.length > 0 ? (
    <>
      {roundsSummary(selectedPlays)}
      <span className="text-fg-3"> · </span>
      <span className={overTarget ? 'text-warning' : undefined}>{formatClock(estimatedSeconds)} aprox.</span>
      <span className="text-fg-3"> · </span>
      <span className="text-primary">{presetLabel ?? 'Estilo pendiente'}</span>
      <span className="text-fg-3"> · </span>
      {musicDecided ? musicSummary : 'Música pendiente'}
    </>
  ) : null;

  return (
    <Dialog open={stylesOpen} onOpenChange={setStylesOpen}>
      <div className="grid items-start gap-6 @[56rem]/content:grid-cols-[minmax(0,1fr)_320px]">
        <section className="flex min-w-0 flex-col gap-3.5">
          <div className="flex flex-col gap-1.5">
            <p className="font-mono text-meta uppercase tracking-ultra text-fg-3">
              Nuevo short · {match.map}
              {match.player ? ` · ${match.player}` : ''}
            </p>
            <h1 className="font-display text-display-sm font-bold uppercase text-fg-1">{PRODUCE_SHORT_TITLE}</h1>
            <p className="text-body-sm text-fg-2">Las jugadas seleccionadas se unen en un único vídeo vertical. Revisa la selección automática antes de continuar.</p>
            {restored ? (
              <p role="status" className="text-body-sm text-fg-3">
                {PRODUCE_SHORT_DRAFT_RESTORED}{' '}
                <button type="button" disabled={busy} onClick={startOver} className="text-primary underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50">
                  {PRODUCE_SHORT_DRAFT_RESET}
                </button>
              </p>
            ) : null}
            <p className="measure-read text-body text-fg-2">
              Ya tienes preseleccionado el mejor minuto. Toca una fila para quitarla o añadirla; el guion enseña el orden
              final.
            </p>
          </div>

          <div className="flex">
            <Button
              type="button"
              size="xs"
              variant="outline-primary"
              disabled={busy || plays.length === 0}
              onClick={() => updateSettings({ selectedIds: [...autoPickBestPlays(plays)] })}
            >
              <Sparkles aria-hidden />
              {AUTO_PICK_LABEL}
            </Button>
          </div>

          <PlayList
            plays={plays}
            selectedIds={selectedIds}
            title={STEP.highlights}
            counter={
              <span className={cn('tabular-nums', overTarget ? 'text-warning' : 'text-primary')}>
                {selectedPlays.length} de {plays.length} elegidos · {clock}
              </span>
            }
            onToggle={toggleSelect}
            onSelectAll={() => !busy && updateSettings({ selectedIds: plays.map((play) => play.id) })}
            onClear={() => !busy && updateSettings({ selectedIds: [] })}
          />
        </section>

        <aside className="flex flex-col gap-3 @[56rem]/content:sticky @[56rem]/content:top-20">
          <ShortStoryboard cues={cues} totalSeconds={estimatedSeconds} />

          <div className="studio-panel flex flex-col gap-2.5 px-3.5 py-3">
            <label htmlFor="short-preset" className="font-mono text-meta uppercase tracking-ultra text-fg-3">
              {STEP.preset}
            </label>
            {visiblePresets !== null && visiblePresets.length === 0 ? (
              <p role="alert" className="text-body-sm text-fg-2">
                No se pudieron cargar los estilos. Recarga la página.
              </p>
            ) : (
              <Select value={variant ?? undefined} onValueChange={chooseVariant} disabled={busy || visiblePresets === null}>
                <SelectTrigger id="short-preset" className="h-10 font-display font-semibold uppercase">
                  <SelectValue placeholder="Cargando estilos…" />
                </SelectTrigger>
                <SelectContent>
                  {(visiblePresets ?? []).map((preset) => (
                    <SelectItem key={preset.name} value={preset.name}>
                      {preset.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>

          {selectedPreset ? (
            <div className="studio-panel space-y-3 p-3.5">
              <div className="flex items-start gap-3">
                <PresetPreview preset={selectedPreset} className="w-20" />
                <div className="min-w-0 space-y-2">
                  <p className="text-body-sm text-fg-1">{presetDescription(selectedPreset)}</p>
                  <p className="text-label text-fg-2">Vista orientativa del estilo.</p>
                </div>
              </div>
              <DialogTrigger asChild>
                <Button type="button" variant="outline" size="sm" className="w-full" disabled={busy}>Comparar estilos</Button>
              </DialogTrigger>
            </div>
          ) : null}

          <MusicCard
            eyebrow={STEP.music}
            decided={musicDecided}
            songTitle={songTitle}
            musicVolume={musicVolume}
            gameVolume={gameVolume}
            busy={busy}
            onOpenPicker={() => setSongOpen(true)}
            onChooseNone={() => resetMusic(true)}
            onClear={() => resetMusic(false)}
            onVolumeChange={(value) => updateSettings({ musicVolume: value })}
            onGameVolumeChange={(value) => updateSettings({ gameVolume: value })}
          />

          <details className="group/overlays studio-panel px-3.5 py-3">
            <summary className="flex cursor-pointer list-none items-center justify-between gap-2 [&::-webkit-details-marker]:hidden">
              <span className="font-mono text-meta uppercase tracking-ultra text-fg-3">{STEP.overlays}</span>
              <span className="flex items-center gap-1.5 font-display text-body-sm font-semibold uppercase text-fg-1">
                {overlaysSummary(editConfig)}
                <ChevronRight aria-hidden className="size-4 text-primary transition-transform duration-(--dur-fast) group-open/overlays:rotate-90" />
              </span>
            </summary>
            <div className="mt-3 border-t border-border-subtle pt-3">
              <EditOptions
                value={editConfig}
                onChange={changeEditConfig}
                disabled={busy}
              />
            </div>
          </details>
        </aside>
      </div>

      <div className="flex-1" />

      <ProduceFooter
        ready={ready}
        busy={creating}
        tone="short"
        eyebrow="Short"
        summary={summary}
        hint={
          selectedPlays.length === 0
            ? PRODUCE_SHORT_EMPTY_HINT
            : forgeHint(roundsSummary(selectedPlays), presetLabel)
        }
        briefItems={briefItems}
        backHref={seriesId ? seriesHref(seriesId) : hubHref({ open: matchId })}
        error={createError}
        cta={
          <Button
            variant="hero"
            size="lg"
            disabled={!ready}
            loading={creating}
            loadingText="Preparando grabación…"
            onClick={() => void onCreate()}
            className="neon-notch shrink-0 focus-visible:-outline-offset-4"
          >
            Crear Short
          </Button>
        }
      />

      <SongPickerDialog open={songOpen} onOpenChange={setSongOpen} onChoose={onChooseSong} selectedSongId={songId} />
      <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>Compara los estilos de tu Short</DialogTitle>
          <DialogDescription>Ejemplos orientativos del HUD y el color. El vídeo final utiliza tus jugadas.</DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,180px),1fr))] gap-3">
          {(visiblePresets ?? []).map((preset) => (
            <button key={preset.name} type="button" disabled={busy} aria-pressed={variant === preset.name}
              onClick={() => { chooseVariant(preset.name); setStylesOpen(false); }}
              className={cn('flex flex-col items-start gap-3 rounded-lg border p-3 text-left focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50', variant === preset.name ? 'border-primary bg-primary/10' : 'border-border-strong hover:bg-surface-3')}>
              <PresetPreview preset={preset} className="mx-auto w-24" />
              <span className="font-display text-body font-semibold text-fg-1">{preset.label}</span>
              <span className="text-body-sm text-fg-2">{presetDescription(preset)}</span>
              {variant === preset.name ? <span className="mt-auto text-body-sm text-primary">Seleccionado</span> : null}
            </button>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function musicBriefFor(decided: boolean, songTitle: string | null, volumePercent: number, gameVolumePercent: number): MusicBrief {
  if (!decided) return { status: 'pending' };
  if (songTitle) return { status: 'track', title: songTitle, volumePercent, gameVolumePercent };
  return { status: 'none' };
}

function overlaysSummary(edit: EditConfig): string {
  const parts: string[] = [];
  if (edit.killCounter) parts.push('Kill counter');
  if (edit.hookText) parts.push('Título');
  if (edit.intro || edit.outro) parts.push('Intro / outro');
  if (edit.keyDropStyle) parts.push('Afiliado');
  return parts.length > 0 ? parts.join(' + ') : 'Solo efectos';
}
