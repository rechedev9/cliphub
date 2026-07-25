'use client';

import Link from 'next/link';
import { useSyncExternalStore, type ReactElement } from 'react';
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

/**
 * The control-room transport: what the machine is doing, on every route.
 *
 * Everything here is derived from the orchestrator's reel list. The ring is
 * drawn only where the API reports real segment counts (`captureProgress`,
 * which exists solely while a reel is recording); `queued` and `composing` are
 * genuinely indeterminate and say so instead of animating an invented
 * percentage — design.md's "never fabricate progress" is not decorative advice
 * here, it is the difference between a status display and a lie.
 */
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
      // A live job is the one thing in the shell that always has somewhere to
      // go, so the transport is the link to it rather than a decorative chip.
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
        <span className="font-[family-name:var(--font-mono)] text-meta text-fg-2 tabular-nums">
          {job.progress.done}/{job.progress.total}
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

/**
 * A determinate ring where there are real segments, a two-tone dot where there
 * are not. The ring is a conic-gradient with a punched centre: no SVG, no
 * animation, and it repaints only when the poll delivers a new count.
 */
function StageIndicator({ job }: { job: ShellJob }): ReactElement {
  if (job.progress === null) {
    return (
      <span
        aria-hidden
        className="size-2.5 shrink-0 rounded-full bg-primary shadow-(--glow-primary-sm)"
      />
    );
  }

  const pct = Math.round((job.progress.done / job.progress.total) * 100);
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
  const progress = job.progress === null ? '' : ` ${job.progress.done} de ${job.progress.total}`;
  const rest = queued > 0 ? ` · ${queued} en espera` : '';
  return `${STAGE_LABEL[job.stage]}: ${job.title}${progress}${rest}. Ir a la biblioteca.`;
}
