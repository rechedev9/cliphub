'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { AlertTriangle } from 'lucide-react';
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
  nonVideoExtension,
} from '@/lib/streams/plan';
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
    if (!trimmed) {
      setError('Pega una URL de clip o VOD de Twitch, YouTube o Kick. Para un archivo local, usa un MP4.');
      return;
    }
    const badExt = nonVideoExtension(trimmed);
    if (badExt) {
      setError(
        `Esa URL apunta a un archivo .${badExt}, no a un vídeo. Pega el enlace de un clip o VOD de Twitch, YouTube o Kick, o usa “Subir un MP4”.`,
      );
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      open(await streamsApi.createFromUrl({ sourceUrl: trimmed, title: title.trim() || undefined }));
    } catch (err) {
      setError(errorMessage(err, 'No se pudo iniciar ese trabajo. Revisa la URL y vuelve a intentarlo.'));
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
        setError(errorMessage(err, 'No se pudo procesar ese archivo. Prueba con otro MP4.'));
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
        <p role="status" className="measure-list flex items-center gap-2 font-mono text-meta uppercase tracking-wider text-fg-3">
          <span aria-hidden className="studio-spinner" />
          Cargando streams
        </p>
      );
  } else if (jobs.length === 0) {
    list = (
      <p className="measure-list border border-dashed border-border px-4 py-6 text-center text-body-sm text-fg-2">
        Todavía no hay streams. Pega una URL o sube un MP4.
      </p>
    );
  } else {
    list = (
      <ul className="measure-list flex flex-col gap-2.5">
        {jobs.map((job) => (
          <StreamListRow key={job.id} job={job} onOpen={() => open(job)} onDeleted={() => setPollGeneration((g) => g + 1)} />
        ))}
      </ul>
    );
  }

  return (
    <div className="flex flex-col gap-3.5">
      <StudioPageHeader
        className="measure-list"
        title="Clips de stream"
        actions={<Button asChild variant="outline"><Link href={CLIPS_HREF}>Crear desde una demo</Link></Button>}
        description="Convierte una grabación en Shorts verticales: importa el vídeo, marca los cortes y ajusta el encuadre. Facecam, banners y música son opcionales."
      />

      {listError !== null ? (
        <div
          role="alert"
          className="measure-list flex flex-wrap items-center gap-3 border border-destructive/45 bg-destructive/10 px-3.5 py-2.5 text-body-sm text-destructive"
        >
          <AlertTriangle aria-hidden className="size-4 shrink-0" />
          <span className="min-w-0 flex-1">{offline ? STREAM_OFFLINE_MESSAGE : listError}</span>
          <Button type="button" variant="outline" size="sm" onClick={() => setPollGeneration((g) => g + 1)}>
            Reintentar
          </Button>
        </div>
      ) : null}

      <WorkflowProgress steps={['Importar vídeo', 'Elegir cortes', 'Ajustar encuadre', 'Crear y descargar']} current={0} />
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

      <p className="measure-list mt-1 font-mono text-meta uppercase tracking-widest text-fg-3">
        {jobs === null ? 'Tus proyectos de stream' : `Tus proyectos de stream · ${jobs.length}`}
      </p>

      {list}
    </div>
  );
}
