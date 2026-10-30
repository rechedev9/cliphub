'use client';

import { useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { MonitorPlay, Smartphone } from 'lucide-react';
import { api } from '@/lib/api';
import { createJobFromLibraryDemo, type LibraryDemo } from '@/lib/api/library';
import { PRODUCE_FORMAT, flowHref, type ProduceFormat } from '@/lib/clips/routes';
import { Button } from '@/components/ui/button';

const FAILED_JOB_STATUS = 'failed';

/**
 * The Crear job a library demo should open: its newest linked job that still
 * exists and did not fail, else a new job over the stored demo. The flow page
 * then resumes it at the right step (waiting, player pick or plays).
 */
async function crearJobFor(demo: LibraryDemo): Promise<string> {
  for (const jobId of [...demo.job_ids].reverse()) {
    const scan = await api.getScan(jobId).catch(() => null);
    if (scan !== null && scan.status !== FAILED_JOB_STATUS) return jobId;
  }
  return createJobFromLibraryDemo(demo.id);
}

/** "Crear Short" and "Vídeo largo" for a partida: the analysis area hands the demo to Crear. */
export function CreateFromDemo({ demo }: { demo: LibraryDemo }): ReactNode {
  const router = useRouter();
  const [busy, setBusy] = useState<ProduceFormat | null>(null);
  const [error, setError] = useState(false);

  async function open(format: ProduceFormat): Promise<void> {
    setBusy(format);
    setError(false);
    try {
      router.push(flowHref(format, { job: await crearJobFor(demo) }));
    } catch {
      setError(true);
      setBusy(null);
    }
  }

  return (
    <>
      <Button type="button" variant="outline" size="sm" disabled={busy !== null} onClick={() => void open(PRODUCE_FORMAT.short)}>
        <Smartphone aria-hidden />
        {busy === PRODUCE_FORMAT.short ? 'Preparando' : 'Crear Short'}
      </Button>
      <Button type="button" variant="outline" size="sm" disabled={busy !== null} onClick={() => void open(PRODUCE_FORMAT.full)}>
        <MonitorPlay aria-hidden />
        {busy === PRODUCE_FORMAT.full ? 'Preparando' : 'Vídeo largo'}
      </Button>
      {error ? (
        <span role="alert" className="text-meta text-destructive">
          No se pudo pasar la demo a Crear.
        </span>
      ) : null}
    </>
  );
}
