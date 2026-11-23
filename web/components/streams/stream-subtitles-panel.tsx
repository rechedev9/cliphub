'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { toast } from 'sonner';
import { installTranscriber } from '@/lib/api/voice';
import { SUBTITLES_LABEL } from '@/lib/produce/subtitles';
import { voiceErrorText } from '@/lib/replay/voice-errors';
import {
  StreamSubtitlesError, fetchStreamTranscript, patchStreamSubtitles, patchStreamVoiceLine, streamSubtitleNote, streamSubtitleRows,
  streamSubtitlesStatus, stretchesKey, transcribeStream,
  type StreamSubtitleRow, type StreamSubtitlesKind, type StreamTranscript, type Stretch,
} from '@/lib/streams/subtitles';
import { PhraseRow, SubtitleToggle, TranscriberStatus } from '@/components/produce/subtitle-parts';
import { Button } from '@/components/ui/button';

const POLL_MS = 3000;
const TRANSCRIBE_DELAY_MS = 1500;

export type StreamSubtitlesVideo = { id: string; windows: readonly Stretch[] };

export type StreamSubtitlesPanelProps = {
  jobId: string;
  kind: StreamSubtitlesKind;
  video: StreamSubtitlesVideo | null;
  stretches: readonly Stretch[];
  disabled?: boolean;
  renderKey?: string;
  onStale?: (stale: boolean) => void;
};

export function clipStretch(clip: { start_seconds: number; end_seconds: number }): Stretch {
  return { start: clip.start_seconds, end: clip.end_seconds };
}

function errorCode(error: unknown): string | undefined {
  return error instanceof StreamSubtitlesError ? error.code : undefined;
}

export function StreamSubtitlesPanel({ jobId, kind, video, stretches, disabled = false, renderKey = '', onStale }: StreamSubtitlesPanelProps): ReactNode {
  const [transcript, setTranscript] = useState<StreamTranscript | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const asked = useRef('');

  const load = useCallback(async (): Promise<void> => {
    try {
      setTranscript(await fetchStreamTranscript(jobId));
      setLoadError(null);
    } catch {
      setLoadError('No se pudo leer la transcripción de este stream.');
    }
  }, [jobId]);

  useEffect(() => {
    void load();
  }, [load, renderKey]);

  const reportStale = useRef(onStale);
  reportStale.current = onStale;
  useEffect(() => {
    if (transcript !== null) reportStale.current?.(transcript.stale[kind]);
  }, [transcript, kind]);

  const status = streamSubtitlesStatus(transcript?.availability ?? 'available');
  const enabled = transcript !== null && transcript.subtitles[kind] && status.enabled;
  const missing = enabled && transcript.transcriber.state === 'not_installed';
  useEffect(() => {
    if (!missing) return;
    void installTranscriber().catch(() => undefined).then(load);
  }, [load, missing]);

  const waiting = transcript !== null && (['queued', 'running'].includes(transcript.state) || transcript.transcriber.state === 'installing' || missing);
  useEffect(() => {
    if (!waiting) return;
    const timer = window.setInterval(() => void load(), POLL_MS);
    return () => window.clearInterval(timer);
  }, [load, waiting]);

  const usable = transcript?.transcriber.state === 'installed' || transcript?.transcriber.state === 'installing';
  const key = stretchesKey(stretches);
  const stretchesRef = useRef(stretches);
  stretchesRef.current = stretches;
  useEffect(() => {
    if (!enabled || !usable || key === '' || asked.current === key) return;
    const timer = window.setTimeout(() => {
      asked.current = key;
      transcribeStream(jobId, stretchesRef.current).then(setTranscript, () => {
        asked.current = '';
      });
    }, TRANSCRIBE_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [enabled, usable, jobId, key]);

  const run = async (work: () => Promise<unknown>, fallback: string): Promise<void> => {
    setBusy(true);
    try {
      await work();
      await load();
    } catch (error) {
      toast.error(voiceErrorText(errorCode(error), fallback));
    } finally {
      setBusy(false);
    }
  };

  const locked = disabled || busy;
  const hidden = video === null ? [] : transcript?.subtitles.hidden[video.id] ?? [];
  const rows = transcript === null || video === null ? [] : streamSubtitleRows({ lines: transcript.lines, windows: video.windows, hidden });
  const note = transcript === null || !enabled ? null : streamSubtitleNote(transcript);
  const correct = (row: StreamSubtitleRow, text: string): void => {
    const clean = text.trim();
    if (clean === '' || clean === row.text) return;
    void run(() => patchStreamVoiceLine({ jobId, lineId: row.id }, { text: clean }), 'No se pudo guardar la corrección.');
  };
  const discard = (id: string, discarded: boolean): void => {
    void run(() => patchStreamVoiceLine({ jobId, lineId: id }, { discarded }), discarded ? 'No se pudo descartar la frase.' : 'No se pudo recuperar la frase.');
  };
  const hide = (id: string, next: boolean): void => {
    if (video === null) return;
    void run(() => patchStreamSubtitles(jobId, { video: video.id, line_id: id, hidden: next }), 'No se pudo guardar el cambio.');
  };
  const toggle = (next: boolean): void => {
    void run(() => patchStreamSubtitles(jobId, kind === 'shorts' ? { shorts: next } : { summary: next }), 'No se pudo guardar el cambio.');
  };

  return <div className="flex min-w-0 flex-col gap-3">
    <p className="text-body-sm text-fg-2">Lo que dices en los momentos de tus vídeos, transcrito en este PC a partir de la pista del micro, o del audio del stream si no tiene una aparte, en el idioma en que hablas. Una corrección o un descarte vale para todos los vídeos del stream; ocultar una frase vale solo para este vídeo.</p>
    {loadError ? <p role="alert" className="text-body-sm text-warning">{loadError}</p> : null}
    {transcript ? <>
      <SubtitleToggle label={SUBTITLES_LABEL} checked={enabled} disabled={locked || !status.enabled} onChange={toggle} />
      {status.note ? <p role="status" className="text-body-sm text-fg-2">{status.note}</p> : null}
      {enabled ? <>
        <TranscriberStatus transcriber={transcript.transcriber} busy={locked} onRetry={() => void run(installTranscriber, 'No se pudo preparar la transcripción.')} />
        {note ? <p role="status" className="text-body-sm text-fg-2">{note}</p> : null}
        {usable && transcript.state === 'failed' && stretches.length > 0 ? (
          <Button variant="secondary" size="sm" className="self-start" disabled={locked}
            onClick={() => void run(() => transcribeStream(jobId, stretches), 'No se pudo empezar la transcripción.')}>
            Transcribir de nuevo
          </Button>
        ) : null}
        {usable && stretches.length === 0 ? <p className="text-body-sm text-fg-2">Elige primero qué entra en el vídeo: solo se transcriben esos tramos.</p> : null}
        {rows.length > 0 ? <ul className="flex flex-col gap-2" aria-label="Frases de este vídeo">
          {rows.map((row) => <PhraseRow key={row.id} row={row} locked={locked}
            heading={<span className="font-semibold text-fg-1 tabular-nums">{row.clock}</span>}
            inputLabel={`Frase del minuto ${row.clock}`} discardedNote="No sale en ningún vídeo de este stream."
            onHide={(next) => hide(row.id, next)} onDiscard={(discarded) => discard(row.id, discarded)}
            onCorrect={(text) => correct(row, text)} />)}
        </ul> : null}
        {rows.length === 0 && usable && transcript.state === 'ready' && stretches.length > 0
          ? <p className="text-body-sm text-fg-2">{video === null ? 'Elige un vídeo para ver sus frases.' : 'No se oye a nadie hablar en lo que muestra este vídeo.'}</p> : null}
      </> : null}
    </> : null}
  </div>;
}
