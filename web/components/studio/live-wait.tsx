import type { ReactNode } from 'react';
import { Loader2 } from 'lucide-react';
import type { JobProgress } from '@/lib/api/types';
import { jobProgressCount, jobProgressPercent } from '@/lib/job-progress';
import { cn } from '@/lib/utils';

export type LiveWaitProps = {
  progress?: JobProgress;
  label?: string;
  className?: string;
};

/** Spinner that stays alive with a real percent and current/total. */
export function LiveWait({ progress, label, className }: LiveWaitProps): ReactNode {
  const pct = progress ? jobProgressPercent(progress) : 0;
  const count = progress ? jobProgressCount(progress) : '0 / 0';

  return (
    <div
      role="status"
      aria-live="polite"
      aria-busy="true"
      className={cn('flex flex-col items-center gap-2', className)}
    >
      <span className="grid size-12 place-items-center border border-primary/45 bg-surface-0 text-primary">
        <Loader2 className="size-5 animate-spin motion-reduce:animate-none" aria-hidden />
      </span>
      <p className="font-mono text-title tabular-nums text-primary">{pct}%</p>
      <p className="font-mono text-meta tabular-nums text-fg-3">{count}</p>
      {label ? <p className="font-display text-body-sm font-bold uppercase text-fg-1">{label}</p> : null}
    </div>
  );
}
