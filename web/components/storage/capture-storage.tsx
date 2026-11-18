'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Trash2 } from 'lucide-react';
import { api } from '@/lib/api';
import type { CaptureRetention } from '@/lib/api/storage';
import { setCaptureRetention } from '@/lib/api/video-edit';
import { deleteCapturesErrorMessage, deleteFeedbackTone } from '@/lib/delete-error';
import { captureRetentionNote, formatBytes, freeableJobs, storageLoadErrorMessage, type FreeableJob } from '@/lib/storage';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';

const CAPTURES_DELETE_WARNING =
  'Se borran las grabaciones de CS2 de esta demo completa. El vídeo final y sus versiones se quedan. Para editarlo habrá que volver a grabar sus rondas.';

type StorageState =
  | { kind: 'loading' }
  | { kind: 'error'; message: string }
  | { kind: 'ready'; totalBytes: number; jobs: FreeableJob[]; retention: CaptureRetention | null };

/** Disk used by Studio jobs, and the Full Demo captures that can be freed per long video. */
export function CaptureStorage(): ReactNode {
  const [state, setState] = useState<StorageState>({ kind: 'loading' });
  const [reloadKey, setReloadKey] = useState(0);
  const [freed, setFreed] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    void Promise.all([api.getStorage(), api.listMatches().catch(() => [])])
      .then(([usage, matches]) => {
        if (active) setState({ kind: 'ready', totalBytes: usage.totalBytes, jobs: freeableJobs(usage, matches), retention: usage.captureRetention });
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
        ? { ...current, totalBytes: Math.max(0, current.totalBytes - bytes), jobs: current.jobs.filter((row) => row.jobId !== job.jobId) }
        : current,
    );
  };

  const onRetentionSaved = (keep: number, bytes: number): void => {
    const which = keep === 1 ? 'del vídeo largo más reciente' : `de los ${keep} vídeos largos más recientes`;
    const kept = keep === 0 ? 'Studio ya no guarda capturas' : `Se guardan las capturas ${which}`;
    setFreed(bytes > 0 ? `${kept}. Se han liberado ${formatBytes(bytes)}.` : `${kept}.`);
    setReloadKey((key) => key + 1);
  };

  let body: ReactNode;
  if (state.kind === 'ready') {
    body = (
      <div className="flex flex-col gap-3">
        {state.retention ? <RetentionControl key={state.retention.keep} retention={state.retention} onSaved={onRetentionSaved} /> : null}
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
          {captureRetentionNote(state.kind === 'ready' ? state.retention : null)}
        </p>
      </div>
      {body}
    </section>
  );
}

/** Capturas guardadas: how many of the newest long videos keep their captures; lowering it frees the rest now. */
function RetentionControl({ retention, onSaved }: { retention: CaptureRetention; onSaved: (keep: number, freedBytes: number) => void }): ReactNode {
  const [value, setValue] = useState(String(retention.keep));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const keep = /^[0-9]{1,2}$/.test(value) ? Number(value) : Number.NaN;
  const valid = Number.isInteger(keep) && keep <= retention.max;
  const lowering = valid && keep < retention.keep && !retention.keepsAll;

  const save = async (): Promise<void> => {
    if (!valid || saving) return;
    setSaving(true);
    setError(null);
    try {
      onSaved(keep, await setCaptureRetention(keep));
    } catch {
      setError('No se pudo guardar. Comprueba que Studio sigue abierto e inténtalo de nuevo.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex flex-col gap-2 border border-border bg-surface-1 px-3.5 py-2.5">
      <div className="flex flex-wrap items-end gap-3">
        <label htmlFor="capture-retention" className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="text-body-sm font-semibold text-fg-1">Capturas guardadas</span>
          <span className="text-meta text-fg-2">Cuántos de los vídeos largos más recientes conservan sus capturas, de 0 a {retention.max}.</span>
        </label>
        <Input id="capture-retention" type="number" inputMode="numeric" min={0} max={retention.max} step={1} value={value}
          disabled={saving} aria-invalid={!valid} onChange={(event) => setValue(event.target.value)} className="w-20 tabular-nums" />
        <Button type="button" variant="outline" size="sm" disabled={!valid || keep === retention.keep} loading={saving} loadingText="Guardando…" onClick={() => void save()}>
          Guardar
        </Button>
      </div>
      {lowering ? (
        <p className="text-body-sm text-warning">Al guardar se borran ahora las capturas de los vídeos largos que quedan fuera. Editarlos volverá a grabar sus rondas.</p>
      ) : null}
      {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
    </div>
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
