'use client';

import { useState, type ComponentProps, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { api } from '@/lib/api';
import { createJobFromLibraryDemo, type LibraryDemo } from '@/lib/api/library';
import { MATCH_STATUS_FAILED } from '@/lib/api/types';
import { MATCH_STATUS_SCANNED } from '@/lib/clips/hub';
import { flowHref, type ProduceFormat } from '@/lib/clips/routes';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';

/** A job in one of these has no player picked yet, so its flow opens on the player picker. */
const UNPICKED_STATUSES: ReadonlySet<string> = new Set(['queued', 'scanning', MATCH_STATUS_SCANNED]);

export type CrearRequest = {
  readonly demo: LibraryDemo;
  readonly format: ProduceFormat;
  /** The video needs its own player pick, such as several teammates in one video. */
  readonly pickPlayers?: boolean | undefined;
};

/** The newest linked job that still exists and did not fail (and, for a fresh pick, has no player yet), else a new job. */
export async function crearJobFor(request: CrearRequest): Promise<string> {
  for (const jobId of [...request.demo.job_ids].reverse()) {
    const scan = await api.getScan(jobId).catch(() => null);
    if (scan === null || scan.status === MATCH_STATUS_FAILED) continue;
    if (request.pickPlayers !== true || UNPICKED_STATUSES.has(scan.status)) return jobId;
  }
  return createJobFromLibraryDemo(request.demo.id);
}

type ButtonProps = ComponentProps<typeof Button>;

/** Hands a library demo to a Crear flow: the flow resumes its job at the right step. */
export function OpenInCrearButton({
  request,
  variant,
  size,
  className,
  children,
}: {
  request: CrearRequest;
  variant?: ButtonProps['variant'];
  size?: ButtonProps['size'];
  /** Classes of the button itself; the wrapper grows with it in a flex row. */
  className?: string;
  children: ReactNode;
}): ReactNode {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);

  function open(): void {
    setBusy(true);
    setFailed(false);
    crearJobFor(request)
      .then((job) => router.push(flowHref(request.format, { job })))
      .catch(() => {
        setFailed(true);
        setBusy(false);
      });
  }

  return (
    <span className={cn('inline-flex flex-wrap items-center gap-2', className === undefined ? undefined : 'min-w-0 flex-1')}>
      <Button type="button" variant={variant} size={size} loading={busy} onClick={open} className={className}>
        {children}
      </Button>
      {failed ? (
        <span role="alert" className="text-meta text-destructive">
          No se pudo pasar la demo a Crear.
        </span>
      ) : null}
    </span>
  );
}
