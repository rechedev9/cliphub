'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { ChevronLeft, Maximize, Minimize } from 'lucide-react';
import Link from 'next/link';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import type { Stroke } from '@/lib/api/spatial';
import type { TacticalEvent, TacticalSource } from '@/lib/api/tactical';
import { matchHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { DEMO_DETAILS_ID, MATCH_SUMMARY_TAB, MATCH_TAB_QUERY } from '@/lib/library/rounds';
import { scoreAfter, teamsInScoreOrder, winnerTeamName } from '@/lib/replay/strip-round';
import { centreRound } from '@/lib/replay/centre-round';
import type { CentreRound } from '@/lib/replay/centre-round';
import { ReplayClock } from '@/lib/replay/clock';
import { pickPlayer } from '@/lib/replay/player-focus';
import { roundOutcome } from '@/lib/replay/round-outcome';
import { grenadeSetpos, playerSetpos } from '@/lib/replay/setpos';
import { isMultiLevelMap } from '@/lib/tactical-transform';
import type { LoadedAnalysis } from '@/lib/replay/loaded-analysis';
import { alignedTick, fromLiveSeconds, liveEvents, liveTimeline, toLiveSeconds } from '@/lib/replay/multi-round';
import { frameDue, loopWithin, playStartSeconds, playbackStep } from '@/lib/replay/playback';
import { namesShown } from '@/lib/replay/map-settings';
import { timelineTickShown } from '@/lib/replay/event-kinds';
import { WHOLE_ROUND, isWholeRound, tramoEnd } from '@/lib/replay/tramo';
import type { Tramo } from '@/lib/replay/tramo';
import type { TramoControl } from './tramo-bar';
import { defaultMultiSelection, entryKey, matchesToLoad, stepIndex, toggleMultiKey, withFocus } from '@/lib/replay/playlist';
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
import { useMatchAnalyses } from '@/hooks/use-match-analyses';
import { chooseDrawMode, useTelestratorDraft } from '@/components/tactical/spatial/telestrator';
import type { DrawMode, TelestratorDraft } from '@/components/tactical/spatial/telestrator';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { CentreState } from './centre-state';
import { MapHud } from './map-hud';
import { MapFooter } from './map-footer';
import type { DemoLink } from './map-footer';
import { ReplayToolbar } from './replay-toolbar';
import type { MapCapture } from './gif-export';
import { GifPanel } from './gif-panel';
import { PlaylistBookmark } from './playlist-bookmark';
import { useReplayGif } from './use-replay-gif';
import { useSavedPlaylists } from './use-saved-playlists';
import type { FocusControl } from './replay-players';
import { useMapSettings } from './use-map-settings';
import { ReplayPreferences } from './replay-preferences';
import { RadarCentre } from './radar-centre';
import { ReplaySidePanel } from './replay-side-panel';
import { ReplayTransport, superposeCopy } from './replay-transport';
import { SetposNotice } from './setpos-notice';
import { ReplayStackOverlay, type ReplayStack } from './replay-stack-overlay';
import { ShortcutHelp } from './shortcut-help';
import { useFullscreen } from './use-fullscreen';
import { useSetposCopies } from './use-setpos-copies';
import { useReplayVoice } from './use-replay-voice';
import { VoiceRow, VoiceSyncBar, voiceSyncing } from './voice-bar';

/** The side panels and the notes follow the playhead this many times a second. */
const UI_TICKS_PER_SECOND = 4;

const NO_DRAWINGS: readonly { id: string; strokes: readonly Stroke[] }[] = [];

const EMPTY_TIMELINE = roundTimeline({ tick_start: 0, tick_freeze_end: 0, tick_end: 64, tick_official: 64 }, 64);

export type ReplayWorkspaceProps = {
  /** `library`: strip ids are library match ids, which also enables the playlist bookmark, drawing and the voice. `job`: a Crear job id. */
  source: TacticalSource['kind'];
  /** The rounds in play order. */
  entries: readonly ReplayRef[];
  index: number;
  onIndexChange: (index: number) => void;
  /** An analysis the page already loaded, so it is not fetched twice. */
  seed?: { id: string; analysis: LoadedAnalysis };
  /** The match's name before its analysis arrives, such as `Mirage · NAVI vs FaZe`. */
  groupLabel?: (matchId: string) => string | null;
  /** The day a library match was played, shown by Detalles de la demo under the map. */
  matchDate?: (matchId: string) => string | null;
  /** Shown above the round line, such as a playlist's name. */
  title?: string;
  /** The players in the foreground when the page keeps them (the Replay does, in the URL); without it the workspace keeps its own. */
  players?: FocusControl;
  /** The tramo when the page keeps it in the URL, shared with its other forms; without it the workspace keeps its own. */
  tramo?: TramoControl;
  /** The saved playlist open in the page: B adds to it whatever was picked before. */
  openPlaylistId?: string;
  /** Hangs under the playlist bookmark on the map, such as the open playlist's comment on the round. */
  roundCard?: ReactNode;
  /** Players to put in the foreground by SteamID64, found in each match by it since slots change between demos. */
  focusPlayers?: readonly string[];
  /** Opens with every round of the list superposed, as a grenade's Reproducir or a link with a tramo does. */
  superpose?: boolean;
  /** Zones a finding's link marks on the map, as CS2 place tokens. */
  zones?: readonly string[];
  /** A Volver link over the map, such as the single match's way back to the library. */
  back?: { href: string; label: string };
  /** The scope controls floating over the top left of the map, for a team's rounds. */
  stack?: ReplayStack;
};

/**
 * The replay: the map in the centre, the players on the right, and under them
 * the strip of rounds and the timeline with its event marks. The rounds may come
 * from several matches; each match's analysis loads when the strip reaches it.
 */
export function ReplayWorkspace({
  source,
  entries,
  index,
  onIndexChange,
  seed,
  groupLabel,
  matchDate,
  title,
  players,
  tramo: pageTramo,
  openPlaylistId,
  roundCard,
  focusPlayers,
  superpose = false,
  zones,
  back,
  stack,
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
  const playlist = useSavedPlaylists({ enabled: library, openId: openPlaylistId });
  const [zoomSlot, setZoomSlot] = useState<HTMLDivElement | null>(null);
  const setpos = useSetposCopies();
  const hoverSlot = useRef<number | null>(null);
  const onHoverSlot = useCallback((slot: number | null) => {
    hoverSlot.current = slot;
  }, []);
  const [uiTick, setUiTick] = useState(0);
  const [eventsOpen, setEventsOpen] = useState(false);
  const [help, setHelp] = useState(false);
  const [preferences, setPreferences] = useState(false);
  const preferencesChip = useRef<HTMLButtonElement>(null);
  // The dialog opens inside the replay only in full screen, so entering or leaving it closes the dialog instead of moving it.
  const [rootElement, setRootElement] = useState<HTMLElement | null>(null);
  const setRoot = useCallback((element: HTMLElement | null) => {
    rootRef.current = element;
    setRootElement(element);
  }, []);
  const fullscreen = useFullscreen(rootRef);
  useEffect(() => setPreferences(false), [fullscreen.active]);
  const draft = useTelestratorDraft();

  const entry = entries[index];
  const focusKey = entry === undefined ? '' : entryKey(entry);
  // With players in focus, or a list opened to superpose, every round is superposed; otherwise the cap keeps the map readable.
  const everyRound = focus.length > 0 || superpose;
  const selectedKeys = useMemo(() => {
    if (!multi) return [];
    return everyRound ? entries.map(entryKey) : withFocus(multiKeys, focusKey);
  }, [entries, everyRound, focusKey, multi, multiKeys]);

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
    return centreRound({ key: focusKey, matchId: entry.matchId, doc: loaded.doc, round, frames: loaded.framesFor(round.number), names: namesOn, focused: true });
  }, [entry, focusKey, loaded, namesOn, round]);
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
      out.push(centreRound({ key, matchId: candidate.matchId, doc: state.analysis.doc, round: other, frames: state.analysis.framesFor(other.number), names: namesOn, focused: false }));
    }
    return out;
  }, [analyses, entries, focused, multi, namesOn, selectedKeys]);

  const centreRounds = useMemo(() => (focused === null ? [] : [...overlay, focused]), [focused, overlay]);
  const transport = useMemo<RoundTimeline>(() => {
    if (focused === null) return EMPTY_TIMELINE;
    return multi ? liveTimeline(centreRounds.map((entryRound) => entryRound.timeline), focused.timeline.tickrate) : focused.timeline;
  }, [centreRounds, focused, multi]);
  const transportEvents = useMemo(() => {
    if (focused === null) return [];
    return multi ? liveEvents(focused.timeline, focused.events, transport.durationSeconds) : focused.events;
  }, [focused, multi, transport]);

  const timelineEvents = useMemo(() => transportEvents.filter((entry) => timelineTickShown(entry.event.kind, mapSettings.show)), [mapSettings.show, transportEvents]);

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
  const voice = useReplayVoice({ matchId: library && entry !== undefined ? entry.matchId : null, clock, focused, tickOf, playing, speed });

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
    else setpos.unavailable('Esta ronda no guardó la posición de ese jugador');
  };

  // The throw's own setpos, from the Eventos list: where the thrower stood and looked as the grenade left the hand.
  const copyGrenadeSetpos = (event: TacticalEvent): void => {
    if (focused === null) return;
    const copied = grenadeSetpos({ event, players: focused.doc.players, round: focused.round.number, timeline: focused.timeline });
    if (copied !== null) setpos.copy(copied);
    else setpos.unavailable('La demo no mostró este lanzamiento: no hay setpos');
  };

  // A click on a player or a roster name puts them in the foreground and copies their setpos, as EDGE does.
  const pickSlot = (slot: number, additive: boolean): void => {
    setFocus(pickPlayer(focus, slot, additive));
    copyPlayerSetpos(slot);
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
    // Preferencias del replay is part of the replay wherever it mounts; any other dialog keeps its keys.
    const inPreferences = target instanceof Element && target.closest('[data-replay-preferences]') !== null;
    if (!inPreferences && target instanceof Element && target.closest('[role="dialog"]') !== null) return;
    if (target instanceof HTMLElement && target !== document.body) {
      // Clicking the map focuses the replay or leaves focus on a wrapper around it, such as the tab panel.
      if (root === null || (!inPreferences && !root.contains(target) && !target.contains(root))) return;
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
    } else if (shortcut.startsWith('tool-') || shortcut === 'undo-stroke') {
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

  const roundInfo = useCallback(
    (candidate: ReplayRef) => {
      const state = analyses.get(candidate.matchId);
      if (state?.state !== 'ready') return null;
      const found = state.analysis.round(candidate.round);
      if (found === undefined) return null;
      const { doc } = state.analysis;
      return {
        score: scoreAfter(doc, found).join(':'),
        winner: found.winner,
        outcome: roundOutcome(found.end_reason),
        half: found.half,
        teams: teamsInScoreOrder(doc),
        winnerName: winnerTeamName(doc, found),
      };
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
        const teams = teamsInScoreOrder(doc).join(' vs ');
        return `${prettyMapName(doc.demo.map)} · ${teams}`;
      }
      if (state?.state === 'failed') return 'Partida sin análisis';
      return groupLabel?.(matchId) ?? (state === undefined ? 'Partida' : 'Cargando partida…');
    },
    [analyses, groupLabel],
  );
  const nowSeconds = uiTick / UI_TICKS_PER_SECOND;
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
    onStart: () => setPlaying(false),
  });
  if (entries.length === 0) {
    return (
      <section className="studio-panel p-4 sm:p-5" aria-label="Repetición">
        <p className="self-center text-body text-fg-2">Ninguna ronda cumple el filtro. Quita alguno para volver a ver rondas.</p>
      </section>
    );
  }

  const panelWidths: CSSProperties & { '--replay-right': string } = { '--replay-right': 'clamp(224px, 21.25vw, 272px)' };
  const followedName = followSlot === null || focused === null ? null : (focused.doc.players.find((player) => player.slot === followSlot)?.name ?? null);
  const focusNames = focused === null ? '' : focused.doc.players.filter((player) => focus.includes(player.slot)).map((player) => player.name).join(' y ');
  const superposeText = superposeCopy({ multi, everyRound, anyFocus: focus.length > 0, focusNames, shown: centreRounds.length, total: entries.length });
  // A Crear job has no partida; a library one shows once its date is known, so the link neither dangles nor shifts.
  const demoDate = entry === undefined ? null : (matchDate?.(entry.matchId) ?? null);
  const partidaKnown = matchDate === undefined ? current?.state === 'ready' : demoDate !== null;
  const demoLink: DemoLink | null =
    library && entry !== undefined && partidaKnown
      ? { date: demoDate, href: `${matchHref(entry.matchId)}?${MATCH_TAB_QUERY}=${MATCH_SUMMARY_TAB}#${DEMO_DETAILS_ID}` }
      : null;
  return (
    <section
      ref={setRoot}
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
              'min-[1180px]:grid-cols-[minmax(0,1fr)_minmax(0,var(--replay-right))] min-[1180px]:grid-rows-[minmax(0,1fr)_auto]',
            ],
      )}
    >
      <div
        className={cn(
          'flex min-h-0 min-w-0 flex-col rounded-lg border border-border bg-surface-0',
          // With the export panel open, a short column scrolls whole instead of hiding the panel.
          gif.open ? 'overflow-y-auto overscroll-contain [scrollbar-width:thin]' : 'overflow-hidden',
          fullscreen.active ? 'col-start-1 row-start-1' : 'h-[72vh] min-[1180px]:col-start-1 min-[1180px]:row-start-1 min-[1180px]:h-auto',
        )}
      >
        <header className="flex h-9 shrink-0 items-center gap-3 border-b border-border-subtle bg-surface-1 px-3">
          {back === undefined ? null : (
            <Link href={back.href} className="flex shrink-0 items-center gap-0.5 rounded-sm pr-1 text-meta text-fg-2 outline-none hover:text-fg-1 focus-visible:ring-2 focus-visible:ring-ring">
              <ChevronLeft className="size-3.5" aria-hidden />
              {back.label}
            </Link>
          )}
          <h2 className="flex min-w-0 items-baseline gap-2 truncate">
            {title === undefined ? null : <span className="truncate text-meta text-fg-3">{title}</span>}
            <span className="font-display text-body font-bold text-fg-1">Ronda {entry?.round}</span>
          </h2>
          <div className="ml-auto flex items-center gap-1.5">
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
        <div className={cn('relative flex-1', gif.open ? 'min-h-[200px]' : 'min-h-0')}>
          {focused === null ? (
            <CentreState state={current} round={entry?.round} onRetry={() => entry !== undefined && retry(entry.matchId)} />
          ) : (
            <RadarCentre
              mode={multi ? 'multi' : 'single'}
              rounds={centreRounds}
              clock={clock}
              tickOf={tickOf}
              telestrator={library ? { draft, shown: NO_DRAWINGS } : null}
              zones={zones}
              followSlot={followSlot}
              onFollowSlot={setFollowSlot}
              focus={focus}
              onPickSlot={pickSlot}
              onHoverSlot={onHoverSlot}
              onCopySlot={copyPlayerSetpos}
              onCopyThrow={copyGrenadeSetpos}
              settings={mapSettings}
              barSlot={zoomSlot}
              captureRef={captureRef}
              speakingAt={voice.speakingAt}
            />
          )}
          {focused === null ? null : (
            <MapHud
              focused={focused}
              roundSeconds={timelineSeconds(focused.timeline, tickOf(focused, clock.seconds))}
              eventsOpen={eventsOpen}
              showFeed={mapSettings.show.killfeed}
              showScore={mapSettings.show.scoreline}
              scoreCentred={stack !== undefined}
              onToggleEvents={() => setEventsOpen((value) => !value)}
              onCopyThrow={copyGrenadeSetpos}
              onSeek={(seconds) => {
                setPlaying(false);
                seekTo(clockSecondsOf(focused.timeline, timelineTick(focused.timeline, seconds)));
              }}
            />
          )}
          {stack === undefined || fullscreen.active ? null : <ReplayStackOverlay stack={stack} />}
          {library && focused !== null && entry !== undefined && !multi ? <PlaylistBookmark control={playlist} entry={entry} card={roundCard} /> : null}
          <SetposNotice note={setpos.note} />
        </div>
        {gif.open ? <GifPanel gif={gif} tramo={{ tramo, onChange: setTramo }} multi={multi} speed={speed} onClose={gif.toggle} /> : null}
        <MapFooter demo={demoLink} zoomSlot={setZoomSlot} />
      </div>

      <aside
        aria-label="Jugadores de la ronda"
        className={cn(
          'flex min-h-0 min-w-0 flex-col overflow-hidden rounded-lg border border-border bg-surface-1 p-2',
          fullscreen.active ? 'col-start-2 row-start-1' : 'min-[1180px]:col-start-2 min-[1180px]:row-start-1',
        )}
      >
        <ReplaySidePanel focused={focused} nowSeconds={nowSeconds} tickOf={tickOf} focus={focus} onPickSlot={pickSlot} onCopySlot={copyPlayerSetpos} />
      </aside>

      <ReplayTransport
        fullscreen={fullscreen.active}
        toolbar={
          <ReplayToolbar
            draft={library ? draft : null}
            gif={{ open: gif.open, onToggle: gif.toggle }}
            preferences={{ open: preferences, onOpen: () => setPreferences((value) => !value), triggerRef: preferencesChip }}
            onHelp={() => {
              // Dialogs open outside the full-screen layer, so leave it to show them.
              if (fullscreen.active) fullscreen.toggle();
              setHelp(true);
            }}
            follow={followedName === null ? null : { name: followedName, onStop: () => setFollowSlot(null) }}
          />
        }
        superpose={{ on: multi, disabled: focused === null, ...superposeText, onToggle: toggleMulti }}
        strip={{
          entries,
          index,
          onSelect: (at) => select(at),
          groupLabel: labelOf,
          roundInfo,
          multi: multi
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
            : null,
          onToggleMulti: (key) => setMultiKeys((value) => toggleMultiKey(withFocus(value, focusKey), key, focusKey)),
        }}
        timeline={{
          timeline: transport,
          events: timelineEvents,
          playing,
          speed,
          loop,
          scrubRef,
          clockRef: clockLabelRef,
          onTogglePlay: togglePlay,
          onSpeedChange: setSpeed,
          onToggleLoop: () => setLoop((value) => !value),
          onSeek: seekTo,
          onScrubbingChange: setScrubbing,
          tramo: multi ? { tramo, onChange: setTramo } : undefined,
          timeouts: multi ? undefined : round?.timeouts,
        }}
        voice={{ row: library ? <VoiceRow voice={voice} /> : null, sync: library && voiceSyncing(voice) ? <VoiceSyncBar voice={voice} /> : null }}
      />

      <ReplayPreferences
        open={preferences}
        onOpenChange={setPreferences}
        settings={mapSettings}
        onSettings={setMapSettings}
        twoFloors={focused !== null && isMultiLevelMap(focused.doc.geometry.calibration)}
        drawing={library}
        container={fullscreen.active ? rootElement : null}
        returnFocus={preferencesChip}
      />
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
