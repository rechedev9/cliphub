'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useId, useRef, useState, useSyncExternalStore, type ReactElement } from 'react';
import { captureProgressPercent } from '@/lib/capture-progress';
import { CLIPS_HREF } from '@/lib/clips/routes';
import {
  serverShellActivitySnapshot,
  shellActivitySnapshot,
  subscribeToShellActivity,
  type ShellJob,
  type ShellJobStage,
} from '@/lib/shell-activity';
import { cn } from '@/lib/utils';

const STAGE_META: Record<ShellJobStage, { tag: string; sub: string; tone: string }> = {
  recording: { tag: 'REC', sub: 'CS2 + HLAE grabando · no toques el juego', tone: 'text-stream-text' },
  composing: { tag: 'RENDER', sub: 'Edición · FFmpeg', tone: 'text-primary' },
  parsing: { tag: 'PARSEO', sub: 'Parseando la demo', tone: 'text-primary' },
  acquiring: { tag: 'DESCARGA', sub: 'Descargando el vídeo de origen', tone: 'text-primary' },
  queued: { tag: 'EN COLA', sub: 'Empieza al acabar el REC actual', tone: 'text-fg-3' },
};

const PILL_CLASS =
  'flex min-h-10 min-w-0 items-center gap-2.5 rounded-md border border-border-strong bg-surface-2 px-3 font-[family-name:var(--font-mono)] text-meta tracking-wider uppercase';

/** Command-strip job pill plus the 400px dropdown listing every live job. */
export function JobTransport(): ReactElement | null {
  const pathname = usePathname();
  const activity = useSyncExternalStore(
    subscribeToShellActivity,
    shellActivitySnapshot,
    serverShellActivitySnapshot,
  );
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const panelId = useId();

  useEffect(() => {
    if (!open) return;
    const onPointer = (event: PointerEvent): void => {
      if (rootRef.current !== null && !rootRef.current.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent): void => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('pointerdown', onPointer);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('pointerdown', onPointer);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const [top] = activity.jobs;
  if (top === undefined && pathname !== CLIPS_HREF) return null;

  const active = activity.jobs.filter((job) => job.stage !== 'queued').length;
  const queued = activity.jobs.length - active;
  const more = activity.jobs.length - 1;

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        aria-label={top === undefined ? 'Trabajos: todo listo' : `Trabajos: ${pillTag(top)}, ${top.title}`}
        onClick={() => setOpen((value) => !value)}
        className={cn(
          PILL_CLASS,
          'transition-colors duration-(--dur-fast) ease-standard hover:bg-surface-3',
          top?.stage === 'recording' ? 'border-stream/45' : null,
          top === undefined ? 'max-sm:hidden' : null,
        )}
      >
        {top === undefined ? (
          <>
            <span aria-hidden className="size-2 shrink-0 rounded-full bg-success" />
            <span className="text-success">Todo listo</span>
          </>
        ) : (
          <>
            <StageIndicator stage={top.stage} />
            <span className={cn('tabular-nums', STAGE_META[top.stage].tone)}>{pillTag(top)}</span>
            {more > 0 ? (
              <>
                <span aria-hidden className="h-3.5 w-px shrink-0 bg-border-strong" />
                <span className="text-primary">{more} más</span>
              </>
            ) : null}
          </>
        )}
        <span aria-hidden className="text-fg-3">
          ▾
        </span>
      </button>

      {open ? (
        <div
          id={panelId}
          role="dialog"
          aria-label="Trabajos en marcha"
          className="studio-enter absolute top-full right-0 z-40 mt-1 w-[400px] max-w-[calc(100vw-1rem)] overflow-hidden rounded-[10px] border border-border bg-surface-4 shadow-[var(--elev-4)]"
        >
          <div className="flex min-h-10 items-center justify-between gap-3 border-b border-border-strong px-3.5 py-2 font-[family-name:var(--font-mono)] text-meta tracking-widest text-fg-3 uppercase">
            <span>
              Trabajos · {activity.jobs.length === 0 ? 'nada en marcha' : `${active} activos · ${queued} en cola`}
            </span>
            <Link
              href={CLIPS_HREF}
              onClick={() => setOpen(false)}
              className="inline-flex min-h-10 items-center text-primary hover:text-fg-1"
            >
              Ver en 01 →
            </Link>
          </div>
          {activity.jobs.length === 0 ? (
            <p className="px-3.5 py-4 text-label text-fg-2">Nada en marcha. Todo listo.</p>
          ) : (
            <ul>
              {activity.jobs.map((job) => (
                <li key={job.id} className="border-b border-border-subtle last:border-b-0">
                  <JobRow job={job} onNavigate={() => setOpen(false)} />
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}
    </div>
  );
}

function JobRow({ job, onNavigate }: { job: ShellJob; onNavigate: () => void }): ReactElement {
  const meta = STAGE_META[job.stage];
  const percent = job.progress === null ? null : captureProgressPercent(job.progress);
  return (
    <Link
      href={job.href}
      onClick={onNavigate}
      className="flex min-h-10 flex-col gap-1.5 px-3.5 py-3 transition-colors duration-(--dur-fast) ease-standard hover:bg-surface-5"
    >
      <span className="flex items-center justify-between gap-2.5">
        <span className="flex min-w-0 items-center gap-2 font-[family-name:var(--font-display)] text-label font-bold text-fg-1 uppercase">
          <StageIndicator stage={job.stage} />
          <span className="truncate">{job.title}</span>
        </span>
        <span className={cn('shrink-0 font-[family-name:var(--font-mono)] text-meta tracking-wider tabular-nums', meta.tone)}>
          {pillTag(job)}
        </span>
      </span>
      <span className={cn('studio-bar', meta.tone)} aria-hidden>
        <BarFill stage={job.stage} percent={percent} />
      </span>
      <span className="font-[family-name:var(--font-mono)] text-meta tracking-wider text-fg-3 uppercase">
        {meta.sub}
      </span>
    </Link>
  );
}

function BarFill({ stage, percent }: { stage: ShellJobStage; percent: number | null }): ReactElement {
  if (stage === 'queued') return <span style={{ width: '0%' }} />;
  if (percent === null) return <span className="studio-indeterminate" />;
  return <span style={{ width: `${percent}%` }} />;
}

function StageIndicator({ stage }: { stage: ShellJobStage }): ReactElement {
  if (stage === 'recording') {
    return <span aria-hidden className="neon-pulse size-2 shrink-0 rounded-full bg-stream" />;
  }
  if (stage === 'queued') {
    return <span aria-hidden className="size-2 shrink-0 rounded-full border border-fg-3" />;
  }
  return <span aria-hidden className="studio-spinner text-primary" />;
}

function pillTag(job: ShellJob): string {
  const { tag } = STAGE_META[job.stage];
  if (job.progress === null) return tag;
  if (job.stage === 'recording') return `${tag} R${job.progress.done}/${job.progress.total}`;
  if (job.stage === 'composing' && job.kind === 'reel') return `${tag} ${captureProgressPercent(job.progress)}%`;
  return tag;
}
