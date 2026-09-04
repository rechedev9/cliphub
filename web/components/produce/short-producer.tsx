'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { ChevronRight, Sparkles } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { EditConfig, Match, Play, Preset } from '@/lib/api/types';
import { DEFAULT_EDIT_CONFIG } from '@/lib/api/reel-store';
import { GAME_VOLUME_DEFAULT_PERCENT } from '@/lib/api/reel-music';
import { hubHref, seriesHref } from '@/lib/clips/routes';
import { forgeHint } from '@/lib/forge-hint';
import { PRODUCE_SHORT_EMPTY_HINT, PRODUCE_SHORT_TITLE } from '@/lib/produce/copy';
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
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { EditOptions } from '@/components/clips/edit-options';
import { PlayList } from '@/components/clips/play-list';
import { SongPickerDialog } from '@/components/clips/song-picker-dialog';
import { MusicCard, MUSIC_VOLUME } from './music-card';
import { ProduceFooter } from './produce-footer';
import { ShortStoryboard } from './short-storyboard';

const SHORT_FORMAT: EditConfig['format'] = 'short-9x16';
const CLOCK_TARGET = formatClock(SHORT_TARGET_SECONDS);
const AUTO_PICK_LABEL = 'Auto: mejores 60 s';

/** Step eyebrows: the list is 01, the aside walks 02 → 04, the footer brief closes. */
const STEP = {
  highlights: '01 · Highlights detectados',
  preset: '02 · Preset',
  music: '03 · Música',
  overlays: '04 · Overlays',
} as const;

export type ShortProducerProps = {
  matchId: string;
  match: Match;
  plays: Play[];
  /** From a series map card: a finished render returns to the series. */
  seriesId: string | null;
};

/** The Short constructor: the best minute is preselected, then preset, music and overlays, approve, render. */
export function ShortProducer({ matchId, match, plays, seriesId }: ShortProducerProps): ReactNode {
  const router = useRouter();
  const [presets, setPresets] = useState<Preset[] | null>(null);
  // A first-time user lands on a renderable plan; every row stays a toggle.
  const [selectedIds, setSelectedIds] = useState<ReadonlySet<string>>(() => autoPickBestPlays(plays));
  const [variant, setVariant] = useState<string | null>(null);
  const [songId, setSongId] = useState<string | null>(null);
  const [songTitle, setSongTitle] = useState<string | null>(null);
  const [musicDecided, setMusicDecided] = useState(false);
  const [musicVolume, setMusicVolume] = useState<number>(MUSIC_VOLUME.default);
  const [gameVolume, setGameVolume] = useState<number>(GAME_VOLUME_DEFAULT_PERCENT);
  // The Short constructor is single-format: `SHORT_FORMAT` is the only source
  // of truth, so the edit config can never drift to 16:9 behind the preview.
  const [editConfig, setEditConfig] = useState<EditConfig>(() =>
    constrainEditConfig({ ...DEFAULT_EDIT_CONFIG, format: SHORT_FORMAT }),
  );
  const [songOpen, setSongOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [briefApproved, setBriefApproved] = useState(false);

  // Any decision change invalidates the approval: the checkbox answers the shown brief.
  useEffect(() => {
    setBriefApproved(false);
  }, [selectedIds, variant, songId, musicDecided, musicVolume, gameVolume, editConfig]);

  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const list = await api.listPresets();
        if (!active) return;
        setPresets(list);
        setVariant((cur) => selectShortsFormat(SHORT_FORMAT, cur, list).variant);
      } catch {
        if (active) setPresets([]);
      }
    })();
    return () => {
      active = false;
    };
  }, []);

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
  const configured = selectedPlays.length > 0 && presetLabel !== null && musicDecided;
  const ready = canForgeReel({
    briefApproved,
    creating,
    hasPreset: variant !== null,
    selectionCount: selectedPlays.length,
    musicDecided,
  });

  function toggleSelect(playId: string): void {
    if (creating) return;
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(playId)) next.delete(playId);
      else next.add(playId);
      return next;
    });
  }

  function changeEditConfig(next: EditConfig): void {
    setEditConfig(constrainEditConfig({ ...next, format: SHORT_FORMAT }));
  }

  function chooseVariant(nextVariant: string): void {
    // A landscape preset is never offered here, so only the variant travels.
    setVariant(selectShortsPreset(nextVariant, SHORT_FORMAT, presets ?? []).variant);
  }

  function onChooseSong(chosenId: string, chosenTitle: string): void {
    setSongId(chosenId);
    setSongTitle(chosenTitle);
    setMusicDecided(true);
    setSongOpen(false);
  }

  function resetMusic(decided: boolean): void {
    setSongId(null);
    setSongTitle(null);
    setMusicVolume(MUSIC_VOLUME.default);
    setGameVolume(GAME_VOLUME_DEFAULT_PERCENT);
    setMusicDecided(decided);
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
      toast('Short en render', { description: 'FFmpeg montando · míralo en la fila' });
      router.push(seriesId ? seriesHref(seriesId) : hubHref({ open: matchId }));
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'No se pudo crear el short.');
      setCreating(false);
    }
  }

  const summary = configured ? (
    <>
      {roundsSummary(selectedPlays)}
      <span className="text-fg-3"> · </span>
      <span className={overTarget ? 'text-warning' : undefined}>{formatClock(estimatedSeconds)} aprox.</span>
      <span className="text-fg-3"> · </span>
      <span className="text-primary">{presetLabel}</span>
      <span className="text-fg-3"> · </span>
      {songTitle ? `♪ ${songTitle}` : 'sin música'}
    </>
  ) : null;

  return (
    <>
      <div className="grid items-start gap-6 @[56rem]/content:grid-cols-[minmax(0,1fr)_320px]">
        <section className="flex min-w-0 flex-col gap-3.5">
          <div className="flex flex-col gap-1.5">
            <p className="font-mono text-meta uppercase tracking-ultra text-fg-3">
              Nuevo short · {match.map}
              {match.player ? ` · ${match.player}` : ''}
            </p>
            <h1 className="font-display text-display-sm font-bold uppercase text-fg-1">{PRODUCE_SHORT_TITLE}</h1>
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
              disabled={creating || plays.length === 0}
              onClick={() => setSelectedIds(autoPickBestPlays(plays))}
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
            onSelectAll={() => !creating && setSelectedIds(new Set(plays.map((play) => play.id)))}
            onClear={() => !creating && setSelectedIds(new Set())}
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
                No se pudieron cargar los presets. Recarga la página.
              </p>
            ) : (
              <Select value={variant ?? undefined} onValueChange={chooseVariant} disabled={creating || visiblePresets === null}>
                <SelectTrigger id="short-preset" className="h-10 font-display font-semibold uppercase">
                  <SelectValue placeholder="Cargando presets…" />
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

          <MusicCard
            eyebrow={STEP.music}
            decided={musicDecided}
            songTitle={songTitle}
            musicVolume={musicVolume}
            gameVolume={gameVolume}
            busy={creating}
            onOpenPicker={() => setSongOpen(true)}
            onChooseNone={() => resetMusic(true)}
            onClear={() => resetMusic(false)}
            onVolumeChange={setMusicVolume}
            onGameVolumeChange={setGameVolume}
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
                disabled={creating}
              />
            </div>
          </details>
        </aside>
      </div>

      <div className="flex-1" />

      <ProduceFooter
        tone="short"
        eyebrow="Short"
        summary={summary}
        hint={
          selectedPlays.length === 0
            ? PRODUCE_SHORT_EMPTY_HINT
            : forgeHint(roundsSummary(selectedPlays), presetLabel)
        }
        briefItems={briefItems}
        briefApproved={briefApproved}
        briefReady={configured}
        onBriefApprovedChange={setBriefApproved}
        backHref={seriesId ? seriesHref(seriesId) : hubHref({ open: matchId })}
        busy={creating}
        error={createError}
        cta={
          <Button
            variant="hero"
            size="lg"
            disabled={!ready}
            loading={creating}
            loadingText="Clipeando…"
            onClick={() => void onCreate()}
            className="neon-notch shrink-0 focus-visible:-outline-offset-4"
          >
            Clipear short →
          </Button>
        }
      />

      <SongPickerDialog open={songOpen} onOpenChange={setSongOpen} onChoose={onChooseSong} selectedSongId={songId} />
    </>
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
