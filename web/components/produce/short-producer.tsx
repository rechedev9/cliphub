'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ChevronRight, Clapperboard, RefreshCw, Zap } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { parseFailureReason } from '@/lib/api/failure-reason';
import { DEFAULT_PRE_ROLL_SECONDS, type EditConfig, type Match, type Play, type Preset, type RenderFormat } from '@/lib/api/types';
import { GAME_VOLUME_DEFAULT_PERCENT } from '@/lib/api/reel-music';
import { fetchStorageSpace } from '@/lib/api/video-edit';
import { PRODUCE_FORMAT, RESULTS_HREF, STORAGE_HREF, seriesHref } from '@/lib/clips/routes';
import { FORGE_HINT_LOADING_PRESETS, forgeHint } from '@/lib/forge-hint';
import { shortCreateErrorMessage } from '@/lib/produce/short-create-error';
import { PRODUCE_SHORT_CTA, PRODUCE_SHORT_DRAFT_RESET, PRODUCE_SHORT_DRAFT_RESTORED, PRODUCE_SHORT_EMPTY_HINT, PRODUCE_SHORT_TITLE, PRODUCE_SUMMARY_CTA, PRODUCE_SUMMARY_TITLE } from '@/lib/produce/copy';
import { addWithoutClashes, BEST_MOMENTS_TARGET_SECONDS, bestMomentsEdit, clashingPlay } from '@/lib/produce/multi-pov';
import { autoPovNotes, autoPovSummary, fetchAutoPov } from '@/lib/produce/auto-pov';
import { defaultShortSettings } from '@/lib/produce/short-draft';
import { preRollOf, preRolledPlays, preRolledSelection, withPreRoll } from '@/lib/produce/pre-roll';
import { summaryStyle, withoutOnePlayerOptions, withSummaryStyle } from '@/lib/produce/summary-style';
import { editCtaLabel, shortEditNeedsCapture, spaceWarning, type ShortCaptureChoice, type ShortEditSeed, type SpaceCheck } from '@/lib/produce/edit-capture';
import { horizontalCtaLabel, horizontalGonePlays, horizontalNeedsCapture, horizontalNotice, withShortComms, withShortFraming, type HorizontalDraft } from '@/lib/produce/horizontal';
import { filterPlays, NO_PLAY_FILTER, playFilterOptions, type PlayFilter } from '@/lib/produce/play-filter';
import { useShortDraft } from '@/hooks/use-short-draft';
import {
  autoPickBestPlays,
  estimatedSelectionSeconds,
  formatClock,
  overTargetNotice,
  roundsSummary,
  selectionTimeline,
  SHORT_TARGET_SECONDS,
  switchReelSelection,
  victimReplaySeconds,
  type ReelSelections,
} from '@/lib/produce/short-selection';
import { editSubtitles, playScope, subtitlesSummary, withSubtitles } from '@/lib/produce/subtitles';
import { canForgeReel, constrainEditConfig, missingAffiliateCode, reelCreativeBrief, verticalHudAllowed, withPresetLimits, type MusicBrief } from '@/lib/reel-brief';
import { radarAllowed } from '@/lib/radar-overlay';
import { MULTI_PLAYER_CARD_REASON } from '@/lib/player-cards';
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
import { useProduceBack } from './use-produce-back';
import { ShortStoryboard } from './short-storyboard';
import { ShortHero } from './short-hero';
import { FORMAT_TONE, type FormatTone } from '@/components/studio/format-tone';
import { roundOrdinals } from '@/lib/produce/round-ordinals';
import { playsInRounds, roundListLabel } from '@/lib/library/analysis-video';
import { PresetPreview } from './preset-preview';
import { ShortPreview } from './short-preview';
import { PlayFilters } from './play-filters';
import { AutoShortsDialog } from './auto-shorts-dialog';
import { PreRollPicker, SummaryStylePicker } from './summary-options';
import { SubtitlesPanel } from './subtitles-panel';

const PANEL_CLASS = 'flex min-w-0 flex-col gap-3 rounded-2xl bg-surface-2 p-4';
const PANEL_TITLE_CLASS = 'font-display text-body font-bold text-fg-1';
/** The space check waits for the selection to settle. */
const SPACE_CHECK_DELAY_MS = 800;
/** The preview frame's ring; FORMAT_TONE has no ring class. */
const FRAME_RING: Readonly<Record<FormatTone, string>> = {
  short: 'ring-format-short',
  full: 'ring-format-full',
  stream: 'ring-format-stream',
  summary: 'ring-format-summary',
};

export type ShortProducerProps = {
  matchId: string;
  match: Match;
  plays: Play[];
  /** One clip per death of the player; the list switches to them for a death reel. */
  deathPlays?: Play[];
  /** From a series map card: a finished render returns to the series. */
  seriesId: string | null;
  /** From the analysis area: the plays of these rounds open selected. */
  focusRounds?: readonly number[] | null;
  /** 16:9 makes the best-moments video of a multi-POV match; a Short is 9:16. */
  format?: RenderFormat;
  /** Editar in Mis vídeos: the version this edit starts from. */
  editSeed?: ShortEditSeed | null;
  /** Crear horizontal in Mis vídeos: the Short this 16:9 summary is made from. */
  horizontal?: HorizontalDraft | null;
};

/** The Short constructor: the best minute is preselected, then preset, music and overlays, render. */
export function ShortProducer({ matchId, match, plays: killPlays, deathPlays: allDeathPlays = [], seriesId, focusRounds = null, format = 'short-9x16', editSeed = null, horizontal = null }: ShortProducerProps): ReactNode {
  const router = useRouter();
  const landscape = format === 'landscape-16x9';
  const tone: FormatTone = landscape ? 'summary' : 'short';
  const multiPov = (match.povPlayers?.length ?? 0) > 1;
  // One player in 16:9: the summary of every kill, with its own pre-roll and capture style.
  const onePlayerSummary = landscape && !multiPov;
  // A horizontal keeps its Short's capture style, even with several players, so it reuses the Short's clips.
  const ownStyle = onePlayerSummary || (landscape && horizontal !== null);
  const framing = horizontal ?? editSeed;
  const { back } = useProduceBack({ jobId: matchId, format: landscape ? PRODUCE_FORMAT.full : PRODUCE_FORMAT.short, seriesId });
  // A multi-POV video mixes plays only: the death list is the anchor's alone.
  const deathPlays = multiPov ? [] : allDeathPlays;
  const targetSeconds = landscape ? BEST_MOMENTS_TARGET_SECONDS : SHORT_TARGET_SECONDS;
  const targetMinutes = Math.round(targetSeconds / 60);
  const shape = (edit: EditConfig, deaths: boolean): EditConfig => {
    let own = edit;
    if (multiPov) own = horizontal ? withPreRoll(edit, DEFAULT_PRE_ROLL_SECONDS) : withoutOnePlayerOptions(edit);
    return landscape
      ? withShortFraming(withShortComms(bestMomentsEdit(constrainEditConfig({ ...own, format })), horizontal === null ? null : own), framing)
      : constrainEditConfig({ ...own, format, deathReel: deaths, ...(multiPov ? { killCounter: false, statsCard: false } : {}) });
  };
  const [presets, setPresets] = useState<Preset[] | null>(null);
  const [presetAttempt, setPresetAttempt] = useState(0);
  const focusIds = focusRounds ? playsInRounds(killPlays, focusRounds) : [];
  const { settings, updateSettings, resetSettings, discardDraft, loaded, restored } = useShortDraft({ matchId, plays: killPlays, deathPlays, focusIds, targetSeconds, seed: editSeed?.settings ?? horizontal?.settings ?? null, summary: onePlayerSummary });
  const { variant, songId, songTitle, musicVolume, gameVolume } = settings;
  // A player without kills can still make a reel of their deaths.
  const deathReel = deathPlays.length > 0 && (settings.editConfig.deathReel === true || killPlays.length === 0);
  const editConfig = shape(settings.editConfig, deathReel);
  const plays = deathReel ? deathPlays : killPlays;
  const selectedIds = useMemo(() => new Set(settings.selectedIds), [settings.selectedIds]);
  const [songOpen, setSongOpen] = useState(false);
  const [stylesOpen, setStylesOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  // Elegir automáticamente: the picked ids, who it left out and each play's reason.
  const [autoPov, setAutoPov] = useState<{ ids: string[]; summary: string | null; notes: Map<string, string> } | null>(null);
  const [directing, setDirecting] = useState(false);
  const busy = creating || !loaded;
  const [reelSelections, setReelSelections] = useState<ReelSelections>({ kills: null, deaths: null });
  const [playFilter, setPlayFilter] = useState<PlayFilter>(NO_PLAY_FILTER);
  const filterOptions = useMemo(() => playFilterOptions(plays), [plays]);
  const visiblePlays = useMemo(() => filterPlays(plays, playFilter), [plays, playFilter]);
  const visibleIds = useMemo(() => new Set(visiblePlays.map((play) => play.id)), [visiblePlays]);

  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const list = await api.listPresets();
        if (!active) return;
        setPresets(list);
        // An unavailable catalog must not overwrite a saved preset with null.
        if (list.length > 0) updateSettings((cur) => ({ variant: selectShortsFormat(format, cur.variant, list).variant }), false);
      } catch {
        if (active) setPresets([]);
      }
    })();
    return () => {
      active = false;
    };
  }, [updateSettings, presetAttempt, format]);

  const selectedPlays = plays.filter((play) => selectedIds.has(play.id));
  const replay = { victimReplay: editConfig.victimReplay === true, victimReplaySpeed: editConfig.victimReplaySpeed };
  const preRoll = preRollOf(editConfig);
  // What the capture records: the pre-roll lengthens each play and can join close plays of one round.
  const recorded = preRolledSelection(plays, { selectedIds, preRollSeconds: preRoll });
  const rowPlays = preRolledPlays(plays, preRoll);
  const cues = selectionTimeline(recorded, { selectedIds: new Set(recorded.map((play) => play.id)), ...replay });
  const estimatedSeconds = estimatedSelectionSeconds(recorded, replay);
  // A one-player summary keeps every kill the user leaves in: it has no length to stay within.
  const capSeconds = onePlayerSummary ? null : targetSeconds;
  const overTarget = capSeconds !== null && estimatedSeconds > capSeconds;
  const overNotice = capSeconds === null ? null : overTargetNotice({
    totalSeconds: estimatedSeconds,
    targetSeconds: capSeconds,
    replaySeconds: estimatedSeconds - estimatedSelectionSeconds(recorded),
    short: !landscape,
  });
  const hiddenSelected = selectedPlays.filter((play) => !visibleIds.has(play.id)).length;
  const codeMissing = missingAffiliateCode(editConfig);
  const visiblePresets = presets === null ? null : shortsPresetsForFormat(presets, format);
  const presetsFailed = visiblePresets !== null && visiblePresets.length === 0;
  // Multi-POV: a play overlapping a chosen play of another player cannot be filmed with it.
  const clashes = useMemo(() => {
    const out = new Map<string, string>();
    for (const play of plays) {
      const other = selectedIds.has(play.id) ? undefined : clashingPlay(play, plays.filter((p) => selectedIds.has(p.id)));
      if (other?.player) out.set(play.id, other.player.name);
    }
    return out;
  }, [plays, selectedIds]);
  // The summary stays only while the selection is still the automatic one.
  const autoPovNote = autoPov !== null && autoPov.ids.length === selectedIds.size && autoPov.ids.every((id) => selectedIds.has(id)) ? autoPov.summary : null;
  const selectedPreset = visiblePresets?.find((preset) => preset.name === variant) ?? null;
  const style = summaryStyle(editConfig);
  // The summary's capture style replaces the preset's HUD in the preview and the brief.
  const shownPreset = ownStyle && selectedPreset ? { ...selectedPreset, hudMode: style.hud } : selectedPreset;
  const presetLabel = ownStyle ? style.label : selectedPreset?.label ?? null;
  // A Full HUD style keeps the draft's vertical HUD but never sends it.
  const hudAllowed = verticalHudAllowed(selectedPreset);
  const briefItems = reelCreativeBrief(editConfig, shownPreset, musicBriefFor(songTitle, musicVolume, gameVolume));
  const ready = !codeMissing && canForgeReel({
    creating: busy,
    hasPreset: selectedPreset !== null,
    selectionCount: selectedPlays.length,
  });
  // Editar reuses the version's capture unless a change needs CS2 again; the record worker still decides.
  const recordedPreset = editSeed ? presets?.find((preset) => preset.name === editSeed.settings.variant) : undefined;
  const selectedIdList = selectedPlays.map((play) => play.id);
  const nextCapture: ShortCaptureChoice | null = selectedPreset === null ? null
    : { playIds: selectedIdList, hudMode: selectedPreset.hudMode, edit: withPresetLimits(shape(editConfig, deathReel), selectedPreset) };
  let needsCapture: boolean | null = null;
  if (nextCapture !== null && horizontal !== null) needsCapture = horizontalNeedsCapture(horizontal, nextCapture);
  else if (nextCapture !== null && editSeed !== null) {
    needsCapture = shortEditNeedsCapture({ playIds: editSeed.settings.selectedIds, hudMode: recordedPreset?.hudMode, edit: editSeed.settings.editConfig }, nextCapture);
  }
  const [space, setSpace] = useState<SpaceCheck | null>(null);
  const renderSeconds = Math.round(estimatedSeconds);
  const captureSeconds = needsCapture === false ? 0 : renderSeconds;
  useEffect(() => {
    if (renderSeconds === 0) return;
    const controller = new AbortController();
    const timer = setTimeout(() => {
      void fetchStorageSpace({ captureSeconds, renderSeconds, signal: controller.signal }).catch(() => null).then((next) => {
        if (!controller.signal.aborted) setSpace(next);
      });
    }, SPACE_CHECK_DELAY_MS);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [captureSeconds, renderSeconds]);
  const lowSpace = renderSeconds > 0 ? spaceWarning(space) : null;

  function toggleSelect(playId: string): void {
    if (busy) return;
    updateSettings((prev) => {
      const next = new Set(prev.selectedIds);
      if (next.has(playId)) next.delete(playId);
      else if (!clashes.has(playId)) next.add(playId);
      return { selectedIds: [...next] };
    });
  }

  // Auto-POV: Studio picks the players and plays like an observer, within the video's length and never two POVs at once.
  async function chooseAutomatically(): Promise<void> {
    if (busy || directing) return;
    setDirecting(true);
    try {
      const direction = await fetchAutoPov(matchId, { targetSeconds, extraSeconds: victimReplaySeconds(replay), ...(onePlayerSummary ? { preRollSeconds: preRoll } : {}) });
      const known = new Set(killPlays.map((play) => play.id));
      const ids = direction.segmentIds.filter((id) => known.has(id));
      if (ids.length === 0) {
        toast('Ninguna jugada para el resumen', { description: multiPov ? 'Estos jugadores no tienen kills en la demo. Elige las jugadas a mano.' : 'El jugador no tiene kills en la demo.' });
        return;
      }
      setPlayFilter(NO_PLAY_FILTER);
      updateSettings({ selectedIds: ids });
      setAutoPov({ ids, summary: autoPovSummary(direction), notes: autoPovNotes(direction) });
    } catch {
      toast.error('No se pudo elegir automáticamente', { description: 'Studio no ha respondido. Prueba de nuevo o elige las jugadas a mano.' });
    } finally {
      setDirecting(false);
    }
  }

  function changeEditConfig(next: EditConfig): void {
    if (busy) return;
    updateSettings({ editConfig: shape(next, deathReel) });
  }

  function chooseDeathReel(next: boolean): void {
    if (busy || next === deathReel) return;
    // Each tab keeps its own selection; a first visit picks what fits the target, like "Ajustar".
    const fallback = (): string[] => [...(next
      ? autoPickBestPlays(deathPlays, { targetSeconds })
      : autoPickBestPlays(killPlays, { ...replay, targetSeconds }))];
    const switched = switchReelSelection(reelSelections, { current: settings.selectedIds, toDeaths: next, fallback });
    setReelSelections(switched.saved);
    setPlayFilter(NO_PLAY_FILTER);
    updateSettings((cur) => ({ editConfig: shape(cur.editConfig, next), selectedIds: switched.selectedIds }));
  }

  function chooseVariant(nextVariant: string): void {
    // Only presets of this format are offered, so only the variant travels.
    if (busy) return;
    updateSettings({ variant: selectShortsPreset(nextVariant, format, presets ?? []).variant });
  }

  function onChooseSong(chosenId: string, chosenTitle: string): void {
    if (busy) return;
    updateSettings({ songId: chosenId, songTitle: chosenTitle, musicDecided: true });
    setSongOpen(false);
  }

  // "Sin música" is the default, so clearing the track never leaves the music undecided.
  function resetMusic(): void {
    if (busy) return;
    updateSettings({ songId: null, songTitle: null, musicVolume: MUSIC_VOLUME.default, gameVolume: GAME_VOLUME_DEFAULT_PERCENT, musicDecided: true });
  }

  function startOver(): void {
    if (busy) return;
    resetSettings({ ...defaultShortSettings(killPlays, { deathPlays, targetSeconds, summary: onePlayerSummary }), variant: selectShortsFormat(format, null, presets ?? []).variant });
    setReelSelections({ kills: null, deaths: null });
    setCreateError(null);
    setSongOpen(false);
  }

  async function onCreate(): Promise<void> {
    if (!ready) return;
    await startReel(selectedPlays.map((play) => play.id), withPresetLimits(shape(editConfig, deathReel), selectedPreset));
  }

  // Auto-Shorts: one capture of these plays, each rendered as its own Short.
  async function onCreateAutoShorts(playIds: string[]): Promise<void> {
    if (busy || playIds.length === 0 || selectedPreset === null || codeMissing) return;
    await startReel(playIds, { ...withPresetLimits(shape(editConfig, false), selectedPreset), separateShorts: true });
  }

  async function startReel(playIds: string[], edit: EditConfig): Promise<void> {
    setCreating(true);
    setCreateError(null);
    const batch = edit.separateShorts === true;
    try {
      const video = await api.createVideo({
        matchId,
        playIds,
        mode: songId ? 'music' : 'clean',
        songId: songId ?? undefined,
        // Only a reduced volume travels; full volume stays the legacy default.
        musicVolume: songId && musicVolume < MUSIC_VOLUME.max ? musicVolume / 100 : undefined,
        gameVolume: songId ? gameVolume / 100 : undefined,
        variant: variant ?? undefined,
        editConfig: edit,
      });
      discardDraft();
      // A start Studio refused lands as a failed card in Mis vídeos; the toast says so instead of "en preparación".
      if (video.status === 'failed') toast.error(failedStartTitle(landscape, batch), { description: parseFailureReason(video.failureReason).message });
      else if (batch) toast('Auto-Shorts en preparación', { description: `CS2 graba ${playIds.length} ${playIds.length === 1 ? 'jugada' : 'jugadas'} y cada una sale como un Short en Mis vídeos.` });
      else if (horizontal !== null && needsCapture === false) toast('Montaje en cola', { description: 'Studio monta el horizontal con las jugadas ya grabadas del Short: CS2 no se abre. El Short no cambia.' });
      else if (editSeed !== null && needsCapture === false) toast('Montaje en cola', { description: 'Studio vuelve a montar el vídeo con las jugadas ya grabadas: CS2 no se abre. La versión que ya tienes se queda en Versiones.' });
      else toast(landscape ? 'Resumen en preparación' : 'Short en preparación', { description: 'Sigue la grabación y el montaje en Mis vídeos.' });
      router.push(doneHref);
    } catch (err) {
      setCreateError(shortCreateErrorMessage(err));
      setCreating(false);
    }
  }

  const heroTitle = producerTitle(match, landscape);
  const musicSummary = songTitle ? `♪ ${songTitle}` : 'Sin música';
  // A created Short lands in Mis vídeos; from a series map card it returns to the series.
  const flow = landscape ? PRODUCE_FORMAT.full : PRODUCE_FORMAT.short;
  const doneHref = seriesId ? seriesHref(seriesId, flow) : RESULTS_HREF;
  const summary = selectedPlays.length > 0 ? (
    <>
      {selectedPlays.length} {selectedPlays.length === 1 ? 'jugada' : 'jugadas'}
      <span className="text-fg-3"> · </span>
      <span className={overTarget ? 'text-warning' : undefined}>{formatClock(estimatedSeconds)} aprox.</span>
      <span className="text-fg-3"> · </span>
      <span className="text-fg-1">{presetLabel ?? (presets === null ? 'Cargando estilo' : 'Estilo pendiente')}</span>
      <span className="text-fg-3"> · </span>
      {musicSummary}
      {overNotice ? <span className="mt-0.5 block text-warning">{overNotice}</span> : null}
    </>
  ) : null;
  let footerHint = PRODUCE_SHORT_EMPTY_HINT;
  if (selectedPlays.length > 0) {
    footerHint = codeMissing
      ? 'Escribe tu código de afiliado en Textos y gráficos, o quita el banner afiliado.'
      : forgeHint(roundsSummary(selectedPlays), { label: presetLabel, loading: presets === null });
  }

  return (
    <Dialog open={stylesOpen} onOpenChange={setStylesOpen}>
      <div className="mb-5 mt-1">
        <ShortHero
          tone={tone}
          title={heroTitle}
          meta={match.map}
          cues={cues}
          totalSeconds={estimatedSeconds}
          targetSeconds={capSeconds}
          overNotice={overNotice}
          notices={<>
            {focusRounds && focusRounds.length > 0 ? (
              <p role="status" className="text-body-sm text-fg-2">
                {focusIds.length > 0
                  ? `Llegas desde el análisis: van elegidas las jugadas de ${focusRounds.length === 1 ? 'la ronda' : 'las rondas'} ${roundListLabel(focusRounds)}.`
                  : `El jugador no hace kills en ${focusRounds.length === 1 ? 'la ronda' : 'las rondas'} ${roundListLabel(focusRounds)}: ${onePlayerSummary ? 'van todas sus kills' : 'va la selección automática'}.`}
              </p>
            ) : null}
            {autoPovNote ? (
              <p role="status" className="text-body-sm text-fg-2">{autoPovNote}</p>
            ) : null}
            {editSeed ? (
              <p role="status" className="text-body-sm text-fg-2">
                Editas la versión del {editSeed.date}. La que ya tienes se queda en Versiones.
                {needsCapture === false ? ' Las jugadas ya están grabadas: solo se vuelve a montar.' : null}
                {needsCapture === true ? ' Estos cambios necesitan grabar otra vez con CS2.' : null}
              </p>
            ) : null}
            {horizontal ? (
              <p role="status" className={cn('text-body-sm', horizontalGonePlays({ draft: horizontal, playIds: selectedIdList }) > 0 ? 'text-warning' : 'text-fg-2')}>
                {horizontalNotice({ draft: horizontal, needsCapture, playIds: selectedIdList })}
              </p>
            ) : null}
            {lowSpace ? (
              <p role="status" className="text-body-sm text-warning">
                {lowSpace}{' '}
                <Link href={STORAGE_HREF} className="text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1 focus-visible:outline-2 focus-visible:outline-ring">Liberar espacio en Almacenamiento</Link>
              </p>
            ) : null}
            {restored ? (
              <p role="status" className="text-body-sm text-fg-2">
                {PRODUCE_SHORT_DRAFT_RESTORED}{' '}
                <button type="button" disabled={busy} onClick={startOver} className="text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1 focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50">
                  {PRODUCE_SHORT_DRAFT_RESET}
                </button>
              </p>
            ) : null}
          </>}
        />
      </div>
      {/* Plays | preview over the aside, until three columns fit beside each other. */}
      <div data-short-workspace="" className="grid min-w-0 items-start gap-5 @[56rem]/content:grid-cols-[minmax(0,1.3fr)_minmax(280px,1fr)] @[56rem]/content:grid-rows-[auto_1fr] @[85rem]/content:grid-cols-[minmax(0,1.3fr)_260px_minmax(280px,320px)] @[85rem]/content:grid-rows-none">
        <div className="flex min-w-0 flex-col gap-3 @[56rem]/content:row-span-2 @[85rem]/content:row-span-1">
          {deathPlays.length > 0 && killPlays.length > 0 ? (
            <ReelSourceSwitch deathReel={deathReel} disabled={busy} tone={tone} onChange={chooseDeathReel} />
          ) : null}
          <PlayList
            plays={rowPlays}
            selectedIds={selectedIds}
            title={deathReel ? 'Muertes' : 'Jugadas'}
            tone={tone}
            toolbar={<div className="flex flex-wrap gap-2">
              {onePlayerSummary ? null : <Button type="button" size="xs" variant="outline" disabled={busy || visiblePlays.length === 0}
                title={deathReel
                  ? `Deja elegidas las muertes que caben en ${targetMinutes} min. Cambia la selección actual.`
                  : `Cambia la selección por las mejores jugadas que caben en ${targetMinutes} min, contando la repetición si está activa.`}
                onClick={() => updateSettings({ selectedIds: [...autoPickBestPlays(preRolledSelection(visiblePlays, { selectedIds: visibleIds, preRollSeconds: preRoll }), { victimReplay: !deathReel && editConfig.victimReplay === true, victimReplaySpeed: editConfig.victimReplaySpeed, targetSeconds })] })}><Zap aria-hidden />{deathReel ? `Ajustar a ${targetMinutes} min` : `Elegir las mejores (${targetMinutes} min)`}</Button>}
              {(multiPov || onePlayerSummary) && !deathReel ? (
                <Button type="button" size="xs" variant="outline" disabled={busy || killPlays.length === 0} loading={directing} loadingText="Eligiendo…"
                  title={multiPov
                    ? 'Elige jugadores y jugadas como un observador: clutches, duelos, la bomba y las rachas, sin dos vistas a la vez.'
                    : `Elige las mejores jugadas en unos ${targetMinutes} min, como un observador: clutches, duelos, la bomba y las rachas.`}
                  onClick={() => void chooseAutomatically()}><Clapperboard aria-hidden />Elegir automáticamente</Button>
              ) : null}
              {!landscape && !deathReel ? <AutoShortsDialog matchId={matchId} plays={killPlays} disabled={busy || selectedPreset === null || codeMissing} creating={creating} onCreate={(ids) => void onCreateAutoShorts(ids)} /> : null}
            </div>}
            filters={<PlayFilters options={filterOptions} value={playFilter} disabled={busy} onChange={setPlayFilter} />}
            visibleIds={visibleIds}
            emptyFiltered={
              <p className="rounded-xl bg-surface-2 p-4 text-body-sm text-fg-2">Ninguna jugada cumple estos filtros. Las elegidas siguen en el vídeo.</p>
            }
            counter={<span className="tabular-nums">
              {selectedPlays.length} de {plays.length}
              {hiddenSelected > 0 ? ` · ${hiddenSelected} ${hiddenSelected === 1 ? 'elegida oculta' : 'elegidas ocultas'} por el filtro` : null}
            </span>}
            clashes={multiPov ? clashes : undefined}
            notes={autoPov?.notes}
            onToggle={toggleSelect}
            onSelectAll={() => !busy && updateSettings({ selectedIds: addWithoutClashes(selectedPlays, visiblePlays).map((play) => play.id) })}
            onClear={() => !busy && updateSettings({ selectedIds: [] })}
          />
        </div>

        <section aria-label="Vista previa" className="flex min-w-0 flex-col items-center gap-3 @[85rem]/content:sticky @[85rem]/content:top-4">
          {shownPreset ? (
            <ShortPreview preset={shownPreset} edit={withPresetLimits(editConfig, selectedPreset)} map={match.map} player={match.player ?? 'Jugador'}
              frameClassName={cn('rounded-[1.625rem] bg-surface-0 p-1.5 ring-2', FRAME_RING[tone])} className={landscape ? 'w-full max-w-md' : 'w-60'} />
          ) : (
            <div aria-hidden className={cn('rounded-[1.625rem] bg-surface-0 p-1.5 ring-2', FRAME_RING[tone], landscape ? 'w-full max-w-md' : 'w-60')}>
              <div className={cn('w-full rounded-[1.25rem] bg-surface-2', landscape ? 'aspect-video' : 'aspect-[9/16]')} />
            </div>
          )}
          <p className={cn('text-center text-meta text-fg-3', landscape ? 'max-w-md' : 'max-w-60')}>{previewCaption(presetLabel, presets === null)}</p>
          <ShortStoryboard cues={cues} ordinals={roundOrdinals(plays)} {...(landscape ? { title: 'Guion del resumen', videoLabel: 'Resumen' } : {})} />
        </section>

        <aside className="flex min-w-0 flex-col gap-3">
          <section aria-labelledby="short-preset-title" className={PANEL_CLASS}>
            <div className="flex flex-wrap items-center justify-between gap-2">
              <label id="short-preset-title" htmlFor="short-preset" className={PANEL_TITLE_CLASS}>Estilo</label>
              {selectedPreset && !ownStyle ? (
                <DialogTrigger asChild>
                  <Button type="button" variant="link" size="xs" disabled={busy}>Comparar estilos</Button>
                </DialogTrigger>
              ) : null}
            </div>
            {presetsFailed ? (
              <div role="alert" className="flex flex-wrap items-center gap-2 text-body-sm text-fg-2">
                <span>No se pudieron cargar los estilos.</span>
                <Button type="button" variant="outline" size="xs" onClick={() => { setPresets(null); setPresetAttempt((attempt) => attempt + 1); }}>
                  <RefreshCw aria-hidden />
                  Reintentar
                </Button>
              </div>
            ) : null}
            {!presetsFailed && ownStyle ? (
              <SummaryStylePicker value={style.hud} disabled={busy} onChange={(hud) => changeEditConfig(withSummaryStyle(editConfig, hud))} />
            ) : null}
            {!presetsFailed && !ownStyle ? (
              <Select value={variant ?? ''} onValueChange={chooseVariant} disabled={busy || visiblePresets === null}>
                <SelectTrigger id="short-preset" className="h-10 font-display font-semibold">
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
            ) : null}
          </section>

          {!multiPov && !deathReel ? (
            <section aria-labelledby="short-pre-roll-title" className={PANEL_CLASS}>
              <h2 id="short-pre-roll-title" className={PANEL_TITLE_CLASS}>Antes de cada kill</h2>
              <PreRollPicker value={preRoll} disabled={busy} tone={tone} onChange={(seconds) => changeEditConfig(withPreRoll(editConfig, seconds))} />
            </section>
          ) : null}

          <MusicCard
            tone={tone}
            decided
            songTitle={songTitle}
            musicVolume={musicVolume}
            gameVolume={gameVolume}
            busy={busy}
            onOpenPicker={() => setSongOpen(true)}
            onChooseNone={resetMusic}
            onClear={resetMusic}
            onVolumeChange={(value) => updateSettings({ musicVolume: value })}
            onGameVolumeChange={(value) => updateSettings({ gameVolume: value })}
          />

          <details className={cn('group/overlays', PANEL_CLASS)}>
            <summary className="flex min-h-10 cursor-pointer list-none flex-wrap items-center justify-between gap-2 rounded-sm focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
              <span className={PANEL_TITLE_CLASS}>Textos y gráficos</span>
              <span className="flex items-center gap-1.5 text-body-sm text-fg-2">
                <span className="wrap-anywhere">{overlaysSummary(withPresetLimits(editConfig, selectedPreset))}</span>
                <ChevronRight aria-hidden className="size-4 text-fg-2 transition-transform duration-(--dur-fast) group-open/overlays:rotate-90" />
              </span>
            </summary>
            <div className="border-t border-border-subtle pt-3">
              <EditOptions
                value={editConfig}
                onChange={changeEditConfig}
                disabled={busy}
                killCounter={!multiPov}
                statsCard={!multiPov}
                playerCards={{ jobId: matchId, blockedReason: multiPov ? MULTI_PLAYER_CARD_REASON : undefined }}
                verticalHud={hudAllowed}
                radar={radarAllowed(selectedPreset)}
              />
            </div>
          </details>

          <details className={cn('group/subtitles', PANEL_CLASS)}>
            <summary className="flex min-h-10 cursor-pointer list-none flex-wrap items-center justify-between gap-2 rounded-sm focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
              <span className={PANEL_TITLE_CLASS}>Subtítulos</span>
              <span className="flex items-center gap-1.5 text-body-sm text-fg-2">
                {subtitlesSummary(editSubtitles(editConfig), editConfig.voiceComms)}
                <ChevronRight aria-hidden className="size-4 text-fg-2 transition-transform duration-(--dur-fast) group-open/subtitles:rotate-90" />
              </span>
            </summary>
            <div className="border-t border-border-subtle pt-3">
              <SubtitlesPanel jobId={matchId} scope={playScope(selectedPlays)} voices={editConfig.voiceComms} voicesLabel="Comms del equipo"
                value={editSubtitles(editConfig)} onChange={(next) => changeEditConfig(withSubtitles(editConfig, next))}
                onVoicesChange={landscape ? undefined : (voiceComms) => changeEditConfig({ ...editConfig, voiceComms })} disabled={busy} />
            </div>
          </details>
        </aside>
      </div>

      <ProduceFooter
        ready={ready}
        busy={creating}
        tone={tone}
        eyebrow={landscape ? 'Resumen 16:9' : 'Short'}
        summary={summary}
        hint={footerHint}
        briefItems={briefItems}
        back={back}
        error={createError}
        cta={
          <Button
            variant="default"
            size="sm"
            disabled={!ready}
            loading={creating}
            loadingText="Preparando grabación…"
            onClick={() => void onCreate()}
            className="shrink-0"
          >
            {horizontal
              ? horizontalCtaLabel(needsCapture)
              : editCtaLabel({ capture: needsCapture === null ? null : { needed: needsCapture, missingRounds: [] }, editing: editSeed !== null, fallback: landscape ? PRODUCE_SUMMARY_CTA : PRODUCE_SHORT_CTA })}
          </Button>
        }
      />

      <SongPickerDialog open={songOpen} onOpenChange={setSongOpen} onChoose={onChooseSong} selectedSongId={songId} />
      <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>Compara los estilos</DialogTitle>
          <DialogDescription>
            Dibujos de ejemplo del HUD y el color de cada estilo: arriba a la derecha, la kill feed del juego si el estilo la conserva. El vídeo final usa tus jugadas, y el radar, los textos y los efectos no salen aquí.
          </DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,180px),1fr))] gap-3">
          {(visiblePresets ?? []).map((preset) => (
            <button key={preset.name} type="button" disabled={busy} aria-pressed={variant === preset.name}
              onClick={() => { chooseVariant(preset.name); setStylesOpen(false); }}
              className={cn('flex flex-col items-start gap-3 rounded-lg border p-3 text-left focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50', variant === preset.name ? 'border-primary bg-primary/10' : 'border-border-strong hover:bg-surface-3')}>
              <PresetPreview preset={preset} className="mx-auto w-24" />
              <span className="font-display text-body font-semibold text-fg-1">{preset.label}</span>
              <span className="text-body-sm text-fg-2">{presetDescription(preset)}</span>
              {variant === preset.name ? <span className="mt-auto text-body-sm text-fg-1">Seleccionado</span> : null}
            </button>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}

/** Kills or deaths: what the Short is made of. */
function ReelSourceSwitch({ deathReel, disabled, tone, onChange }: { deathReel: boolean; disabled: boolean; tone: FormatTone; onChange: (deathReel: boolean) => void }): ReactNode {
  const options = [
    { value: false, label: 'Kills' },
    { value: true, label: 'Muertes' },
  ];
  return (
    <div role="group" aria-label="Contenido del Short" className="flex w-fit gap-1 rounded-lg border border-border-subtle bg-surface-2 p-1">
      {options.map((option) => (
        <button key={option.label} type="button" disabled={disabled} aria-pressed={deathReel === option.value}
          onClick={() => onChange(option.value)}
          className={cn('min-h-9 rounded-md px-4 font-display text-body-sm font-semibold transition-colors duration-(--dur-fast) focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50',
            deathReel === option.value ? FORMAT_TONE[tone].fill : 'text-fg-2 hover:bg-surface-3')}>
          {option.label}
        </button>
      ))}
    </div>
  );
}

/** "El Short de llorente"; a multi-POV resumen names its players. Falls back to the generic title. */
function producerTitle(match: Match, landscape: boolean): string {
  const povNames = (match.povPlayers ?? []).map((player) => player.name);
  const names = povNames.length > 1 ? povNames : [match.player].filter((name): name is string => Boolean(name));
  if (names.length === 0) return landscape ? PRODUCE_SUMMARY_TITLE : PRODUCE_SHORT_TITLE;
  const who = names.length === 1 ? names[0] : `${names.slice(0, -1).join(', ')} y ${names[names.length - 1]}`;
  return landscape ? `El resumen de ${who}` : `El Short de ${who}`;
}

function failedStartTitle(landscape: boolean, batch: boolean): string {
  if (landscape) return 'El resumen no ha empezado';
  return batch ? 'Los Auto-Shorts no han empezado' : 'El Short no ha empezado';
}

function musicBriefFor(songTitle: string | null, volumePercent: number, gameVolumePercent: number): MusicBrief {
  if (songTitle) return { status: 'track', title: songTitle, volumePercent, gameVolumePercent };
  return { status: 'none' };
}

function overlaysSummary(edit: EditConfig): string {
  const parts: string[] = [];
  if (edit.killCounter) parts.push('Kill counter');
  if (edit.verticalHud && edit.format === 'short-9x16' && !edit.deathReel) parts.push('HUD vertical');
  if ((edit.centeredKillfeed || edit.verticalHud) && edit.format === 'short-9x16' && !edit.deathReel) parts.push('Kill feed centrada');
  if (edit.radar) parts.push('Radar');
  if (edit.hookText) parts.push('Título');
  if (edit.intro || edit.outro) parts.push('Apertura / cierre');
  if (edit.playBadges) parts.push('Insignias');
  if (edit.winProbability) parts.push('Prob. de victoria');
  if (edit.playerLabels) parts.push('Nombres y vida');
  if (edit.grenadeTrails) parts.push('Estelas de granadas');
  if (edit.endCard && edit.format === 'short-9x16') parts.push('Carta de jugador');
  else if (edit.statsCard && edit.format === 'short-9x16') parts.push('Estadísticas');
  if (edit.victimReplay) parts.push('Repetición');
  if (edit.brandWatermark) parts.push('Marca de agua');
  if (edit.brandBanner && !edit.keyDropStyle) parts.push('Sponsor');
  if (edit.keyDropStyle) parts.push('Afiliado');
  return parts.length > 0 ? parts.join(' + ') : 'Solo efectos';
}

function previewCaption(presetLabel: string | null, loading: boolean): string {
  if (presetLabel) return `${presetLabel} con datos de ejemplo: el efecto de kill, la transición y los textos y gráficos que elijas, donde los pone el vídeo. Tus jugadas se ven al grabar.`;
  return loading ? FORGE_HINT_LOADING_PRESETS : 'Elige un estilo para verlo aquí';
}
