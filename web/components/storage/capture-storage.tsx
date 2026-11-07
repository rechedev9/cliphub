'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Trash2 } from 'lucide-react';
import { api } from '@/lib/api';
import { deleteCapturesErrorMessage, deleteFeedbackTone } from '@/lib/delete-error';
import { formatBytes, freeableJobs, storageLoadErrorMessage, type FreeableJob } from '@/lib/storage';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

const CAPTURES_DELETE_WARNING =
  'Se borran las grabaciones de CS2 de esta demo completa. El vídeo final se queda. Para editarlo y montarlo de nuevo habrá que volver a capturar.';

type StorageState =
  | { kind: 'loading' }
  | { kind: 'error'; message: string }
  | { kind: 'ready'; totalBytes: number; jobs: FreeableJob[] };

/** Disk used by Studio jobs, and the Full Demo captures that can be freed per long video. */
export function CaptureStorage(): ReactNode {
  const [state, setState] = useState<StorageState>({ kind: 'loading' });
  const [reloadKey, setReloadKey] = useState(0);
  const [freed, setFreed] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    void Promise.all([api.getStorage(), api.listMatches().catch(() => [])])
      .then(([usage, matches]) => {
        if (active) setState({ kind: 'ready', totalBytes: usage.totalBytes, jobs: freeableJobs(usage, matches) });
      })
      .catch((err: unknown) => {
        if (active) setState({ kind: 'error', message: storageLoadErrorMessage(err) });
      });
    return () => { active = false; };
  }, [reloadKey]);

  const retry = (): void => {
    setState({ kind: 'loading' });
    setReloadKey((key) => key + 1);
  };

  const onFreed = (job: FreeableJob, bytes: number): void => {
    setFreed(`${job.label}: se han liberado ${formatBytes(bytes)}.`);
    setState((current) =>
      current.kind === 'ready'
        ? { kind: 'ready', totalBytes: Math.max(0, current.totalBytes - bytes), jobs: current.jobs.filter((row) => row.jobId !== job.jobId) }
        : current,
    );
  };

  let body: ReactNode;
  if (state.kind === 'ready') {
    body = (
      <div className="flex flex-col gap-3">
        {freed ? <p role="status" className="text-body-sm text-success">{freed}</p> : null}
        {state.jobs.length > 0 ? (
          <ul className="flex flex-col gap-2" aria-label="Demos completas con capturas">
            {state.jobs.map((job) => (
              <CaptureRow key={job.jobId} job={job} onFreed={(bytes) => onFreed(job, bytes)} />
            ))}
          </ul>
        ) : (
          <p className="border border-dashed border-border-strong bg-surface-1 px-4 py-3 text-body-sm text-fg-2">
            No hay capturas que borrar.
          </p>
        )}
      </div>
    );
  } else if (state.kind === 'error') {
    body = (
      <div className="flex flex-col items-start gap-3">
        <p role="alert" className="text-body-sm text-destructive">{state.message}</p>
        <Button type="button" variant="outline" size="sm" onClick={retry}>Reintentar</Button>
      </div>
    );
  } else {
    body = (
      <div role="status" aria-label="Leyendo espacio usado" className="flex flex-col gap-2">
        <Skeleton className="h-11 w-full rounded-none" />
        <Skeleton className="h-16 w-full rounded-none" />
      </div>
    );
  }

  return (
    <section className="flex flex-col gap-3" aria-labelledby="capture-storage-title">
      <div className="flex flex-col gap-1">
        <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
          <h2 id="capture-storage-title" className="font-display text-title font-bold text-fg-1">
            Capturas de demos completas
          </h2>
          {state.kind === 'ready' ? (
            <span className="text-body-sm tabular-nums text-fg-2">Usado por tus vídeos: {formatBytes(state.totalBytes)}</span>
          ) : null}
        </div>
        <p className="measure-read text-body-sm text-fg-2">
          Las grabaciones de CS2 de cada demo completa. Studio las borra solo cuando su render final está listo.
        </p>
      </div>
      {body}
    </section>
  );
}

/** One long video with captures; "Borrar capturas" asks inline before it deletes. */
function CaptureRow({ job, onFreed }: { job: FreeableJob; onFreed: (bytes: number) => void }): ReactNode {
  const [armed, setArmed] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const confirm = async (): Promise<void> => {
    if (deleting) return;
    setDeleting(true);
    setError(null);
    try {
      const bytes = await api.deleteJobCaptures(job.jobId);
      setArmed(false);
      onFreed(bytes);
    } catch (err) {
      setError(deleteCapturesErrorMessage(err));
      setArmed(false);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <li className="flex flex-col gap-3 border border-border bg-surface-1 px-3.5 py-2.5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-1">
          <span className="truncate text-body-sm font-semibold text-fg-1">{job.label}</span>
          <span className="text-meta tabular-nums text-fg-2">
            Capturas {formatBytes(job.captureBytes)} · Render {formatBytes(job.renderBytes)}
          </span>
        </div>
        {armed ? null : (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => { setError(null); setArmed(true); }}
            className="hover:bg-destructive/10 hover:text-destructive"
          >
            <Trash2 aria-hidden />
            Borrar capturas
          </Button>
        )}
      </div>
      {armed ? (
        <div className="flex flex-col gap-3 border-t border-border pt-3">
          <p className="text-body-sm text-fg-2">{CAPTURES_DELETE_WARNING}</p>
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="destructive"
              size="sm"
              autoFocus
              loading={deleting}
              loadingText="Borrando…"
              onClick={() => void confirm()}
              aria-label={`Confirmar borrar capturas de ${job.label}`}
            >
              <Trash2 aria-hidden />
              Borrar capturas
            </Button>
            <Button type="button" variant="outline" size="sm" disabled={deleting} onClick={() => setArmed(false)}>
              Cancelar
            </Button>
          </div>
        </div>
      ) : null}
      {error ? (
        <p role="status" className={`text-body-sm ${deleteFeedbackTone(error) === 'notice' ? 'text-fg-2' : 'text-destructive'}`}>
          {error}
        </p>
      ) : null}
    </li>
  );
}
