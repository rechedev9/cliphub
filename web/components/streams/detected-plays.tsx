'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { streamsApi, type StreamClipRange, type StreamPlay, type StreamPlayScan } from '@/lib/api/streams';
import { errorMessage, formatStreamClock } from '@/lib/streams/plan';
import { playIsCovered, rankPlays } from '@/lib/streams/plays';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { StreamSummaryCard } from '@/components/streams/stream-summary';

const POLL_MS = 2000;

/**
 * Finds the streamer's kills in the source (the red kill-feed highlight of
 * CS2) and offers each run of kills as a moment. A full VOD scans on its own
 * and also offers a landscape summary of the plays the user picks.
 */
export function StreamDetectedPlays({
  jobId,
  vod,
  clips,
  disabled,
  onAdd,
  onSeek,
}: {
  jobId: string;
  vod: boolean;
  clips: readonly StreamClipRange[];
  disabled: boolean;
  onAdd: (plays: StreamPlay[]) => void;
  onSeek: (seconds: number) => void;
}): ReactNode {
  const [scan, setScan] = useState<StreamPlayScan | null>(null);
  const [requestError, setRequestError] = useState<string | null>(null);
  const running = scan?.status === 'running';
  const autoStarted = useRef(false);

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const next = await streamsApi.getPlays(jobId);
        if (active) setScan(next);
      } catch (err) {
        if (active) setRequestError(errorMessage(err, 'No pudimos buscar jugadas. Vuelve a intentarlo.'));
      }
    };
    void load();
    const timer = running ? setInterval(() => void load(), POLL_MS) : undefined;
    return () => {
      active = false;
      if (timer) clearInterval(timer);
    };
  }, [jobId, running]);

  const start = async () => {
    setRequestError(null);
    try {
      setScan(await streamsApi.startPlayScan(jobId));
    } catch (err) {
      setRequestError(errorMessage(err, 'No pudimos buscar jugadas. Vuelve a intentarlo.'));
    }
  };

  useEffect(() => {
    if (!vod || disabled || scan?.status !== 'idle' || autoStarted.current) return;
    autoStarted.current = true;
    void start();
  });

  const ranked = rankPlays(scan?.plays ?? []);
  const open = ranked.filter((play) => !playIsCovered(clips, play));

  return (
    <section aria-label="Jugadas" className="flex flex-col gap-3 rounded-md border border-border-subtle p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="font-semibold">Jugadas</h3>
        {scan?.status === 'done' && open.length > 1 ? (
          <Button variant="outline" size="sm" disabled={disabled} onClick={() => onAdd(open)}>
            Añadir todas
          </Button>
        ) : null}
      </div>

      {scan === null && !requestError ? <p className="text-body-sm text-fg-3">Comprobando…</p> : null}

      {scan?.status === 'idle' && !requestError ? (
        <>
          <p className="text-body-sm text-fg-2">
            Busca tus kills en el kill feed de CS2 y crea un momento por cada jugada.
          </p>
          <Button variant="outline" disabled={disabled} onClick={() => void start()}>
            Buscar jugadas
          </Button>
        </>
      ) : null}

      {running ? (
        <div className="flex flex-col gap-2">
          <p className="text-body-sm text-fg-2" aria-live="polite">
            Buscando jugadas · {Math.round((scan?.progress ?? 0) * 100)} %
          </p>
          <Progress tone="stream" size="sm" value={(scan?.progress ?? 0) * 100} />
        </div>
      ) : null}

      {scan?.status === 'done' && ranked.length === 0 ? (
        <p className="text-body-sm text-fg-2">
          No hemos detectado kills en la kill feed (debe verse arriba a la derecha). Puedes marcar tus momentos a mano abajo.
        </p>
      ) : null}

      {vod && scan?.status === 'done' && ranked.length > 0 ? (
        <StreamSummaryCard jobId={jobId} plays={scan.plays} disabled={disabled} />
      ) : null}

      {scan?.status === 'done' && ranked.length > 0 ? (
        <ul className="flex flex-col divide-y divide-border-subtle">
          {ranked.map((play) => {
            const covered = playIsCovered(clips, play);
            return (
              <li key={play.id} className="flex items-center justify-between gap-2 py-2">
                <button
                  type="button"
                  className="min-h-11 flex-1 rounded text-left text-body-sm hover:text-fg-1"
                  onClick={() => onSeek(play.start_seconds)}
                >
                  <span className="font-semibold">{play.label}</span>
                  <span className="text-fg-3 tabular-nums"> · {formatStreamClock(play.kill_seconds[0] ?? play.start_seconds)}</span>
                </button>
                {covered ? (
                  <span className="text-meta text-fg-3">Añadida</span>
                ) : (
                  <Button variant="ghost" size="sm" disabled={disabled} onClick={() => onAdd([play])}>
                    Añadir
                  </Button>
                )}
              </li>
            );
          })}
        </ul>
      ) : null}

      {scan?.status === 'failed' || requestError ? (
        <div className="flex flex-col gap-2">
          <p role="alert" className="text-body-sm text-destructive">
            {requestError ?? scan?.error ?? 'No pudimos buscar jugadas.'}
          </p>
          <Button variant="outline" disabled={disabled} onClick={() => void start()}>
            Volver a buscar
          </Button>
        </div>
      ) : null}
    </section>
  );
}
