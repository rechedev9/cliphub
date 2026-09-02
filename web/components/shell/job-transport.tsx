'use client';

import Link from '@/src/compat/link';
import { useSyncExternalStore, type ReactElement } from 'react';
import { captureProgressPercent } from '@/lib/capture-progress';
import {
  serverShellActivitySnapshot,
  shellActivitySnapshot,
  subscribeToShellActivity,
  type ShellJob,
  type ShellJobStage,
} from '@/lib/shell-activity';

const STAGE_LABEL: Record<ShellJobStage, string> = {
  queued: 'En cola',
  recording: 'Capturando',
  composing: 'Editando',
};

/** Command-strip job chip. The ring and percent exist only while capturing. */
export function JobTransport(): ReactElement | null {
  const activity = useSyncExternalStore(
    subscribeToShellActivity,
    shellActivitySnapshot,
    serverShellActivitySnapshot,
  );
  const [job] = activity.jobs;
  if (job === undefined) return null;

  const queued = activity.jobs.length - 1;

  return (
    <Link
      href="/videos"
      // The live job always has a destination: the library.
      className="studio-rim flex h-9 min-w-0 items-center gap-2.5 rounded-md border border-border-strong bg-surface-2 pr-3 pl-2 transition-colors duration-(--dur-fast) ease-(--ease-standard) hover:border-border-accent hover:bg-surface-3"
      aria-label={transportLabel(job, queued)}
      title={transportLabel(job, queued)}
    >
      <StageIndicator job={job} />
      <span className="flex min-w-0 items-center gap-2">
        <span className="font-[family-name:var(--font-display)] text-meta font-semibold tracking-wider text-fg-1 uppercase">
          {STAGE_LABEL[job.stage]}
        </span>
        <span className="hidden min-w-0 truncate text-body-sm text-fg-2 lg:inline">{job.title}</span>
      </span>
      {job.progress === null ? null : (
        <span className="flex shrink-0 items-baseline gap-1.5 font-[family-name:var(--font-mono)] text-meta tabular-nums">
          <span className="text-fg-1">{captureProgressPercent(job.progress)}%</span>
          <span className="text-fg-2">
            {job.progress.done}/{job.progress.total}
          </span>
        </span>
      )}
      {queued > 0 ? (
        <span
          className="font-[family-name:var(--font-mono)] text-meta text-fg-3 tabular-nums"
          aria-hidden
        >
          +{queued}
        </span>
      ) : null}
    </Link>
  );
}

/** Determinate ring from capture percent; a dot when the stage has none. */
function StageIndicator({ job }: { job: ShellJob }): ReactElement {
  if (job.progress === null) {
    return (
      <span
        aria-hidden
        className="size-2.5 shrink-0 rounded-full bg-primary shadow-(--glow-primary-sm)"
      />
    );
  }

  const pct = captureProgressPercent(job.progress);
  return (
    <span
      aria-hidden
      className="grid size-5 shrink-0 place-items-center rounded-full"
      style={{ background: `conic-gradient(var(--primary) ${pct}%, var(--surface-4) 0)` }}
    >
      <span className="size-3 rounded-full bg-surface-2" />
    </span>
  );
}

function transportLabel(job: ShellJob, queued: number): string {
  const progress =
    job.progress === null
      ? ''
      : ` ${captureProgressPercent(job.progress)}% · ${job.progress.done} de ${job.progress.total}`;
  const rest = queued > 0 ? ` · ${queued} en espera` : '';
  return `${STAGE_LABEL[job.stage]}: ${job.title}${progress}${rest}. Ir a la biblioteca.`;
}
