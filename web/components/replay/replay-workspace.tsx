'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { Keyboard, Layers, Maximize, Minimize, Radar, RefreshCw } from 'lucide-react';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import type { TacticalSource } from '@/lib/api/tactical';
import { prettyMapName } from '@/lib/format';
import { teamScoreBefore } from '@/lib/library/rounds';
import { centreRound } from '@/lib/replay/centre-round';
import type { CentreRound } from '@/lib/replay/centre-round';
import { ReplayClock } from '@/lib/replay/clock';
import { DEFAULT_LAYERS, showAllPlayers, toggleLayer, togglePlayer } from '@/lib/replay/layers';
import { pickPlayer } from '@/lib/replay/player-focus';
import { roundOutcome } from '@/lib/replay/round-outcome';
import { playerSetpos } from '@/lib/replay/setpos';
import { isMultiLevelMap } from '@/lib/tactical-transform';
import type { ReplayLayers } from '@/lib/replay/layers';
import type { LoadedAnalysis } from '@/lib/replay/loaded-analysis';
import { alignedTick, fromLiveSeconds, liveEvents, liveTimeline, toLiveSeconds } from '@/lib/replay/multi-round';
import { frameDue, loopWithin, playStartSeconds, playbackStep } from '@/lib/replay/playback';
import { namesShown, toggleNames } from '@/lib/replay/map-settings';
import { WHOLE_ROUND, isWholeRound, tramoEnd } from '@/lib/replay/tramo';
import type { Tramo } from '@/lib/replay/tramo';
import type { TramoControl } from './tramo-bar';
import { MULTI_MAX_ROUNDS, defaultMultiSelection, entryKey, matchesToLoad, stepIndex, toggleMultiKey, withFocus } from '@/lib/replay/playlist';
import { replayShortcut, targetTakesKey } from '@/lib/replay/shortcut-keys';
import type { ReplayShortcut } from '@/lib/replay/shortcut-keys';
import {
  SEEK_STEP_SECONDS,
  clampToTimeline,
  roundClockLabelFor,
  roundTimeline,
  seekEventSeconds,
  timelineSeconds,
  timelineTick,
} from '@/lib/tactical-timeline';
import type { RoundTimeline } from '@/lib/tactical-timeline';
import { noteVisibleAt } from '@/lib/tactical-spatial';
import { useMatchAnalyses } from '@/hooks/use-match-analyses';
import type { MatchAnalysis } from '@/hooks/use-match-analyses';
import { RoundPlayActions } from '@/components/stratbook/round-play-actions';
import { chooseDrawMode, useTelestratorDraft } from '@/components/tactical/spatial/telestrator';
import type { DrawMode, TelestratorDraft } from '@/components/tactical/spatial/telestrator';
import { useAnnotations } from '@/components/tactical/spatial/annotation-panel';
import type { AnnotationStore, NoteItem } from '@/components/tactical/spatial/annotation-panel';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { KeyMomentList, KeyMomentMarks } from './key-moments-panel';
import { MapBar } from './map-bar';
import { MapHud } from './map-hud';
import { FoldedRounds, PanelFoldButton, usePanelFolds } from './panel-folds';
import type { MapCapture } from './gif-export';
import { GifPanel } from './gif-panel';
import { PlaylistPicker } from './playlist-picker';
import { useReplayGif } from './use-replay-gif';
import { useSavedPlaylists } from './use-saved-playlists';
import type { FocusControl } from './replay-players';
import { useMapSettings } from './use-map-settings';
import { ReplayLeftPanel } from './replay-left-panel';
import { ReplayTimeline } from './replay-timeline';
import { RadarCentre } from './radar-centre';
import { ReplaySidePanel } from './replay-side-panel';
import { RoundStrip } from './round-strip';
import { SetposNotice } from './setpos-notice';
import { ShortcutHelp } from './shortcut-help';
import { useFullscreen } from './use-fullscreen';
import { useKeyMoments } from './use-key-moments';
import { useRoundPov } from './use-round-pov';
import { useSetposCopies } from './use-setpos-copies';
import { useWinTimelines } from './use-win-timelines';
import { WinStrip } from './win-strip';
import type { WinStripState } from './win-strip';
import { winProbabilityAt, winStripPoints } from '@/lib/replay/win-strip';

/** The side panels and the notes follow the playhead this many times a second. */
const UI_TICKS_PER_SECOND = 4;

const EMPTY_TIMELINE = roundTimeline({ tick_start: 0, tick_freeze_end: 0, tick_end: 64, tick_official: 64 }, 64);

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
  /** Shown above the round line, such as a playlist's name. */
  title?: string;
  /** The players in the foreground when the page keeps them (the Replay does, in the URL); without it the workspace keeps its own. */
  players?: FocusControl;
  /** The tramo when the page keeps it in the URL, shared with its other forms; without it the workspace keeps its own. */
  tramo?: TramoControl;
  /** Where the round's notes and drawings live instead of its match, such as an open playlist (`where` names it). */
  roundNotes?: { store: AnnotationStore; where: string };
  /** The saved playlist open in the page: B adds to it whatever was picked before. */
  openPlaylistId?: string;
  /** Players to put in the foreground by SteamID64, found in each match by it since slots change between demos. */
  focusPlayers?: readonly string[];
  /** Opens with every round of the list superposed, as a grenade's Reproducir does. */
  superpose?: boolean;
};

const NO_MATCHES: readonly string[] = [];

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
  title,
  players,
  tramo: pageTramo,
  roundNotes,
  openPlaylistId,
  focusPlayers,
  superpose = false,
}: ReplayWorkspaceProps): ReactNode {
  const library = source === 'library';
  const clock = useMemo(() => new ReplayClock(), []);
  const rootRef = useRef<HTMLElement>(null);
  const scrubRef = useRef<HTMLInputElement>(null);
  const clockLabelRef = useRef<HTMLSpanElement>(null);
  const pendingTick = useRef<number | null>(null);
  // Whether the user holds the scrubber, and whether they moved the playhead since the last played frame.
  const scrubbingRef = useRef(false);
  const seekedRef = useRef(false);

  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [loop, setLoop] = useState(false);
  const [ownLayers, setLayers] = useState<ReplayLayers>(DEFAULT_LAYERS);
  const [multi, setMulti] = useState(superpose);
  const captureRef = useRef<MapCapture | null>(null);
  // The span the superposed rounds play in; one round alone plays whole.
  const [ownTramo, setOwnTramo] = useState<Tramo>(WHOLE_ROUND);
  const tramo = pageTramo?.tramo ?? ownTramo;
  const setTramo = pageTramo?.onChange ?? setOwnTramo;
  const [multiKeys, setMultiKeys] = useState<string[]>([]);
  const [followSlot, setFollowSlot] = useState<number | null>(null);
  const [ownFocus, setOwnFocus] = useState<number[]>([]);
  const focus = players?.focus ?? ownFocus;
  const setFocus = players?.onChange ?? setOwnFocus;
  const [mapSettings, setMapSettings] = useMapSettings();
  const namesOn = namesShown(mapSettings, multi);
  const layers = useMemo(() => ({ ...ownLayers, names: namesOn }), [namesOn, ownLayers]);
  const [folds, toggleFold] = usePanelFolds();
  const playlist = useSavedPlaylists({ enabled: library, openId: openPlaylistId });
  const [zoomSlot, setZoomSlot] = useState<HTMLDivElement | null>(null);
  const setpos = useSetposCopies();
  const roundPov = useRoundPov();
  const hoverSlot = useRef<number | null>(null);
  const onHoverSlot = useCallback((slot: number | null) => {
    hoverSlot.current = slot;
  }, []);
  const [uiTick, setUiTick] = useState(0);
  const [eventsOpen, setEventsOpen] = useState(false);
  const [help, setHelp] = useState(false);
  const fullscreen = useFullscreen(rootRef);
  const draft = useTelestratorDraft();

  const entry = entries[index];
  const focusKey = entry === undefined ? '' : entryKey(entry);
  // Slots mean a player only inside one match: a focus the workspace keeps itself ends with the match.
  const matchId = entry?.matchId;
  useEffect(() => setOwnFocus([]), [matchId]);
  // With players in focus, or a list opened to superpose, every round is superposed; otherwise the cap keeps the map readable.
  const everyRound = focus.length > 0 || superpose;
  const selectedKeys = useMemo(() => {
    if (!multi) return [];
    return everyRound ? entries.map(entryKey) : withFocus(multiKeys, focusKey);
  }, [entries, everyRound, focusKey, multi, multiKeys]);
  const pickSlot = useCallback((slot: number, additive: boolean) => setFocus(pickPlayer(focus, slot, additive)), [focus, setFocus]);

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
  const loadedDoc = loaded?.doc;
  // Declared after the reset above, so a match that is already loaded gets its slots in the same commit.
  useEffect(() => {
    if (focusPlayers === undefined || focusPlayers.length === 0 || loadedDoc === undefined) return;
    setOwnFocus(loadedDoc.players.filter((player) => focusPlayers.includes(player.steamid64)).map((player) => player.slot).sort((a, b) => a - b));
  }, [focusPlayers, loadedDoc]);

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
    seekedRef.current = tick !== undefined;
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

  const seekTo = useCallback(
    (seconds: number) => {
      seekedRef.current = true;
      clock.set(clampToTimeline(transport, seconds));
    },
    [clock, transport],
  );
  const setScrubbing = useCallback((active: boolean) => {
    scrubbingRef.current = active;
    if (!active) seekedRef.current = true;
  }, []);
  const jumpTo = useCallback(
    (seconds: number) => {
      setPlaying(false);
      seekTo(seconds);
    },
    [seekTo],
  );
  const moments = useKeyMoments({ focused, enabled: layers.moments, transport, clockSecondsOf, onJump: jumpTo });

  const select = useCallback(
    (next: number, tick?: number) => {
      pendingTick.current = tick ?? null;
      onIndexChange(stepIndex(entries.length, next, 0));
    },
    [entries.length, onIndexChange],
  );

  const ready = focused !== null;
  const fps = mapSettings.fps;
  const span = useMemo(
    () => (multi && !isWholeRound(tramo) ? { from: tramo.from, to: Math.min(tramoEnd(tramo), transport.durationSeconds) } : null),
    [multi, tramo, transport.durationSeconds],
  );
  useEffect(() => {
    if (!playing || !ready) return;
    let frame = 0;
    let last = performance.now();
    // Any move of the clock this loop did not make (a note, multi round on or off) counts as a seek too.
    let written = clock.seconds;
    const step = (now: number): void => {
      if (!frameDue(now - last, fps)) {
        frame = requestAnimationFrame(step);
        return;
      }
      const next = playbackStep({
        position: clock.seconds,
        elapsedMs: now - last,
        speed,
        durationSeconds: transport.durationSeconds,
        loop,
        scrubbing: scrubbingRef.current,
        seeked: seekedRef.current || clock.seconds !== written,
        hasNext: !multi && index < entries.length - 1,
      });
      last = now;
      if (span !== null && next.action !== 'hold') {
        seekedRef.current = false;
        clock.set(loopWithin(next.position, span));
        written = clock.seconds;
        frame = requestAnimationFrame(step);
        return;
      }
      if (next.action !== 'hold') {
        seekedRef.current = false;
        clock.set(next.position);
        written = clock.seconds;
      }
      // A playlist plays on: the next round starts where this one ends.
      if (next.action === 'next') select(index + 1);
      else if (next.action === 'stop') setPlaying(false);
      else frame = requestAnimationFrame(step);
    };
    frame = requestAnimationFrame(step);
    return () => cancelAnimationFrame(frame);
  }, [playing, ready, speed, loop, transport, clock, multi, index, entries.length, select, span, fps]);

  const togglePlay = useCallback(() => {
    setPlaying((value) => {
      if (value) return false;
      seekedRef.current = false;
      clock.set(playStartSeconds(clock.seconds, transport.durationSeconds));
      return true;
    });
  }, [clock, transport]);

  const copyPlayerSetpos = (slot: number): void => {
    if (focused === null) return;
    const { frames, doc, timeline } = focused;
    const copied = playerSetpos({ frames, players: doc.players, round: focused.round.number, timeline, slot, tick: tickOf(focused, clock.seconds) });
    if (copied !== null) setpos.copy(copied);
  };

  const recordPov = (slot: number): void => {
    const player = focused?.doc.players.find((candidate) => candidate.slot === slot);
    if (focused !== null && player !== undefined) roundPov.record({ matchId: focused.matchId, round: focused.round.number, steamId: player.steamid64, name: player.name });
  };

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
    if (event.defaultPrevented) return;
    const shortcut = replayShortcut({ key: event.key, shiftKey: event.shiftKey, ctrlKey: event.ctrlKey, altKey: event.altKey, metaKey: event.metaKey, altGraph: event.getModifierState('AltGraph') });
    if (shortcut === null) return;
    const target = event.target;
    const root = rootRef.current;
    if (target instanceof HTMLElement && target !== document.body) {
      // Clicking the map focuses the replay or leaves focus on a wrapper around it, such as the tab panel.
      if (root === null || (!root.contains(target) && !target.contains(root))) return;
      const inputType = target.tagName === 'INPUT' ? target.getAttribute('type') : null;
      const keyboardFocus = pointerTarget.current === null || !target.contains(pointerTarget.current);
      if (!targetTakesKey({ tag: target.tagName, inputType, editable: target.isContentEditable, keyboardFocus }, shortcut)) return;
    }
    if (shortcut === 'escape') {
      if (eventsOpen) setEventsOpen(false);
      else if (followSlot !== null || focus.length > 0) {
        setFollowSlot(null);
        setFocus([]);
      } else if (fullscreen.active) fullscreen.toggle();
      return;
    }
    event.preventDefault();
    if (shortcut === 'help') {
      // Dialogs open outside the full-screen layer, so leave it to show them.
      if (fullscreen.active) fullscreen.toggle();
      setHelp((value) => !value);
    } else if (shortcut === 'fullscreen') fullscreen.toggle();
    else if (shortcut === 'multi') toggleMulti();
    else if (shortcut === 'follow') {
      const slot = focus[0] ?? followSlot;
      setFollowSlot(followSlot !== null && followSlot === slot ? null : slot);
    } else if (shortcut === 'fold-left' || shortcut === 'fold-right') toggleFold(shortcut === 'fold-left' ? 'left' : 'right');
    else if (shortcut.startsWith('tool-') || shortcut === 'undo-stroke') {
      if (library) drawShortcut(draft, shortcut);
    } else if (shortcut === 'gif') {
      // The first G shows the tramo that goes in; the second records it.
      if (gif.open) gif.start();
      else gif.toggle();
    } else if (shortcut === 'playlist') {
      if (library && entry !== undefined) playlist.toggle(entry);
    } else if (shortcut === 'setpos') {
      const slot = hoverSlot.current ?? (focus.length === 1 ? focus[0] : undefined);
      if (slot !== undefined) copyPlayerSetpos(slot);
    } else if (shortcut === 'play') {
      spaceTookButton.current = target instanceof HTMLButtonElement || (target instanceof HTMLInputElement && target.type === 'checkbox');
      togglePlay();
    } else if (shortcut === 'round-prev' || shortcut === 'round-next') select(index + (shortcut === 'round-next' ? 1 : -1));
    else if (shortcut === 'event-back' || shortcut === 'event-forward') {
      const at = seekEventSeconds(transportEvents, clock.seconds, shortcut === 'event-forward' ? 1 : -1);
      if (at !== undefined) seekTo(at);
    } else seekTo(clock.seconds + (shortcut === 'seek-forward' ? 1 : -1) * SEEK_STEP_SECONDS);
  };
  // A button activates on the Space keyup, so a Space the replay used must not also click it.
  const spaceTookButton = useRef(false);
  // The element last pressed with the mouse: a control focused that way gives Space to play or pause.
  const pointerTarget = useRef<Element | null>(null);
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent): void => keyHandler.current(event);
    const onKeyUp = (event: KeyboardEvent): void => {
      if (event.key !== ' ' || !spaceTookButton.current) return;
      spaceTookButton.current = false;
      event.preventDefault();
    };
    const onPointerDown = (event: PointerEvent): void => {
      pointerTarget.current = event.target instanceof Element ? event.target : null;
    };
    window.addEventListener('keydown', onKeyDown);
    window.addEventListener('keyup', onKeyUp);
    window.addEventListener('pointerdown', onPointerDown, true);
    return () => {
      window.removeEventListener('keydown', onKeyDown);
      window.removeEventListener('keyup', onKeyUp);
      window.removeEventListener('pointerdown', onPointerDown, true);
    };
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
      if (found === undefined) return null;
      return { score: teamScoreBefore(state.analysis.doc, found).join(':'), winner: found.winner, outcome: roundOutcome(found.end_reason), half: found.half };
    },
    [analyses],
  );
  const labelOf = useCallback(
    (matchId: string) => {
      // The page knows the match's date; the analysis alone only knows the map and the teams.
      const named = groupLabel?.(matchId) ?? null;
      if (named !== null) return named;
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
  const nowSeconds = uiTick / UI_TICKS_PER_SECOND;
  const notesMatchId = library && entry !== undefined ? entry.matchId : undefined;
  const noteMatchIds = useMemo(() => (notesMatchId === undefined ? [] : [notesMatchId]), [notesMatchId]);
  const matchNotes = useAnnotations(roundNotes === undefined ? noteMatchIds : NO_MATCHES, entry?.round);
  const notes = roundNotes?.store ?? matchNotes;
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
  const gif = useReplayGif({
    captureRef,
    focused,
    multi,
    rounds: centreRounds.length,
    transport,
    events: transportEvents,
    tramo,
    speed,
    draft: library ? draft.strokes : [],
    notes: focusTimeline === null ? [] : (notes.notes ?? []).map((note) => ({ at: clockSecondsOf(focusTimeline, note.tick), strokes: note.strokes })),
    onStart: () => setPlaying(false),
  });

  if (entries.length === 0) {
    return (
      <section className="studio-panel grid gap-5 p-4 sm:p-5 lg:grid-cols-[260px_minmax(0,1fr)]" aria-label="Repetición">
        {filters === undefined ? null : <div className="flex flex-col gap-3">{filters}</div>}
        <p className="self-center text-body text-fg-2">Ninguna ronda cumple el filtro. Quita alguno para volver a ver rondas.</p>
      </section>
    );
  }

  const panelWidths: CSSProperties & { '--replay-left': string; '--replay-right': string } = {
    '--replay-left': folds.left ? '2.75rem' : '236px',
    '--replay-right': folds.right ? '2.75rem' : '288px',
  };
  const followedName = followSlot === null || focused === null ? null : (focused.doc.players.find((player) => player.slot === followSlot)?.name ?? null);
  const focusNames = focused === null ? '' : focused.doc.players.filter((player) => focus.includes(player.slot)).map((player) => player.name).join(' y ');
  let superposeNote = `hasta ${MULTI_MAX_ROUNDS} rondas`;
  if (everyRound && focus.length === 0) superposeNote = multi ? `${centreRounds.length} de ${entries.length}` : 'todas las de la lista';
  else if (everyRound) superposeNote = multi ? `${centreRounds.length} de ${focusNames}` : `todas las de ${focusNames}`;
  else if (multi) superposeNote = `${centreRounds.length} de ${MULTI_MAX_ROUNDS} como máximo`;
  let superposeHint = `Superpone todas las rondas del alcance con ${focusNames} en primer plano, desde el final del tiempo de compra`;
  if (everyRound && focus.length === 0) superposeHint = 'Superpone todas las rondas de la lista desde el final del tiempo de compra';
  else if (!everyRound) superposeHint = `Superpone hasta ${MULTI_MAX_ROUNDS} rondas desde el final del tiempo de compra: pulsa una ronda para sumarla. Elige un jugador para ver todas las suyas`;
  const roundActions = !library ? null : (
    <div className="flex flex-wrap gap-1">
      {focused !== null && entry !== undefined && !multi ? (
        <RoundPlayActions
          matchId={entry.matchId}
          doc={focused.doc}
          round={focused.round}
          currentTick={() => tickOf(focused, clock.seconds)}
          playlist={<PlaylistPicker control={playlist} entry={entry} />}
        />
      ) : null}
    </div>
  );
  return (
    <section
      ref={rootRef}
      data-replay-workspace=""
      aria-label="Repetición"
      // Takes the focus a click on the map or a panel would drop, so the shortcuts keep working.
      tabIndex={-1}
      data-fullscreen={fullscreen.active ? '' : undefined}
      style={panelWidths}
      className={cn(
        'grid grid-cols-1 gap-2 outline-none',
        fullscreen.active
          ? 'fixed inset-0 m-0 h-dvh max-h-none w-dvw max-w-none grid-cols-[minmax(0,1fr)_var(--replay-right)] grid-rows-[minmax(0,1fr)_auto] border-0 bg-surface-0 p-2'
          : [
              'min-[1180px]:h-[calc(100dvh-var(--shell-strip-height)-var(--replay-above,0px)-1.5rem)] min-[1180px]:min-h-[460px]',
              'min-[1180px]:grid-cols-[var(--replay-left)_minmax(0,1fr)_var(--replay-right)] min-[1180px]:grid-rows-[minmax(0,1fr)_auto]',
            ],
      )}
    >
      <div
        className={cn(
          'flex min-h-0 min-w-0 flex-col overflow-hidden rounded-lg border border-border bg-surface-0',
          fullscreen.active ? 'col-start-1 row-start-1' : 'h-[72vh] min-[1180px]:col-start-2 min-[1180px]:row-start-1 min-[1180px]:h-auto',
        )}
      >
        <header className="flex h-9 shrink-0 items-center gap-3 border-b border-border-subtle bg-surface-1 px-3">
          <h2 className="flex min-w-0 items-baseline gap-2 truncate">
            {title === undefined ? null : <span className="truncate text-meta text-fg-3">{title}</span>}
            <span className="font-display text-body font-bold text-fg-1">Ronda {entry?.round}</span>
          </h2>
          <div className="ml-auto flex items-center gap-1.5">
            <Button type="button" size="icon-xs" variant="ghost" aria-label="Atajos de teclado" title="Atajos de teclado (?)" onClick={() => {
                if (fullscreen.active) fullscreen.toggle();
                setHelp(true);
              }}>
              <Keyboard aria-hidden />
            </Button>
            <Button
              type="button"
              size="icon-xs"
              variant="ghost"
              aria-pressed={fullscreen.active}
              aria-label={fullscreen.active ? 'Salir de pantalla completa' : 'Pantalla completa'}
              title={fullscreen.active ? 'Salir de pantalla completa (P)' : 'Pantalla completa (P)'}
              onClick={fullscreen.toggle}
            >
              {fullscreen.active ? <Minimize aria-hidden /> : <Maximize aria-hidden />}
            </Button>
          </div>
        </header>
        <MapBar
          settings={mapSettings}
          onSettings={setMapSettings}
          twoFloors={focused !== null && isMultiLevelMap(focused.doc.geometry.calibration)}
          layers={layers}
          onToggleLayer={(key) => (key === 'names' ? setMapSettings(toggleNames(mapSettings, multi)) : setLayers((value) => toggleLayer(value, key)))}
          onTrailSeconds={(trailSeconds) => setLayers((value) => ({ ...value, trailSeconds }))}
          draft={library ? draft : null}
          gif={{ open: gif.open, onToggle: gif.toggle }}
          follow={followedName === null ? null : { name: followedName, onStop: () => setFollowSlot(null) }}
          zoomSlot={setZoomSlot}
        />
        {gif.open ? <GifPanel gif={gif} tramo={{ tramo, onChange: setTramo }} multi={multi} speed={speed} onClose={gif.toggle} /> : null}
        <div className="relative min-h-0 flex-1">
          {focused === null ? (
            <CentreState state={current} round={entry?.round} onRetry={() => entry !== undefined && retry(entry.matchId)} />
          ) : (
            <RadarCentre
              mode={multi ? 'multi' : 'single'}
              rounds={centreRounds}
              clock={clock}
              tickOf={tickOf}
              layers={layers}
              telestrator={library ? { draft, shown: shownDrawings } : null}
              followSlot={followSlot}
              onFollowSlot={setFollowSlot}
              focus={focus}
              onPickSlot={pickSlot}
              onHoverSlot={onHoverSlot}
              onCopySlot={copyPlayerSetpos}
              settings={mapSettings}
              barSlot={zoomSlot}
              captureRef={captureRef}
            />
          )}
          {focused === null ? null : (
            <MapHud
              focused={focused}
              roundSeconds={timelineSeconds(focused.timeline, tickOf(focused, clock.seconds))}
              eventsOpen={eventsOpen}
              showFeed={mapSettings.show.killfeed}
              onToggleEvents={() => setEventsOpen((value) => !value)}
              onSeek={(seconds) => {
                setPlaying(false);
                seekTo(clockSecondsOf(focused.timeline, timelineTick(focused.timeline, seconds)));
              }}
            />
          )}
          <SetposNotice note={setpos.note} />
        </div>
      </div>

      <aside
        aria-label="Ronda y filtros"
        className={cn(
          'flex min-h-0 min-w-0 flex-col gap-2 overflow-y-auto overscroll-contain rounded-lg border border-border bg-surface-1 [scrollbar-width:thin] min-[1180px]:col-start-1 min-[1180px]:row-start-1',
          folds.left ? 'items-center p-1' : 'p-3',
          fullscreen.active && 'hidden',
        )}
      >
        <PanelFoldButton side="left" folded={folds.left} onToggle={() => toggleFold('left')} className={folds.left ? undefined : 'self-end -mt-1 -mr-1 -mb-9 relative z-10'} />
        {folds.left ? <FoldedRounds entries={entries} index={index} onSelect={(at) => select(at)} /> : null}
        {folds.left ? null : (
        <ReplayLeftPanel
          focused={focused}
          actions={roundActions}
          filters={filters}
          moments={layers.moments && focused !== null ? { count: moments.length, list: <KeyMomentList moments={moments} /> } : null}
        />
        )}
      </aside>

      <aside
        aria-label="Jugadores y notas de la ronda"
        className={cn(
          'flex min-h-0 min-w-0 flex-col gap-1 overflow-hidden rounded-lg border border-border bg-surface-1',
          folds.right ? 'items-center overflow-y-auto p-1 [scrollbar-width:thin]' : 'p-2',
          fullscreen.active ? 'col-start-2 row-start-1' : 'min-[1180px]:col-start-3 min-[1180px]:row-start-1',
        )}
      >
        <PanelFoldButton side="right" folded={folds.right} onToggle={() => toggleFold('right')} className={folds.right ? undefined : 'self-end -mb-8 relative z-10'} />
        <ReplaySidePanel
          folded={folds.right}
          focused={focused}
          nowSeconds={nowSeconds}
          tickOf={tickOf}
          layers={layers}
          focus={focus}
          onPickSlot={pickSlot}
          onFocus={setFocus}
          onTogglePlayer={(slot) => setLayers((value) => togglePlayer(value, slot))}
          onShowAll={() => setLayers((value) => showAllPlayers(value))}
          setpos={setpos.list}
          onCopySetpos={setpos.copyAgain}
          pov={library ? { runs: roundPov.runs, onRecord: recordPov } : null}
          notes={
            notesMatchId === undefined || focused === null || focusTimeline === null
              ? null
              : {
                  store: notes,
                  items: noteItems,
                  draft,
                  anchorLabel: `${roundNotes === undefined ? 'Se ancla a' : `Se guarda en ${roundNotes.where},`} la ronda ${focused.round.number} · ${roundClockLabelFor(focusTimeline, timelineSeconds(focusTimeline, tickOf(focused, nowSeconds)))}`,
                  onCreate: saveNote,
                }
          }
        />
      </aside>

      <div
        className={cn(
          'flex min-w-0 flex-col gap-1.5 rounded-lg border border-border bg-surface-1 px-3 py-2',
          fullscreen.active ? 'col-span-2 row-start-2' : 'min-[1180px]:col-span-3 min-[1180px]:row-start-2',
        )}
      >
        <div className="flex min-w-0 items-end gap-3">
          <Button
            type="button"
            size="xs"
            variant={multi ? 'outline-primary' : 'outline'}
            aria-pressed={multi}
            disabled={focused === null}
            onClick={toggleMulti}
            className="h-10 shrink-0"
            title={superposeHint}
          >
            <Layers aria-hidden />
            <span className="flex flex-col items-start leading-tight">
              Superponer
              <span className="text-meta font-normal text-fg-3">{superposeNote}</span>
            </span>
          </Button>
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
                    locked: everyRound,
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
        </div>
        <ReplayTimeline
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
          onScrubbingChange={setScrubbing}
          tramo={multi ? { tramo, onChange: setTramo } : undefined}
          timeouts={multi ? undefined : round?.timeouts}
          below={
            moments.length === 0 && winState === null ? undefined : (
              <>
                <KeyMomentMarks moments={moments} />
                {winState === null ? null : <WinStrip state={winState} />}
              </>
            )
          }
        />
      </div>

      <ShortcutHelp open={help} onOpenChange={setHelp} />
    </section>
  );
}

const DRAW_KEYS: Partial<Record<ReplayShortcut, DrawMode>> = { 'tool-move': 'move', 'tool-draw': 'draw', 'tool-arrow': 'arrow', 'tool-erase': 'erase' };

/** H, D, A and E pick the map's tool as its buttons do; T turns timed strokes on or off; Z undoes. */
function drawShortcut(draft: TelestratorDraft, shortcut: ReplayShortcut): void {
  const mode = DRAW_KEYS[shortcut];
  if (mode !== undefined) chooseDrawMode(draft, mode);
  else if (shortcut === 'tool-timed') draft.setTool({ ...draft.tool, timed: !draft.tool.timed });
  else if (shortcut === 'undo-stroke') draft.undo();
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
    return <Skeleton role="status" aria-label="Cargando la partida" className="absolute inset-3" />;
  }
  if (state.state === 'failed') {
    return (
      <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 px-6 text-center">
        <p className="text-body-sm text-fg-2">{state.message}</p>
        <Button type="button" variant="outline" size="sm" onClick={onRetry}>
          <RefreshCw aria-hidden />
          Reintentar
        </Button>
      </div>
    );
  }
  return (
    <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 px-6 text-center">
      <Radar className="size-6 text-fg-3" aria-hidden />
      <p className="text-body-sm text-fg-2">La ronda {round} no está en el análisis de esta partida.</p>
    </div>
  );
}
