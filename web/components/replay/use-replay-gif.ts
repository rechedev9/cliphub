'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type RefObject } from 'react';
import type { Stroke } from '@/lib/api/spatial';
import { prettyMapName } from '@/lib/format';
import type { CentreRound } from '@/lib/replay/centre-round';
import { gifCaption, gifFrames, gifSpan, plantSecondsOf, replayGifFileName } from '@/lib/replay/replay-gif';
import type { GifSpan } from '@/lib/replay/replay-gif';
import { pickGifTarget, writeGifTarget } from '@/lib/replay/save-gif';
import type { Tramo } from '@/lib/replay/tramo';
import type { RoundTimeline, TimelineEvent } from '@/lib/tactical-timeline';
import { encodeReplayGif } from './gif-export';
import type { GifOverlay, MapCapture } from './gif-export';

export type GifState =
  | { kind: 'idle' }
  | { kind: 'picking' }
  | { kind: 'recording'; percent: number }
  | { kind: 'saving' }
  | { kind: 'done'; message: string }
  | { kind: 'failed'; message: string };

export type ReplayGif = {
  open: boolean;
  toggle: () => void;
  state: GifState;
  busy: boolean;
  /** How long the GIF plays, in seconds; 0 when there is nothing to record. */
  seconds: number;
  /** Why the tramo cannot be recorded, or null when it can. */
  blocked: string | null;
  /** Records the tramo; call it from the click or the key press, which the desktop's save dialog needs. */
  start: () => void;
  cancel: () => void;
};

const NO_PLANT = 'Esta ronda no tiene plant: elige otro tramo.';
const OUTSIDE = 'El tramo queda fuera de esta ronda: elige otro.';
const NOT_READY = 'El mapa todavía no se puede dibujar.';
const FAILED = 'No se pudo crear el GIF. Prueba con un tramo más corto.';

function isBusy(state: GifState): boolean {
  return state.kind === 'picking' || state.kind === 'recording' || state.kind === 'saving';
}

function download(bytes: Uint8Array<ArrayBuffer>, fileName: string): void {
  const url = URL.createObjectURL(new Blob([bytes], { type: 'image/gif' }));
  const link = document.createElement('a');
  link.href = url;
  link.download = fileName;
  link.click();
  window.setTimeout(() => URL.revokeObjectURL(url), 30_000);
}

/**
 * The GIF of the tramo on screen: one round, or every superposed one, played
 * at the transport's speed with the map's framing, layers and drawings.
 */
export function useReplayGif(source: {
  captureRef: RefObject<MapCapture | null>;
  focused: CentreRound | null;
  multi: boolean;
  /** Rounds on the map, 1 alone. */
  rounds: number;
  transport: RoundTimeline;
  events: readonly TimelineEvent[];
  tramo: Tramo;
  speed: number;
  /** The drawing being made, shown all along, and the saved ones at their moment on the transport's clock. */
  draft: readonly Stroke[];
  /** Called as the recording starts, so playback stops. */
  onStart: () => void;
}): ReplayGif {
  const { captureRef, focused, multi, rounds, transport, events, tramo, speed, draft, onStart } = source;
  const [open, setOpen] = useState(false);
  const [state, setState] = useState<GifState>({ kind: 'idle' });
  const abortRef = useRef<AbortController | null>(null);
  useEffect(() => () => abortRef.current?.abort(), []);

  const freezeSeconds = multi || focused === null ? 0 : focused.timeline.freezeSeconds;
  const plantSeconds = plantSecondsOf(events);
  const span = useMemo<GifSpan | null>(
    () => (focused === null ? null : gifSpan({ tramo, freezeSeconds, durationSeconds: transport.durationSeconds, plantSeconds })),
    [focused, freezeSeconds, plantSeconds, tramo, transport.durationSeconds],
  );
  let blocked: string | null = null;
  if (focused === null) blocked = NOT_READY;
  else if (span === null) blocked = tramo.fromPlant === true && plantSeconds === null ? NO_PLANT : OUTSIDE;
  const busy = isBusy(state);

  const start = useCallback(() => {
    if (busy) return;
    setOpen(true);
    const capture = captureRef.current;
    if (capture === null || focused === null || span === null) {
      setState({ kind: 'failed', message: blocked ?? NOT_READY });
      return;
    }
    const roundsLabel = multi ? `${rounds} rondas` : `R${focused.round.number}`;
    const fileName = replayGifFileName({
      map: prettyMapName(focused.doc.demo.map),
      teams: focused.doc.teams.map((team) => team.name || team.key),
      rounds: roundsLabel,
      span,
      freezeSeconds,
    });
    const shownDraft = [...draft];
    const overlay: GifOverlay = {
      strokesAt: () => shownDraft,
      captionAt: (seconds) => gifCaption({ rounds: roundsLabel, events, seconds, freezeSeconds }),
    };
    const frames = gifFrames(span, speed);
    const controller = new AbortController();
    abortRef.current = controller;
    onStart();
    setState({ kind: 'picking' });
    // The bridge is called before the first await, while the click still counts as the user's.
    const picked = pickGifTarget(fileName);
    void (async () => {
      try {
        const target = await picked;
        if (target.kind === 'canceled') return setState({ kind: 'idle' });
        if (target.kind === 'failed') return setState({ kind: 'failed', message: target.message });
        setState({ kind: 'recording', percent: 0 });
        let shown = 0;
        const bytes = await encodeReplayGif({
          capture,
          frames,
          overlay,
          signal: controller.signal,
          onProgress: (done) => {
            const percent = Math.floor((done / frames.length) * 100);
            if (percent === shown) return;
            shown = percent;
            setState({ kind: 'recording', percent });
          },
        });
        if (target.kind === 'download') {
          download(bytes, fileName);
          return setState({ kind: 'done', message: `GIF descargado: ${fileName}` });
        }
        setState({ kind: 'saving' });
        const written = await writeGifTarget(target.token, bytes);
        if (written.kind === 'failed') return setState({ kind: 'failed', message: written.message });
        const name = written.name === '' ? fileName : written.name;
        setState({ kind: 'done', message: written.copied ? `GIF guardado y copiado: ${name}. Pégalo con Ctrl+V en un documento.` : `GIF guardado: ${name}` });
      } catch (error) {
        if (controller.signal.aborted) return setState({ kind: 'idle' });
        console.error('Replay: the GIF could not be made', error);
        setState({ kind: 'failed', message: FAILED });
      } finally {
        if (abortRef.current === controller) abortRef.current = null;
      }
    })();
  }, [blocked, busy, captureRef, draft, events, focused, freezeSeconds, multi, onStart, rounds, span, speed]);

  const cancel = useCallback(() => abortRef.current?.abort(), []);
  const toggle = useCallback(() => setOpen((value) => !value), []);
  const seconds = span === null ? 0 : (span.to - span.from) / (speed > 0 ? speed : 1);
  return { open, toggle, state, busy, seconds, blocked, start, cancel };
}
