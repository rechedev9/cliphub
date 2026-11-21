'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { AlertTriangle, Film } from 'lucide-react';
import { streamsApi, type StreamJob, type StreamPlayScan } from '@/lib/api/streams';
import { STREAMS_HREF, streamEditorHref } from '@/lib/clips/routes';
import { scanVoiceSettings } from '@/lib/streams/detection';
import { streamHasSource } from '@/lib/streams/list';
import { errorMessage } from '@/lib/streams/plan';
import { streamDocumentTitle, streamTitle } from '@/lib/streams/title';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { StreamSummaryCard } from '@/components/streams/stream-summary';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';

const POLL_MS = 2000;
const SCAN_FAIL_MESSAGE = 'No pudimos buscar jugadas. Vuelve a intentarlo.';

function SummaryPlays({ jobId }: { jobId: string }): ReactNode {
  const [scan, setScan] = useState<StreamPlayScan | null>(null);
  const [requestError, setRequestError] = useState<string | null>(null);
  const autoStarted = useRef(false);
  const running = scan?.status === 'running';

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const next = await streamsApi.getPlays(jobId);
        if (active) setScan(next);
      } catch (err) {
        if (active) setRequestError(errorMessage(err, SCAN_FAIL_MESSAGE));
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
      setScan(await streamsApi.startPlayScan(jobId, scanVoiceSettings(scan?.voice)));
    } catch (err) {
      setRequestError(errorMessage(err, SCAN_FAIL_MESSAGE));
    }
  };

  useEffect(() => {
    if (scan?.status !== 'idle' || autoStarted.current) return;
    autoStarted.current = true;
    void start();
  });

  if (scan?.status === 'failed' || requestError) {
    return (
      <div className="flex flex-col items-start gap-2">
        <p role="alert" className="text-body-sm text-destructive">
          {requestError ?? scan?.error ?? SCAN_FAIL_MESSAGE}
        </p>
        <Button variant="outline" onClick={() => void start()}>
          Volver a buscar
        </Button>
      </div>
    );
  }
  if (scan?.status === 'done' && scan.plays.length > 0) return <StreamSummaryCard jobId={jobId} plays={scan.plays} />;
  if (scan?.status === 'done') {
    return (
      <p className="text-body-sm text-fg-2">
        No hemos encontrado jugadas en este vídeo, así que no hay nada para el resumen. Puedes hacer Shorts marcando tus momentos a mano.
      </p>
    );
  }
  if (scan === null || scan.status === 'idle') return <p className="text-body-sm text-fg-3">Comprobando…</p>;
  return (
    <div className="flex max-w-xl flex-col gap-2">
      <p className="text-body-sm text-fg-2" aria-live="polite">
        Buscando jugadas · {Math.round((scan.progress ?? 0) * 100)} %
      </p>
      <Progress tone="stream" size="sm" value={(scan.progress ?? 0) * 100} />
    </div>
  );
}

function SummaryUnavailable({ job }: { job: StreamJob }): ReactNode {
  const vod = job.source_kind === 'vod';
  return (
    <StudioEmptyState
      icon={Film}
      accent="magenta"
      title={vod ? 'El vídeo aún no está en este PC' : 'Un clip corto no tiene resumen'}
      description={
        vod
          ? 'El resumen se hace con las jugadas del vídeo. Espera a que termine de descargarse.'
          : 'El resumen junta las jugadas de un VOD completo. Con un clip corto puedes hacer Shorts.'
      }
      actions={
        <Button asChild>
          <Link href={streamEditorHref(job.id)}>{vod ? 'Ver progreso' : 'Hacer Shorts'}</Link>
        </Button>
      }
    />
  );
}

export function StreamSummaryScreen({ jobId }: { jobId: string }): ReactNode {
  const [job, setJob] = useState<StreamJob | null | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const title = job ? streamTitle(job) : undefined;
  useRouteTitle(title ? `Resumen de ${title}` : undefined);
  useDocumentTitle(title ? streamDocumentTitle(`Resumen de ${title}`) : undefined);

  useEffect(() => {
    let active = true;
    setError(null);
    streamsApi
      .getJob(jobId)
      .then((next) => {
        if (active) setJob(next);
      })
      .catch((err: unknown) => {
        if (active) setError(errorMessage(err, 'No se pudo abrir ese stream.'));
      });
    return () => {
      active = false;
    };
  }, [jobId, attempt]);

  if (error !== null) {
    return (
      <div role="alert">
        <StudioEmptyState
          icon={AlertTriangle}
          accent="magenta"
          title="No se pudo abrir el resumen"
          description={error}
          className="border-destructive/45"
          actions={
            <Button type="button" onClick={() => setAttempt((n) => n + 1)}>
              Reintentar
            </Button>
          }
        />
      </div>
    );
  }
  if (job === undefined) {
    return (
      <p role="status" className="measure-read flex items-center gap-2 text-body text-fg-2">
        <span aria-hidden className="studio-spinner" />
        Abriendo el resumen
      </p>
    );
  }
  if (job === null) {
    return (
      <StudioEmptyState
        icon={Film}
        accent="magenta"
        title="Ese stream ya no está"
        description="El trabajo no existe o se ha borrado. Vuelve a la lista para traer otro vídeo."
        actions={
          <Button asChild>
            <Link href={STREAMS_HREF}>Volver a Clips de stream</Link>
          </Button>
        }
      />
    );
  }
  if (job.source_kind !== 'vod' || !streamHasSource(job)) return <SummaryUnavailable job={job} />;

  return (
    <div className="measure-work flex flex-col gap-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-1">
          <h1 className="truncate font-display text-title font-bold text-fg-1">Resumen de {title}</h1>
          <p className="text-body-sm text-fg-2">Un vídeo horizontal 16:9 con las jugadas que elijas, una tras otra.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button asChild variant="outline" size="sm">
            <Link href={STREAMS_HREF}>Tus proyectos</Link>
          </Button>
          <Button asChild variant="outline" size="sm">
            <Link href={streamEditorHref(job.id)}>Hacer Shorts</Link>
          </Button>
        </div>
      </div>
      <SummaryPlays jobId={job.id} />
    </div>
  );
}
