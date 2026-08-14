'use client';

import { use, useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { Music, SearchX, Unplug } from 'lucide-react';
import type { EditConfig, Match, Play, Preset } from '@/lib/api/types';
import { SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import { api } from '@/lib/api';
import { DEFAULT_EDIT_CONFIG } from '@/lib/api/reel-store';
import { isSeriesId } from '@/lib/series-status';
import { formatKd, matchDateLabel, playsSelectionLabel, ratingClass } from '@/lib/format';
import { canForgeReel, reelCreativeBrief, type MusicBrief } from '@/lib/reel-brief';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { ScoreBar } from '@/components/brand/score-bar';
import { StatMono } from '@/components/brand/stat-mono';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { isWin, MatchScore } from '@/components/matches/match-score';
import { StudioBackLink } from '@/components/studio/back-link';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { PlayList } from '@/components/clips/play-list';
import { PresetCards } from '@/components/clips/preset-cards';
import { CreateReelBar } from '@/components/clips/create-reel-bar';
import { EditOptions } from '@/components/clips/edit-options';
import { SongPickerDialog } from '@/components/clips/song-picker-dialog';

function isServiceUnavailable(err: unknown): boolean {
  return (err as { code?: string } | null)?.code === SERVICE_UNAVAILABLE_CODE;
}

// Music volume slider, in UI percent. Default 100 renders at full volume (the
// legacy byte-identical form); only < 100 sends a reduced volume to the render.
const VOLUME_MIN = 5;
const VOLUME_MAX = 100;
const VOLUME_STEP = 5;
const VOLUME_DEFAULT = 100;

export default function FindHighlightsPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ series?: string | string[] }>;
}) {
  const { id } = use(params);
  // Set when the picker was entered from a series map card: creating a reel
  // then returns to the series so the user can queue the remaining maps,
  // instead of dead-ending in the Library with the rest of the series lost.
  const { series } = use(searchParams);
  const seriesId = typeof series === 'string' && isSeriesId(series) ? series : null;
  const router = useRouter();

  const [match, setMatch] = useState<Match | null>(null);
  const [plays, setPlays] = useState<Play[] | null>(null);
  const [loaded, setLoaded] = useState(false);
  /** null = ok/not-loaded; offline = 503; error = any other failure. */
  const [loadFailure, setLoadFailure] = useState<'offline' | 'error' | null>(null);

  const [presets, setPresets] = useState<Preset[] | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [variant, setVariant] = useState<string | null>(null);
  const [songId, setSongId] = useState<string | null>(null);
  const [songTitle, setSongTitle] = useState<string | null>(null);
  const [musicDecided, setMusicDecided] = useState(false);
  const [musicVolume, setMusicVolume] = useState<number>(VOLUME_DEFAULT);
  const [editConfig, setEditConfig] = useState<EditConfig>(DEFAULT_EDIT_CONFIG);
  const [songOpen, setSongOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [briefApproved, setBriefApproved] = useState(false);

  useEffect(() => {
    setBriefApproved(false);
  }, [selectedIds, variant, songId, musicDecided, musicVolume, editConfig]);

  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const [m, p] = await Promise.all([api.getMatch(id), api.findClips(id)]);
        if (!active) return;
        setMatch(m);
        setPlays(p);
        setLoadFailure(null);
      } catch (err) {
        // 503 service_unavailable means the orchestrator is down; any other
        // failure (404, 5xx, network) is a load error — not necessarily offline.
        if (!active) return;
        setMatch(null);
        setPlays([]);
        setLoadFailure(isServiceUnavailable(err) ? 'offline' : 'error');
      } finally {
        if (active) setLoaded(true);
      }
    })();
    return () => {
      active = false;
    };
  }, [id]);

  // Load the reel presets and default to the registry's default (first) preset.
  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const list = await api.listPresets();
        if (!active) return;
        setPresets(list);
        setVariant((cur) => cur ?? (list.find((p) => p.default)?.name ?? list[0]?.name ?? null));
      } catch {
        if (active) setPresets([]);
      }
    })();
    return () => {
      active = false;
    };
  }, []);

  // Plan order (the order plays appear in the list), not click order — the
  // Set only tracks membership, so the source of truth for order is always
  // the filter below.
  const selectedPlays = (plays ?? []).filter((p) => selectedIds.has(p.id));
  const selectionLabel = playsSelectionLabel(selectedPlays);
  const selectedPreset = presets?.find((p) => p.name === variant) ?? null;
  const presetLabel = selectedPreset?.label ?? null;
  const briefItems = reelCreativeBrief(
    editConfig,
    selectedPreset,
    musicBriefFor(musicDecided, songTitle, musicVolume),
  );
  const busy = creating;

  function revokeBriefApproval() {
    setBriefApproved(false);
  }

  function toggleSelect(playId: string) {
    if (busy) return;
    revokeBriefApproval();
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(playId)) next.delete(playId);
      else next.add(playId);
      return next;
    });
  }

  function selectAll() {
    if (busy || !plays) return;
    revokeBriefApproval();
    setSelectedIds(new Set(plays.map((p) => p.id)));
  }

  function clearSelection() {
    if (busy) return;
    revokeBriefApproval();
    setSelectedIds(new Set());
  }

  function chooseVariant(nextVariant: string) {
    revokeBriefApproval();
    setVariant(nextVariant);
  }

  function changeEditConfig(nextConfig: EditConfig) {
    revokeBriefApproval();
    setEditConfig(nextConfig);
  }

  function changeMusicVolume(nextVolume: number) {
    revokeBriefApproval();
    setMusicVolume(nextVolume);
  }

  async function onCreate() {
    if (!canForgeReel({ briefApproved, creating: busy, hasPreset: variant !== null, selectionCount: selectedPlays.length, musicDecided })) return;
    setCreating(true);
    setCreateError(null);
    try {
      await api.createVideo({
        matchId: id,
        playIds: selectedPlays.map((p) => p.id),
        mode: songId ? 'music' : 'clean',
        songId: songId ?? undefined,
        // Only a reduced volume travels; full volume stays the legacy default.
        musicVolume: songId && musicVolume < VOLUME_MAX ? musicVolume / 100 : undefined,
        variant: variant ?? undefined,
        editConfig,
      });
      router.push(seriesId ? `/series/${seriesId}` : '/videos');
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'No se pudo crear el reel.');
      setCreating(false);
    }
  }

  function onChooseSong(chosenId: string, chosenTitle: string) {
    revokeBriefApproval();
    setSongId(chosenId);
    setSongTitle(chosenTitle);
    setMusicDecided(true);
    setSongOpen(false);
  }

  function chooseNoMusic() {
    revokeBriefApproval();
    setSongId(null);
    setSongTitle(null);
    setMusicVolume(VOLUME_DEFAULT);
    setMusicDecided(true);
  }

  function clearMusicDecision() {
    revokeBriefApproval();
    setSongId(null);
    setSongTitle(null);
    setMusicVolume(VOLUME_DEFAULT);
    setMusicDecided(false);
  }

  if (!loaded) {
    return <LoadingState />;
  }

  if (!match) {
    const offline = loadFailure === 'offline';
    const errored = loadFailure === 'error';
    let emptyTitle = 'Partida no encontrada';
    let emptyDescription =
      'Esta partida ya no está en tu biblioteca local. Puede que se haya borrado con sus artefactos.';
    if (offline) {
      emptyTitle = 'Servicio local sin conexión';
      emptyDescription =
        'El servicio local de TickCut no respondió. Arráncalo y vuelve a intentarlo.';
    } else if (errored) {
      emptyTitle = 'No se pudo cargar la partida';
      emptyDescription =
        'Hubo un error al leer esta partida. Vuelve a intentarlo o regresa a la lista.';
    }
    return (
      <div className="flex flex-col gap-8">
        <StudioBackLink href="/matches">PARTIDAS</StudioBackLink>
        <StudioEmptyState
          icon={offline || errored ? Unplug : SearchX}
          title={emptyTitle}
          description={emptyDescription}
          actions={
            <Button onClick={() => router.push('/matches')}>VOLVER A PARTIDAS</Button>
          }
        />
      </div>
    );
  }

  const playList = plays ?? [];
  const n = playList.length;
  const win = isWin(match.score);
  // Uploaded demos have no round score (the parser computes none): hide the
  // score block and let the mono meta line carry the play count instead.
  const hasScore = match.score.trim() !== '';
  const fromUpload = match.source === 'upload';
  let backHref = fromUpload ? '/upload' : '/matches';
  let backLabel = fromUpload ? 'SUBIR DEMO' : 'PARTIDAS';
  if (seriesId) {
    backHref = `/series/${seriesId}`;
    backLabel = 'SERIE';
  }
  const meta = [
    matchDateLabel(match),
    `${n} ${n === 1 ? 'jugada' : 'jugadas'}`,
  ].join(' · ');

  // Scoreboard extras exist only on enriched (uploaded) matches; mock/seed
  // matches show the classic K/D/A line.
  const { rating = 0, adr, kast, hsPct } = match.stats;
  const hasRating = rating > 0;

  let presetContent: ReactNode;
  if (presets === null) {
    presetContent = (
      <div className="grid gap-4 @[30rem]/build:grid-cols-2">
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} className="h-40" />
        ))}
      </div>
    );
  } else if (presets.length === 0) {
    presetContent = (
      <p role="alert" className="studio-panel px-5 py-6 text-center text-body-sm text-fg-2">
        No se pudieron cargar los presets. Recarga la página para reintentar.
      </p>
    );
  } else {
    presetContent = (
      <PresetCards
        presets={presets}
        value={variant}
        onChange={chooseVariant}
        disabled={selectedIds.size === 0 || busy}
      />
    );
  }

  return (
    <div className="flex min-h-[calc(100vh-9rem)] flex-col gap-7">
      <StudioBackLink onClick={() => router.push(backHref)}>{backLabel}</StudioBackLink>

      {/* Match summary — accent bar + map title + mono meta, score, stat strip. */}
      <section className="flex flex-col gap-5 @[52rem]/content:flex-row @[52rem]/content:items-center @[52rem]/content:justify-between @[52rem]/content:gap-8">
        <div className="flex items-center gap-5">
          <ScoreBar win={win} className="h-14 w-[3px]" />
          <div className="flex flex-col gap-1.5">
            <h1 className="font-display text-section font-bold uppercase text-fg-1 @[40rem]/content:text-display-sm">
              {match.map}
            </h1>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
              <span className="font-mono text-meta uppercase tracking-wider text-fg-3">{meta}</span>
              {hasRating ? (
                <span className="inline-flex items-baseline gap-1.5 border border-border-strong bg-surface-3 px-2 py-0.5">
                  <span className={cn('font-mono text-body-sm font-semibold tabular-nums', ratingClass(rating))}>
                    {rating.toFixed(2)}
                  </span>
                  <span className="font-mono text-meta uppercase tracking-wider text-fg-3">rating</span>
                </span>
              ) : null}
            </div>
          </div>
          {hasScore ? <MatchScore score={match.score} className="ml-2" /> : null}
        </div>

        <div className="grid grid-cols-4 gap-x-5 gap-y-3 @[40rem]/content:flex @[40rem]/content:flex-wrap @[40rem]/content:items-center @[40rem]/content:gap-x-7">
          <StatMono label="K" value={match.stats.kills} />
          <StatMono label="D" value={match.stats.deaths} />
          <StatMono label="A" value={match.stats.assists} />
          {adr !== undefined ? <StatMono label="ADR" value={Math.round(adr)} /> : null}
          {kast !== undefined ? <StatMono label="KAST" value={`${Math.round(kast)}%`} /> : null}
          {hsPct !== undefined ? <StatMono label="HS" value={`${Math.round(hsPct)}%`} /> : null}
          {match.stats.mvps > 0 ? <StatMono label="MVP" value={match.stats.mvps} /> : null}
          <StatMono label="K/D" value={formatKd(match.stats.kd)} accent />
        </div>
      </section>

      {n === 0 ? (
        <StudioEmptyState
          icon={SearchX}
          title="Sin jugadas destacables"
          description="El análisis no encontró ninguna jugada digna de highlight en esta partida. Prueba con otra demo."
          compact
          actions={<Button onClick={() => router.push(backHref)}>VOLVER A {backLabel}</Button>}
        />
      ) : (
        /*
          Two panes keyed to the content container: the vertical selector on the
          left and the reel build column sticky on the right, so the choices that
          define the output stay visible while the list scrolls. Below the
          threshold the build column simply stacks under the list.
        */
        <div className="grid items-start gap-7 @[64rem]/content:grid-cols-[minmax(0,1.55fr)_minmax(21rem,0.85fr)]">
          <section className="flex flex-col gap-4">
            <div className="flex flex-col gap-1">
              <h2 className="font-mono text-label uppercase tracking-ultra text-primary">
                JUGADAS DETECTADAS{' '}
                <span className="tracking-wider text-fg-3">
                  · <span className="tabular-nums">{n}</span>
                </span>
              </h2>
              <p className="text-body-sm text-fg-2">
                Elige las jugadas que quieras forjar en un reel; 2 o más se concatenan en uno.
              </p>
            </div>

            <PlayList
              plays={playList}
              selectedIds={selectedIds}
              onToggle={toggleSelect}
              onSelectAll={selectAll}
              onClear={clearSelection}
            />
          </section>

          <div className="@container/build flex flex-col gap-6 @[64rem]/content:sticky @[64rem]/content:top-20">
            <section className="flex flex-col gap-3">
              <SectionEyebrow label="PRESET DEL REEL" />
              {presetContent}
            </section>

            <section className="flex flex-col gap-3">
              <SectionEyebrow label="MÚSICA" />
              <MusicDecisionPanel
                decided={musicDecided}
                songTitle={songTitle}
                musicVolume={musicVolume}
                busy={busy}
                onOpenPicker={() => setSongOpen(true)}
                onChooseNone={chooseNoMusic}
                onClear={clearMusicDecision}
                onVolumeChange={changeMusicVolume}
              />
            </section>

            <section className="flex flex-col gap-3">
              <SectionEyebrow label="OPCIONES DE EDICIÓN" />
              <EditOptions value={editConfig} onChange={changeEditConfig} disabled={selectedIds.size === 0 || busy} />
            </section>
          </div>
        </div>
      )}

      <div className="flex-1" />

      {createError ? (
        <p role="alert" className="border border-destructive/40 bg-destructive/10 px-4 py-3 text-body-sm text-destructive">
          {createError}
        </p>
      ) : null}

      {/* Sticky action bar */}
      {n > 0 ? (
        <CreateReelBar
          selectionLabel={selectionLabel}
          presetLabel={presetLabel}
          songTitle={songTitle}
          musicDecided={musicDecided}
          format={editConfig.format}
          onFormatChange={(format) => changeEditConfig({ ...editConfig, format })}
          creating={creating}
          briefItems={briefItems}
          briefApproved={briefApproved}
          onBriefApprovedChange={setBriefApproved}
          onCreate={onCreate}
        />
      ) : null}

      <SongPickerDialog
        open={songOpen}
        onOpenChange={setSongOpen}
        onChoose={onChooseSong}
        selectedSongId={songId}
      />
    </div>
  );
}

function musicBriefFor(decided: boolean, songTitle: string | null, volumePercent: number): MusicBrief {
  if (!decided) return { status: 'pending' };
  if (songTitle) return { status: 'track', title: songTitle, volumePercent };
  return { status: 'none' };
}

function MusicDecisionPanel({
  decided,
  songTitle,
  musicVolume,
  busy,
  onOpenPicker,
  onChooseNone,
  onClear,
  onVolumeChange,
}: {
  decided: boolean;
  songTitle: string | null;
  musicVolume: number;
  busy: boolean;
  onOpenPicker: () => void;
  onChooseNone: () => void;
  onClear: () => void;
  onVolumeChange: (volume: number) => void;
}): ReactNode {
  if (decided && songTitle) {
    return (
      <div className="studio-panel flex flex-col">
        <div className="flex items-center justify-between gap-3 px-4 py-3.5">
          <div className="flex min-w-0 items-center gap-3">
            <Music className="size-5 shrink-0 text-stream" aria-hidden />
            <div className="min-w-0">
              <p className="truncate text-body-sm font-medium text-fg-1">{songTitle}</p>
              <p className="text-meta text-fg-3">Música añadida</p>
            </div>
          </div>
          <div className="flex shrink-0 gap-2">
            <Button variant="secondary" size="sm" disabled={busy} onClick={onOpenPicker}>
              Cambiar
            </Button>
            <Button variant="ghost" size="sm" disabled={busy} onClick={onClear}>
              Quitar
            </Button>
          </div>
        </div>
        <div className="flex items-center gap-4 border-t border-border-subtle px-4 py-3">
          <label
            htmlFor="music-volume"
            className="shrink-0 font-mono text-meta uppercase tracking-wider text-fg-2"
          >
            VOLUMEN <span className="text-stream-text">· {musicVolume}%</span>
          </label>
          <input
            id="music-volume"
            type="range"
            min={VOLUME_MIN}
            max={VOLUME_MAX}
            step={VOLUME_STEP}
            value={musicVolume}
            disabled={busy}
            onChange={(e) => onVolumeChange(Number(e.target.value))}
            className="h-1 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-stream disabled:cursor-not-allowed disabled:opacity-50"
          />
        </div>
      </div>
    );
  }

  if (decided) {
    return (
      <div className="studio-panel flex items-center justify-between gap-3 px-4 py-3.5">
        <div className="min-w-0">
          <p className="text-body-sm font-medium text-fg-1">Sin música</p>
          <p className="text-meta text-fg-3">El reel se forja con el audio de la partida.</p>
        </div>
        <div className="flex shrink-0 gap-2">
          <Button variant="secondary" size="sm" disabled={busy} onClick={onOpenPicker}>
            Elegir tema
          </Button>
          <Button variant="ghost" size="sm" disabled={busy} onClick={onClear}>
            Cambiar
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-body-sm text-fg-2">
        Elige un tema o confirma que va sin música. Después de forjar, cambiarlo exige otro render.
      </p>
      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          disabled={busy}
          onClick={onOpenPicker}
          className="flex min-h-11 flex-1 items-center gap-3 border border-dashed border-stream/55 bg-surface-2 px-4 py-3.5 text-left text-body-sm text-fg-1 transition-colors duration-(--dur-fast) ease-standard hover:border-stream hover:text-fg-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Music className="size-5 shrink-0 text-stream" aria-hidden />
          Elegir un tema
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={onChooseNone}
          className="flex min-h-11 items-center border border-border-strong bg-surface-2 px-4 py-3.5 text-body-sm text-fg-2 transition-colors duration-(--dur-fast) ease-standard hover:border-primary/55 hover:text-fg-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50"
        >
          Sin música
        </button>
      </div>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex flex-col gap-7" role="status" aria-label="Cargando la partida">
      <Skeleton className="h-5 w-28" />
      <Skeleton className="h-16 w-full" />
      <div className="grid items-start gap-7 @[64rem]/content:grid-cols-[minmax(0,1.55fr)_minmax(21rem,0.85fr)]">
        <div className="flex flex-col gap-4">
          <Skeleton className="h-6 w-52" />
          <div className="flex flex-col gap-px overflow-hidden border border-border">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-[86px] w-full" />
            ))}
          </div>
        </div>
        <div className="flex flex-col gap-6">
          <Skeleton className="h-40" />
          <Skeleton className="h-32" />
        </div>
      </div>
    </div>
  );
}
