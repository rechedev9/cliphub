'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { AlertTriangle, Clapperboard } from 'lucide-react';
import { toast } from 'sonner';
import { streamsApi, type StreamJob } from '@/lib/api/streams';
import { startPollLoop } from '@/lib/poll-loop';
import { sortStreamJobs, streamListCadence } from '@/lib/streams/list';
import {
  STREAM_LIST_FAIL_MESSAGE,
  STREAM_OFFLINE_MESSAGE,
  errorMessage,
  isServiceUnavailable,
  isStreamURLValidationError,
} from '@/lib/streams/plan';
import { streamImportErrorMessage, streamSourceUrlError } from '@/lib/streams/import';
import { STREAM_WORKFLOW_STEPS } from '@/lib/streams/editor';
import Link from 'next/link';
import { CLIPS_HREF } from '@/lib/clips/routes';
import { WorkflowProgress } from '@/components/studio/workflow-progress';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { StreamListRow } from '@/components/streams/stream-list-row';
import { StreamSourcePanel } from '@/components/streams/stream-source-panel';

const POLL_FAST_MS = 1500;
const POLL_IDLE_MS = 10000;

/** /streams: new source on top, every stream job as a row underneath. */
export default function StreamsPage(): ReactNode {
  const router = useRouter();
  const [jobs, setJobs] = useState<StreamJob[] | null>(null);
  const [offline, setOffline] = useState(false);
  const [listError, setListError] = useState<string | null>(null);
  const [pollGeneration, setPollGeneration] = useState(0);
  const [sourceUrl, setSourceUrl] = useState('');
  const [title, setTitle] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const stop = startPollLoop({
      fastMs: POLL_FAST_MS,
      idleMs: POLL_IDLE_MS,
      tick: async () => {
        try {
          const next = sortStreamJobs(await streamsApi.listJobs());
          setJobs(next);
          setOffline(false);
          setListError(null);
          return streamListCadence(next);
        } catch (err) {
          setOffline(isServiceUnavailable(err));
          setListError(errorMessage(err, STREAM_LIST_FAIL_MESSAGE));
          return 'idle';
        }
      },
    });
    return stop;
  }, [pollGeneration]);

  const open = useCallback(
    (job: StreamJob) => {
      if (job.status === 'acquiring') {
        toast('Trayendo clip…', { description: 'Descargando el vídeo de origen en este PC' });
      }
      router.push(`/streams/${job.id}`);
    },
    [router],
  );

  const submitUrl = useCallback(async () => {
    const trimmed = sourceUrl.trim();
    const invalid = streamSourceUrlError(trimmed);
    if (invalid) {
      setError(invalid);
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      open(await streamsApi.createFromUrl({ sourceUrl: trimmed, title: title.trim() || undefined }));
    } catch (err) {
      setError(streamImportErrorMessage(err, 'url'));
      setSubmitting(false);
    }
  }, [sourceUrl, title, open]);

  const submitFile = useCallback(
    async (file: File) => {
      setError(null);
      setSubmitting(true);
      try {
        open(await streamsApi.createFromFile(file, title.trim() || undefined));
      } catch (err) {
        setError(streamImportErrorMessage(err, 'file'));
        setSubmitting(false);
      }
    },
    [title, open],
  );

  let list: ReactNode;
  if (jobs === null) {
    // A failed first poll leaves `jobs` null forever; the alert above is then
    // the only truthful report, and a spinner beside it claims work in flight.
    list =
      listError !== null ? null : (
        <p role="status" className="flex items-center gap-2 py-6 text-body-sm text-fg-2">
          <span aria-hidden className="studio-spinner" />
          Cargando streams
        </p>
      );
  } else if (jobs.length === 0) {
    list = (
      <p className="studio-panel px-6 py-8 text-center text-body text-fg-2">
        Tus proyectos aparecerán aquí. Importa un vídeo para crear el primero.
      </p>
    );
  } else {
    list = (
      <ul className="flex flex-col gap-3">
        {jobs.map((job) => (
          <StreamListRow key={job.id} job={job} onOpen={() => open(job)} onDeleted={() => setPollGeneration((g) => g + 1)} />
        ))}
      </ul>
    );
  }

  return (
    <div data-streams-home className="measure-work flex flex-col gap-6">
      <StudioPageHeader
        title="Clips de stream"
        actions={(
          <Button asChild variant="outline" className="bg-surface-1">
            <Link href={CLIPS_HREF}><Clapperboard aria-hidden />Crear desde una demo</Link>
          </Button>
        )}
        description="Convierte tus mejores momentos en Shorts verticales."
      />

      {listError !== null ? (
        <div
          role="alert"
          className="flex flex-wrap items-center gap-3 rounded-lg border border-destructive/45 bg-destructive/10 px-4 py-3 text-body-sm text-destructive"
        >
          <AlertTriangle aria-hidden className="size-4 shrink-0" />
          <span className="min-w-0 flex-1">{offline ? STREAM_OFFLINE_MESSAGE : listError}</span>
          <Button type="button" variant="outline" size="sm" onClick={() => setPollGeneration((g) => g + 1)}>
            Reintentar
          </Button>
        </div>
      ) : null}

      <WorkflowProgress
        steps={STREAM_WORKFLOW_STEPS}
        current={0}
        variant="connected"
      />
      <StreamSourcePanel
        sourceUrl={sourceUrl}
        title={title}
        submitting={submitting}
        error={error}
        onSourceUrlChange={(value) => {
          setSourceUrl(value);
          if (isStreamURLValidationError(error)) setError(null);
        }}
        onTitleChange={setTitle}
        onSubmitUrl={() => void submitUrl()}
        onSubmitFile={(file) => void submitFile(file)}
      />

      <section aria-labelledby="stream-projects-title" className="flex flex-col gap-4">
        <div className="flex items-center gap-3">
          <h2 id="stream-projects-title" className="font-display text-title font-semibold text-fg-1">Tus proyectos</h2>
          {jobs !== null ? (
            <span className="min-w-8 rounded-md bg-surface-3 px-2 py-0.5 text-center text-body-sm tabular-nums text-fg-2">
              {jobs.length}
            </span>
          ) : null}
        </div>
        {list}
      </section>
    </div>
  );
}
