'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { Bookmark, Layers, Radar, RefreshCw } from 'lucide-react';
import { replayHref } from '@/lib/analysis/replay-link';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import type { CollectionItem } from '@/lib/api/collections';
import type { TacticalSource } from '@/lib/api/tactical';
import { prettyMapName } from '@/lib/format';
import { teamScoreBefore } from '@/lib/library/rounds';
import { centreRound } from '@/lib/replay/centre-round';
import type { CentreRound } from '@/lib/replay/centre-round';
import { ReplayClock } from '@/lib/replay/clock';
import { DEFAULT_LAYERS, showAllPlayers, toggleLayer, togglePlayer } from '@/lib/replay/layers';
import type { ReplayLayers } from '@/lib/replay/layers';
import type { LoadedAnalysis } from '@/lib/replay/loaded-analysis';
import { alignedTick, fromLiveSeconds, liveEvents, liveTimeline, toLiveSeconds } from '@/lib/replay/multi-round';
import { defaultMultiSelection, entryKey, matchesToLoad, stepIndex, toggleMultiKey, withFocus } from '@/lib/replay/playlist';
import {
  SEEK_STEP_SECONDS,
  advanceTransport,
  clampToTimeline,
  roundClockLabelFor,
  roundTimeline,
  seekEventSeconds,
  timelineSeconds,
  timelineTick,
} from '@/lib/tactical-timeline';
import type { RoundTimeline } from '@/lib/tactical-timeline';
import { buyLabel, ctPatternLabel, siteLabel, tPatternLabel } from '@/lib/tactical-labels';
import { noteVisibleAt } from '@/lib/tactical-spatial';
import { useMatchAnalyses } from '@/hooks/use-match-analyses';
import type { MatchAnalysis } from '@/hooks/use-match-analyses';
import { useCollections } from '@/hooks/use-collections';
import { RoundPlayActions } from '@/components/stratbook/round-play-actions';
import { TacticalTimelineBar } from '@/components/tactical/tactical-timeline';
import { useTelestratorDraft } from '@/components/tactical/spatial/telestrator';
import { useAnnotations } from '@/components/tactical/spatial/annotation-panel';
import type { NoteItem } from '@/components/tactical/spatial/annotation-panel';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { Skeleton } from '@/components/ui/skeleton';
import { BookmarkPanel, SaveMomentDialog } from './bookmark-panel';
import type { ReplayCentreView } from './centre';
import { LayerPanel } from './layer-panel';
import { RADAR_2D_VIEW } from './radar-centre';
import { ReplaySidePanel } from './replay-side-panel';
import { RoundStrip } from './round-strip';
import { useWinTimelines } from './use-win-timelines';
import { WinStrip } from './win-strip';
import type { WinStripState } from './win-strip';
import { winProbabilityAt, winStripPoints } from '@/lib/replay/win-strip';

/** The side panels and the notes follow the playhead this many times a second. */
const UI_TICKS_PER_SECOND = 4;

const EMPTY_TIMELINE = roundTimeline({ tick_start: 0, tick_freeze_end: 0, tick_end: 64, tick_official: 64 }, 64);

const DEFAULT_VIEWS: readonly ReplayCentreView[] = [RADAR_2D_VIEW];

/** The centre view the user picked last in this window. */
const VIEW_STORAGE_KEY = 'cliphub.replay.view';

function storedView(): string | null {
  try {
    return window.sessionStorage.getItem(VIEW_STORAGE_KEY);
  } catch {
    return null;
  }
}

function storeView(id: string): void {
  try {
    window.sessionStorage.setItem(VIEW_STORAGE_KEY, id);
  } catch {
    // Storage can be off; the view still switches for this page.
  }
}

export type ReplayWorkspaceProps = {
  /** `library`: strip ids are library match ids, which also enables moments, notes and plays. `job`: a Crear job id. */
  source: TacticalSource['kind'];
  /** The rounds in play order. */
  entries: readonly ReplayRef[];
  index: number;
  onIndexChange: (index: number) => void;
  /** An analysis the page already loaded, so it is not fetched twice. */
  seed?: { id: string; analysis: LoadedAnalysis };
  /** The match's name before its analysis arrives, such as `Mirage · NAVI vs FaZe`. */
  groupLabel?: (matchId: string) => string | null;
  /** Round filters, at the top of the left panel. */
  filters?: ReactNode;
  /** Centre views; the first is the default. Without it, the 2D radar. */
  centreViews?: readonly ReplayCentreView[];
  /** Shown above the round line, such as a playlist's name. */
  title?: string;
};

/**
 * The replay: filters, layers and saved moments on the left, the map in the
 * centre, the players on the right, and under them the strip of rounds, the
 * timeline with its event marks and the win probability. The rounds may come
 * from several matches; each match's analysis loads when the strip reaches it.
 */
export function ReplayWorkspace({
  source,
  entries,
  index,
  onIndexChange,
  seed,
  groupLabel,
  filters,
  centreViews = DEFAULT_VIEWS,
  title,
}: ReplayWorkspaceProps): ReactNode {
  const library = source === 'library';
  const router = useRouter();
  const clock = useMemo(() => new ReplayClock(), []);
  const rootRef = useRef<HTMLElement>(null);
  const scrubRef = useRef<HTMLInputElement>(null);
  const clockLabelRef = useRef<HTMLSpanElement>(null);
  const pendingTick = useRef<number | null>(null);

  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [loop, setLoop] = useState(false);
  const [layers, setLayers] = useState<ReplayLayers>(DEFAULT_LAYERS);
  const [multi, setMulti] = useState(false);
  const [multiKeys, setMultiKeys] = useState<string[]>([]);
  const [viewId, setViewId] = useState(centreViews[0]?.id ?? RADAR_2D_VIEW.id);
  const [followSlot, setFollowSlot] = useState<number | null>(null);
  const [uiTick, setUiTick] = useState(0);
  const [saving, setSaving] = useState<CollectionItem | null>(null);
  const draft = useTelestratorDraft();
  const collections = useCollections();

  const entry = entries[index];
  const focusKey = entry === undefined ? '' : entryKey(entry);
  const selectedKeys = useMemo(() => (multi ? withFocus(multiKeys, focusKey) : []), [multi, multiKeys, focusKey]);

  const wanted = useMemo(() => {
    const ids = matchesToLoad(entries, index, 1);
    for (const candidate of entries) {
      if (selectedKeys.includes(entryKey(candidate)) && !ids.includes(candidate.matchId)) ids.push(candidate.matchId);
    }
    return ids;
  }, [entries, index, selectedKeys]);
  const { analyses, retry } = useMatchAnalyses(source, wanted, seed);

  const current = entry === undefined ? undefined : analyses.get(entry.matchId);
  const loaded = current?.state === 'ready' ? current.analysis : null;
  const round = entry === undefined ? undefined : loaded?.round(entry.round);

  const focused = useMemo<CentreRound | null>(() => {
    if (entry === undefined || loaded === null || round === undefined) return null;
    return centreRound({ key: focusKey, matchId: entry.matchId, doc: loaded.doc, round, frames: loaded.framesFor(round.number), layers, focused: true });
  }, [entry, focusKey, layers, loaded, round]);
  const focusTimeline = useMemo(() => (round === undefined || loaded === null ? null : roundTimeline(round, loaded.doc.demo.tickrate)), [loaded, round]);

  const overlay = useMemo<CentreRound[]>(() => {
    if (!multi || focused === null) return [];
    const out: CentreRound[] = [];
    for (const candidate of entries) {
      const key = entryKey(candidate);
      if (key === focused.key || !selectedKeys.includes(key)) continue;
      const state = analyses.get(candidate.matchId);
      if (state?.state !== 'ready' || state.analysis.doc.demo.map !== focused.doc.demo.map) continue;
      const other = state.analysis.round(candidate.round);
      if (other === undefined) continue;
      out.push(centreRound({ key, matchId: candidate.matchId, doc: state.analysis.doc, round: other, frames: state.analysis.framesFor(other.number), layers, focused: false }));
    }
    return out;
  }, [analyses, entries, focused, layers, multi, selectedKeys]);

  const centreRounds = useMemo(() => (focused === null ? [] : [...overlay, focused]), [focused, overlay]);
  const transport = useMemo<RoundTimeline>(() => {
    if (focused === null) return EMPTY_TIMELINE;
    return multi ? liveTimeline(centreRounds.map((entryRound) => entryRound.timeline), focused.timeline.tickrate) : focused.timeline;
  }, [centreRounds, focused, multi]);
  const transportEvents = useMemo(() => {
    if (focused === null) return [];
    return multi ? liveEvents(focused.timeline, focused.events, transport.durationSeconds) : focused.events;
  }, [focused, multi, transport]);

  const tickOf = useCallback(
    (entryRound: CentreRound, seconds: number) => (multi ? alignedTick(entryRound.timeline, seconds) : timelineTick(entryRound.timeline, seconds)),
    [multi],
  );
  /** A tick of the focused round as a position of the current clock. */
  const clockSecondsOf = useCallback(
    (timeline: RoundTimeline, tick: number) => {
      const seconds = clampToTimeline(timeline, timelineSeconds(timeline, tick));
      return multi ? toLiveSeconds(timeline, seconds) : seconds;
    },
    [multi],
  );

  // A new round opens at its start or at the moment its link names; in multi
  // mode the shared clock keeps its place so the same second can be compared.
  const multiRef = useRef(multi);
  multiRef.current = multi;
  const entryTickRef = useRef(entry?.tick);
  entryTickRef.current = entry?.tick;
  useEffect(() => {
    if (focusTimeline === null) return;
    const tick = pendingTick.current ?? entryTickRef.current;
    pendingTick.current = null;
    if (multiRef.current && tick === undefined) return;
    const seconds = tick === undefined ? 0 : clampToTimeline(focusTimeline, timelineSeconds(focusTimeline, tick));
    clock.set(multiRef.current ? toLiveSeconds(focusTimeline, seconds) : seconds);
  }, [focusKey, focusTimeline, clock]);

  useEffect(() => {
    const update = (seconds: number): void => {
      const label = roundClockLabelFor(transport, seconds);
      if (scrubRef.current !== null) {
        scrubRef.current.value = String(seconds);
        scrubRef.current.setAttribute('aria-valuetext', label);
      }
      if (clockLabelRef.current !== null && clockLabelRef.current.textContent !== label) clockLabelRef.current.textContent = label;
      const step = Math.floor(seconds * UI_TICKS_PER_SECOND);
      setUiTick((value) => (value === step ? value : step));
    };
    update(clock.seconds);
    return clock.subscribe(update);
  }, [clock, transport]);

  const seekTo = useCallback((seconds: number) => clock.set(clampToTimeline(transport, seconds)), [clock, transport]);

  const select = useCallback(
    (next: number, tick?: number) => {
      pendingTick.current = tick ?? null;
      onIndexChange(stepIndex(entries.length, next, 0));
    },
    [entries.length, onIndexChange],
  );

  const ready = focused !== null;
  useEffect(() => {
    if (!playing || !ready) return;
    let frame = 0;
    let last = performance.now();
    const step = (now: number): void => {
      const next = advanceTransport(
        { position: clock.seconds, playing: true },
        { elapsedMs: now - last, speed, durationSeconds: transport.durationSeconds, loop },
      );
      last = now;
      clock.set(next.position);
      if (!next.playing) {
        // A playlist plays on: the next round starts where this one ends.
        if (!multi && index < entries.length - 1) select(index + 1);
        else setPlaying(false);
        return;
      }
      frame = requestAnimationFrame(step);
    };
    frame = requestAnimationFrame(step);
    return () => cancelAnimationFrame(frame);
  }, [playing, ready, speed, loop, transport, clock, multi, index, entries.length, select]);

  const togglePlay = useCallback(() => {
    setPlaying((value) => {
      if (value) return false;
      if (clock.seconds >= transport.durationSeconds) clock.set(0);
      return true;
    });
  }, [clock, transport]);

  const toggleMulti = (): void => {
    if (focusTimeline === null) return;
    if (multi) {
      clock.set(fromLiveSeconds(focusTimeline, clock.seconds));
      setMulti(false);
      return;
    }
    clock.set(toLiveSeconds(focusTimeline, clock.seconds));
    setMultiKeys(defaultMultiSelection(entries, index));
    setMulti(true);
  };

  const keyHandler = useRef<(event: KeyboardEvent) => void>(() => {});
  keyHandler.current = (event: KeyboardEvent) => {
    if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return;
    const target = event.target;
    if (target instanceof HTMLElement && target !== document.body) {
      if (rootRef.current === null || !rootRef.current.contains(target)) return;
      if (target.isContentEditable || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT') return;
      const inputType = target.tagName === 'INPUT' ? target.getAttribute('type') : null;
      if (inputType !== null && inputType !== 'range' && inputType !== 'checkbox') return;
      if (event.key === ' ' && (target.tagName === 'BUTTON' || inputType === 'checkbox')) return;
      if ((event.key === 'ArrowLeft' || event.key === 'ArrowRight') && inputType === 'range') return;
    }
    if (event.key === ' ') {
      event.preventDefault();
      togglePlay();
    } else if (event.key === '[' || event.key === ']') {
      event.preventDefault();
      select(index + (event.key === ']' ? 1 : -1));
    } else if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
      event.preventDefault();
      const direction = event.key === 'ArrowRight' ? 1 : -1;
      if (event.shiftKey) {
        const at = seekEventSeconds(transportEvents, clock.seconds, direction);
        if (at !== undefined) seekTo(at);
      } else seekTo(clock.seconds + direction * SEEK_STEP_SECONDS);
    }
  };
  useEffect(() => {
    const listener = (event: KeyboardEvent): void => keyHandler.current(event);
    window.addEventListener('keydown', listener);
    return () => window.removeEventListener('keydown', listener);
  }, []);

  const win = useWinTimelines(library && entry !== undefined && loaded !== null ? entry.matchId : null);
  const winState = useMemo<WinStripState | null>(() => {
    if (win === null || focused === null || focusTimeline === null) return null;
    if (win.kind !== 'ready') return win.kind === 'loading' ? { kind: 'loading' } : win;
    const points = win.timelines.find((timeline) => timeline.round === focused.round.number)?.points ?? [];
    if (points.length === 0) return { kind: 'none', reason: 'Esta ronda no tiene probabilidad de victoria calculada.' };
    const liveNow = multi ? uiTick / UI_TICKS_PER_SECOND : toLiveSeconds(focusTimeline, uiTick / UI_TICKS_PER_SECOND);
    return {
      kind: 'ready',
      points: multi ? winStripPoints(points, 0, transport.durationSeconds) : winStripPoints(points, focusTimeline.freezeSeconds, focusTimeline.durationSeconds),
      now: winProbabilityAt(points, liveNow),
    };
  }, [focusTimeline, focused, multi, transport, uiTick, win]);

  const roundInfo = useCallback(
    (candidate: ReplayRef) => {
      const state = analyses.get(candidate.matchId);
      if (state?.state !== 'ready') return null;
      const found = state.analysis.round(candidate.round);
      return found === undefined ? null : { score: teamScoreBefore(state.analysis.doc, found).join(':'), winner: found.winner };
    },
    [analyses],
  );
  const labelOf = useCallback(
    (matchId: string) => {
      const state = analyses.get(matchId);
      if (state?.state === 'ready') {
        const { doc } = state.analysis;
        const teams = doc.teams.map((team) => team.name || team.key).join(' vs ');
        return `${prettyMapName(doc.demo.map)} · ${teams}`;
      }
      if (state?.state === 'failed') return 'Partida sin análisis';
      return groupLabel?.(matchId) ?? (state === undefined ? 'Partida' : 'Cargando partida…');
    },
    [analyses, groupLabel],
  );
  const momentLabel = useCallback(
    (item: CollectionItem) => {
      const state = analyses.get(item.match_id);
      if (state?.state !== 'ready') return `${labelOf(item.match_id)} · R${item.round}`;
      const found = state.analysis.round(item.round);
      if (found === undefined) return `R${item.round}`;
      const timeline = roundTimeline(found, state.analysis.doc.demo.tickrate);
      return `${prettyMapName(state.analysis.doc.demo.map)} · R${item.round} · ${roundClockLabelFor(timeline, timelineSeconds(timeline, item.tick))}`;
    },
    [analyses, labelOf],
  );

  const openMoment = (item: CollectionItem): void => {
    const at = entries.findIndex((candidate) => candidate.matchId === item.match_id && candidate.round === item.round);
    if (at < 0) {
      router.push(replayHref([{ matchId: item.match_id, round: item.round, tick: item.tick }]));
      return;
    }
    setPlaying(false);
    if (at === index && focusTimeline !== null) clock.set(clockSecondsOf(focusTimeline, item.tick));
    else select(at, item.tick);
  };

  const currentMoment = (): CollectionItem | null => {
    if (focused === null || entry === undefined) return null;
    return { match_id: entry.matchId, round: focused.round.number, tick: Math.max(0, tickOf(focused, clock.seconds)), note: '' };
  };

  const view = centreViews.find((option) => option.id === viewId) ?? centreViews[0] ?? RADAR_2D_VIEW;
  const selectView = useCallback((id: string) => {
    setViewId(id);
    setFollowSlot(null);
    storeView(id);
  }, []);
  const viewIds = centreViews.map((option) => option.id).join('|');
  useEffect(() => {
    const saved = storedView();
    if (saved !== null && viewIds.split('|').includes(saved)) setViewId(saved);
  }, [viewIds]);
  const nowSeconds = uiTick / UI_TICKS_PER_SECOND;
  const notesMatchId = library && entry !== undefined ? entry.matchId : undefined;
  const noteMatchIds = useMemo(() => (notesMatchId === undefined ? [] : [notesMatchId]), [notesMatchId]);
  const notes = useAnnotations(noteMatchIds, entry?.round);
  const noteItems: NoteItem[] =
    focusTimeline === null
      ? []
      : (notes.notes ?? []).map((note) => {
          const at = clockSecondsOf(focusTimeline, note.tick);
          return {
            note,
            label: roundClockLabelFor(focusTimeline, timelineSeconds(focusTimeline, note.tick)),
            active: noteVisibleAt(at, nowSeconds),
            onSeek: () => {
              setPlaying(false);
              clock.set(at);
            },
          };
        });
  const shownDrawings = noteItems
    .filter((item) => item.active && item.note.strokes.length > 0)
    .map((item) => ({ id: item.note.id, strokes: item.note.strokes }));
  const saveNote = (text: string): Promise<boolean> => {
    if (notesMatchId === undefined || focused === null) return Promise.resolve(false);
    return notes.create({ match_id: notesMatchId, round: focused.round.number, tick: tickOf(focused, clock.seconds), text, strokes: draft.strokes });
  };

  if (entries.length === 0) {
    return (
      <section className="studio-panel grid gap-5 p-4 sm:p-5 lg:grid-cols-[260px_minmax(0,1fr)]" aria-label="Repetición">
        {filters === undefined ? null : <div className="flex flex-col gap-3">{filters}</div>}
        <p className="self-center text-body text-fg-2">Ninguna ronda cumple el filtro. Quita alguno para volver a ver rondas.</p>
      </section>
    );
  }

  return (
    <section ref={rootRef} className="studio-panel @container flex flex-col" aria-label="Repetición">
      <header className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 border-b border-border-subtle px-4 py-3 sm:px-5">
        <div className="flex min-w-0 flex-col">
          {title === undefined ? null : <span className="truncate text-meta text-fg-3">{title}</span>}
          <h2 className="font-display text-title font-bold tracking-tight text-fg-1">
            Ronda {entry?.round}
            {focused === null ? null : (
              <span className="pl-3 text-body-sm font-normal tabular-nums text-fg-3">{teamScoreBefore(focused.doc, focused.round).join(':')}</span>
            )}
          </h2>
          {focused === null ? null : (
            <p className="truncate text-meta text-fg-3">
              {buyLabel(focused.round.economy.ct_buy)} vs {buyLabel(focused.round.economy.t_buy)} · {tPatternLabel(focused.round.class.t_side)} /{' '}
              {ctPatternLabel(focused.round.class.ct_side)} · {siteLabel(focused.round.class.site)}
            </p>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {centreViews.length > 1 ? (
            <Segmented label="Vista" value={view.id} options={centreViews.map((option) => ({ value: option.id, label: option.label }))} onChange={selectView} />
          ) : null}
          <Button type="button" size="sm" variant={multi ? 'outline-primary' : 'outline'} aria-pressed={multi} disabled={focused === null} onClick={toggleMulti}>
            <Layers aria-hidden />
            Multi ronda
          </Button>
          {library ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={focused === null}
              onClick={() => {
                setPlaying(false);
                setSaving(currentMoment());
              }}
            >
              <Bookmark aria-hidden />
              Guardar momento
            </Button>
          ) : null}
          {library && focused !== null && entry !== undefined && !multi ? (
            <RoundPlayActions matchId={entry.matchId} doc={focused.doc} round={focused.round} currentTick={() => tickOf(focused, clock.seconds)} />
          ) : null}
        </div>
      </header>

      <div className="grid gap-5 p-4 sm:p-5 @[60rem]:grid-cols-[230px_minmax(0,1fr)_270px]">
        <div className="flex min-w-0 flex-col gap-3">
          {focused === null ? (
            <CentreState state={current} round={entry?.round} onRetry={() => entry !== undefined && retry(entry.matchId)} />
          ) : (
            view.render({
              mode: multi ? 'multi' : 'single',
              rounds: centreRounds,
              clock,
              tickOf,
              layers,
              telestrator: library ? { draft, shown: shownDrawings } : null,
              onSelectView: selectView,
              followSlot,
              onFollowSlot: setFollowSlot,
            })
          )}
        </div>
        <aside aria-label="Filtros, capas y momentos" className="flex min-w-0 flex-col gap-5 @[60rem]:order-first">
          {filters}
          <section className="flex flex-col gap-2">
            <h3 className="text-meta text-fg-3">Capas</h3>
            <LayerPanel
              layers={layers}
              onToggle={(key) => setLayers((value) => toggleLayer(value, key))}
              onShowAll={() => setLayers((value) => showAllPlayers(value))}
            />
          </section>
          {library ? (
            <section className="flex flex-col gap-2">
              <h3 className="text-meta text-fg-3">Momentos guardados</h3>
              <BookmarkPanel
                store={collections}
                momentLabel={momentLabel}
                isCurrent={(item) => entry !== undefined && item.match_id === entry.matchId && item.round === entry.round}
                onOpen={openMoment}
              />
            </section>
          ) : null}
          {focused === null || focused.round.class.reasons.length === 0 ? null : (
            <details className="text-body-sm text-fg-2">
              <summary className="cursor-pointer text-meta text-fg-3 hover:text-fg-1">Por qué se clasificó así</summary>
              <ul className="mt-2 flex flex-col gap-1 font-mono text-meta break-words">
                {focused.round.class.reasons.map((reason) => (
                  <li key={reason}>{reason}</li>
                ))}
              </ul>
            </details>
          )}
        </aside>
        <ReplaySidePanel
          focused={focused}
          nowSeconds={nowSeconds}
          tickOf={tickOf}
          clockSecondsOf={clockSecondsOf}
          layers={layers}
          onTogglePlayer={(slot) => setLayers((value) => togglePlayer(value, slot))}
          follow={view.followsPlayers === true ? { slot: followSlot, onFollow: setFollowSlot } : null}
          onSeek={(seconds) => {
            setPlaying(false);
            seekTo(seconds);
          }}
          notes={
            notesMatchId === undefined || focused === null || focusTimeline === null
              ? null
              : {
                  store: notes,
                  items: noteItems,
                  draft,
                  anchorLabel: `Se ancla a la ronda ${focused.round.number} · ${roundClockLabelFor(focusTimeline, timelineSeconds(focusTimeline, tickOf(focused, nowSeconds)))}`,
                  onCreate: saveNote,
                }
          }
        />
      </div>

      <div className="flex flex-col gap-3 border-t border-border-subtle px-4 py-4 sm:px-5">
        <RoundStrip
          entries={entries}
          index={index}
          onSelect={(at) => select(at)}
          groupLabel={labelOf}
          roundInfo={roundInfo}
          multi={
            multi
              ? {
                  selected: selectedKeys,
                  blocked: (candidate) => {
                    const state = analyses.get(candidate.matchId);
                    if (state?.state === 'failed') return 'Esta partida no tiene análisis';
                    if (state?.state === 'ready' && focused !== null && state.analysis.doc.demo.map !== focused.doc.demo.map) return 'Es de otro mapa';
                    return null;
                  },
                }
              : null
          }
          onToggleMulti={(key) => setMultiKeys((value) => toggleMultiKey(withFocus(value, focusKey), key, focusKey))}
        />
        <TacticalTimelineBar
          timeline={transport}
          events={transportEvents}
          playing={playing}
          speed={speed}
          loop={loop}
          scrubRef={scrubRef}
          clockRef={clockLabelRef}
          onTogglePlay={togglePlay}
          onSpeedChange={setSpeed}
          onToggleLoop={() => setLoop((value) => !value)}
          onSeek={seekTo}
        />
        {winState === null ? null : <WinStrip state={winState} />}
      </div>

      {saving === null || focused === null ? null : (
        <SaveMomentDialog
          store={collections}
          moment={saving}
          momentLabel={momentLabel(saving)}
          onClose={() => setSaving(null)}
        />
      )}
    </section>
  );
}

function CentreState({
  state,
  round,
  onRetry,
}: {
  state: MatchAnalysis | undefined;
  round: number | undefined;
  onRetry: () => void;
}): ReactNode {
  if (state === undefined || state.state === 'loading') {
    return <Skeleton role="status" aria-label="Cargando la partida" className="aspect-square w-full" />;
  }
  if (state.state === 'failed') {
    return (
      <div className="flex aspect-square w-full flex-col items-center justify-center gap-3 rounded-lg border border-border-strong bg-surface-1 px-6 text-center">
        <p className="text-body-sm text-fg-2">{state.message}</p>
        <Button type="button" variant="outline" size="sm" onClick={onRetry}>
          <RefreshCw aria-hidden />
          Reintentar
        </Button>
      </div>
    );
  }
  return (
    <div className="flex aspect-square w-full flex-col items-center justify-center gap-3 rounded-lg border border-border-strong bg-surface-1 px-6 text-center">
      <Radar className="size-6 text-fg-3" aria-hidden />
      <p className="text-body-sm text-fg-2">La ronda {round} no está en el análisis de esta partida.</p>
    </div>
  );
}
