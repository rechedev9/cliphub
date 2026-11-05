'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Download } from 'lucide-react';
import { streamsApi, type StreamSummary } from '@/lib/api/streams';
import { errorMessage, formatStreamClock } from '@/lib/streams/plan';
import { Button, buttonVariants } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';

const POLL_MS = 2000;
const FAIL_MESSAGE = 'No pudimos crear el resumen. Vuelve a intentarlo.';

/** One landscape video with every play of the VOD, in order. */
export function StreamSummaryCard({
  jobId,
  playCount,
  playSeconds,
  disabled,
}: {
  jobId: string;
  playCount: number;
  playSeconds: number;
  disabled: boolean;
}): ReactNode {
  const [summary, setSummary] = useState<StreamSummary | null>(null);
  const [requestError, setRequestError] = useState<string | null>(null);
  const running = summary?.status === 'running';

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const next = await streamsApi.getSummary(jobId);
        if (active) setSummary(next);
      } catch (err) {
        if (active) setRequestError(errorMessage(err, FAIL_MESSAGE));
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
      setSummary(await streamsApi.startSummary(jobId));
    } catch (err) {
      setRequestError(errorMessage(err, FAIL_MESSAGE));
    }
  };

  const done = summary?.status === 'done';
  // A summary made before a new scan covers other plays; offer to remake it.
  const stale = done && summary.play_count !== playCount;
  const videoUrl = done ? streamsApi.summaryVideoUrl(jobId, summary.rendered_at) : null;

  return (
    <section aria-label="Resumen horizontal" className="flex flex-col gap-3 rounded-md border border-border-subtle p-3">
      <h3 className="font-semibold">Resumen horizontal</h3>

      {summary === null && !requestError ? <p className="text-body-sm text-fg-3">Comprobando…</p> : null}

      {summary?.status === 'idle' || stale ? (
        <>
          <p className="text-body-sm text-fg-2">
            Un vídeo 16:9 con tus {playCount} jugadas seguidas, unos {formatStreamClock(playSeconds)}.
          </p>
          <Button variant="outline" disabled={disabled} onClick={() => void start()}>
            {stale ? 'Rehacer resumen' : 'Crear resumen'}
          </Button>
        </>
      ) : null}

      {running ? (
        <div className="flex flex-col gap-2">
          <p className="text-body-sm text-fg-2" aria-live="polite">
            Creando resumen · {Math.round((summary.progress ?? 0) * 100)} %
          </p>
          <Progress tone="stream" size="sm" value={(summary.progress ?? 0) * 100} />
        </div>
      ) : null}

      {done && videoUrl ? (
        <div className="flex flex-col gap-2">
          <video src={videoUrl} controls preload="metadata" className="aspect-video w-full rounded bg-black" />
          <p className="text-body-sm text-fg-2 tabular-nums">
            {summary.play_count} jugadas · {formatStreamClock(summary.duration_seconds ?? 0)}
          </p>
          <a href={videoUrl} download="resumen.mp4" className={buttonVariants({ variant: 'outline' })}>
            <Download aria-hidden />
            Guardar resumen
          </a>
        </div>
      ) : null}

      {summary?.status === 'failed' || requestError ? (
        <div className="flex flex-col gap-2">
          <p role="alert" className="text-body-sm text-destructive">
            {requestError ?? summary?.error ?? FAIL_MESSAGE}
          </p>
          <Button variant="outline" disabled={disabled} onClick={() => void start()}>
            Volver a intentar
          </Button>
        </div>
      ) : null}
    </section>
  );
}
