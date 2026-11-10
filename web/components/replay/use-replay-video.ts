'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type RefObject } from 'react';
import { createReplayVideo, readReplayVideo, replayVideoMessage } from '@/lib/api/replay-video';
import type { Stroke } from '@/lib/api/spatial';
import { prettyMapName } from '@/lib/format';
import type { CentreRound } from '@/lib/replay/centre-round';
import { gifCaption, gifSpan, plantSecondsOf } from '@/lib/replay/replay-gif';
import type { GifSpan } from '@/lib/replay/replay-gif';
import { VIDEO_FPS, VIDEO_MAX_SECONDS, replayVideoFileName, videoFrameTimes, videoSeconds } from '@/lib/replay/replay-video';
import { pickVideoTarget, writeVideoTarget } from '@/lib/replay/save-gif';
import type { Tramo } from '@/lib/replay/tramo';
import { noteVisibleAt } from '@/lib/tactical-spatial';
import type { RoundTimeline, TimelineEvent } from '@/lib/tactical-timeline';
import type { GifOverlay, MapCapture } from './gif-export';
import { renderReplayVideo } from './video-export';

export type VideoState =
  | { kind: 'idle' }
  | { kind: 'picking' }
  | { kind: 'rendering'; percent: number }
  | { kind: 'encoding' }
  | { kind: 'saving' }
  | { kind: 'done'; message: string }
  | { kind: 'failed'; message: string };

/** The match's team voice as the export can use it: null when it has no synced recording. */
export type VideoVoice = { synced: boolean; tracks: readonly number[] } | null;

export type ReplayVideo = {
  state: VideoState;
  busy: boolean;
  seconds: number;
  blocked: string | null;
  /** Whether "Incluir voz del equipo" can be ticked, and why not. */
  voice: { available: boolean; hint: string | null };
  includeVoice: boolean;
  setIncludeVoice: (on: boolean) => void;
  start: () => void;
  cancel: () => void;
};

const NO_PLANT = 'Esta ronda no tiene plant: elige otro tramo.';
const OUTSIDE = 'El tramo queda fuera de esta ronda: elige otro.';
const NOT_READY = 'El mapa todavía no se puede dibujar.';
const NOT_LIBRARY = 'El MP4 se exporta desde las partidas de la biblioteca.';
const TOO_LONG = `El MP4 dura como mucho ${VIDEO_MAX_SECONDS} s: elige un tramo más corto o más velocidad.`;

function isBusy(state: VideoState): boolean {
  return state.kind === 'picking' || state.kind === 'rendering' || state.kind === 'encoding' || state.kind === 'saving';
}

function download(bytes: Uint8Array<ArrayBuffer>, fileName: string): void {
  const url = URL.createObjectURL(new Blob([bytes], { type: 'video/mp4' }));
  const link = document.createElement('a');
  link.href = url;
  link.download = fileName;
  link.click();
  window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
}

function voiceHint(voice: VideoVoice): string | null {
  if (voice === null) return 'Esta partida no tiene voz del equipo.';
  if (!voice.synced) return 'Sincroniza la voz del equipo para incluirla.';
  if (voice.tracks.length === 0) return 'Todas las pistas de voz están silenciadas.';
  return null;
}

/**
 * The MP4 of the tramo on screen: the browser paints the frames like the GIF, at the map's resolution,
 * and the orchestrator encodes them with the focused round's team voice when asked.
 */
export function useReplayVideo(source: {
  captureRef: RefObject<MapCapture | null>;
  focused: CentreRound | null;
  /** Library match ids only: the export lives under the library's routes. */
  library: boolean;
  multi: boolean;
  rounds: number;
  transport: RoundTimeline;
  events: readonly TimelineEvent[];
  tramo: Tramo;
  speed: number;
  tickOf: (round: CentreRound, seconds: number) => number;
  voice: VideoVoice;
  draft: readonly Stroke[];
  notes: readonly { at: number; strokes: readonly Stroke[] }[];
  onStart: () => void;
}): ReplayVideo {
  const { captureRef, focused, library, multi, rounds, transport, events, tramo, speed, tickOf, voice, draft, notes, onStart } = source;
  const [state, setState] = useState<VideoState>({ kind: 'idle' });
  const [wantVoice, setIncludeVoice] = useState(true);
  const abortRef = useRef<AbortController | null>(null);
  useEffect(() => () => abortRef.current?.abort(), []);

  const freezeSeconds = multi || focused === null ? 0 : focused.timeline.freezeSeconds;
  const plantSeconds = plantSecondsOf(events);
  const span = useMemo<GifSpan | null>(
    () => (focused === null ? null : gifSpan({ tramo, freezeSeconds, durationSeconds: transport.durationSeconds, plantSeconds })),
    [focused, freezeSeconds, plantSeconds, tramo, transport.durationSeconds],
  );
  const seconds = span === null ? 0 : videoSeconds(span, speed);
  let blocked: string | null = null;
  if (!library) blocked = NOT_LIBRARY;
  else if (focused === null) blocked = NOT_READY;
  else if (span === null) blocked = tramo.fromPlant === true && plantSeconds === null ? NO_PLANT : OUTSIDE;
  else if (seconds > VIDEO_MAX_SECONDS) blocked = TOO_LONG;
  const hint = voiceHint(voice);
  const includeVoice = wantVoice && hint === null;
  const busy = isBusy(state);

  const start = useCallback(() => {
    if (busy) return;
    const capture = captureRef.current;
    if (capture === null || focused === null || span === null || blocked !== null) {
      setState({ kind: 'failed', message: blocked ?? NOT_READY });
      return;
    }
    const roundsLabel = multi ? `${rounds} rondas` : `R${focused.round.number}`;
    const fileName = replayVideoFileName({
      map: prettyMapName(focused.doc.demo.map),
      teams: focused.doc.teams.map((team) => team.name || team.key),
      rounds: roundsLabel,
      span,
      freezeSeconds,
    });
    const shownDraft = [...draft];
    const shownNotes = [...notes];
    const overlay: GifOverlay = {
      strokesAt: (at) => [...shownDraft, ...shownNotes.filter((note) => noteVisibleAt(note.at, at)).flatMap((note) => note.strokes)],
      captionAt: (at) => gifCaption({ rounds: roundsLabel, events, seconds: at, freezeSeconds }),
    };
    const times = videoFrameTimes(span, speed);
    const fromTick = Math.max(0, tickOf(focused, span.from));
    const toTick = Math.max(fromTick + 1, tickOf(focused, span.to));
    const tracks = voice === null ? [] : [...voice.tracks];
    const controller = new AbortController();
    abortRef.current = controller;
    onStart();
    setState({ kind: 'picking' });
    // The bridge is called before the first await, while the click still counts as the user's.
    const picked = pickVideoTarget(fileName);
    void (async () => {
      try {
        const target = await picked;
        if (target.kind === 'canceled') return setState({ kind: 'idle' });
        if (target.kind === 'failed') return setState({ kind: 'failed', message: target.message });
        setState({ kind: 'rendering', percent: 0 });
        let shown = 0;
        const drawn = await renderReplayVideo({
          capture,
          times,
          overlay,
          signal: controller.signal,
          onProgress: (done) => {
            const percent = Math.floor((done / times.length) * 100);
            if (percent === shown) return;
            shown = percent;
            setState({ kind: 'rendering', percent });
          },
        });
        setState({ kind: 'encoding' });
        const spec = {
          fps: VIDEO_FPS,
          width: drawn.width,
          height: drawn.height,
          speed: speed > 0 ? speed : 1,
          include_voice: includeVoice,
          voice_tracks: includeVoice ? tracks : [],
          segments: [{ round: focused.round.number, from_tick: fromTick, to_tick: toTick, frames: drawn.frames.length }],
        };
        const made = await createReplayVideo({ matchId: focused.matchId, spec, frames: drawn.frames, signal: controller.signal });
        const bytes = await readReplayVideo(made.video_id, controller.signal);
        const heard = made.voice ? ' con la voz del equipo' : '';
        if (target.kind === 'download') {
          download(bytes, fileName);
          return setState({ kind: 'done', message: `MP4 descargado${heard}: ${fileName}` });
        }
        setState({ kind: 'saving' });
        const written = await writeVideoTarget({ token: target.token, bytes });
        setState(written.kind === 'saved' ? { kind: 'done', message: `MP4 guardado${heard}: ${written.name === '' ? fileName : written.name}` } : { kind: 'failed', message: written.message });
      } catch (error) {
        if (controller.signal.aborted) return setState({ kind: 'idle' });
        console.error('Replay: the MP4 could not be made', error);
        setState({ kind: 'failed', message: replayVideoMessage(error) });
      } finally {
        if (abortRef.current === controller) abortRef.current = null;
      }
    })();
  }, [blocked, busy, captureRef, draft, events, focused, freezeSeconds, includeVoice, multi, notes, onStart, rounds, span, speed, tickOf, voice]);

  const cancel = useCallback(() => abortRef.current?.abort(), []);
  return { state, busy, seconds, blocked, voice: { available: hint === null, hint }, includeVoice, setIncludeVoice, start, cancel };
}
