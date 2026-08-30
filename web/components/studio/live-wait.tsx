import type { ReactNode } from 'react';
import { Loader2 } from 'lucide-react';
import type { JobProgress } from '@/lib/api/types';
import { jobProgressDisplay } from '@/lib/job-progress';
import { cn } from '@/lib/utils';

export type LiveWaitProps = {
  progress?: JobProgress;
  label?: string;
  className?: string;
};

/** Spinner that stays alive with a real percent and current/total. */
export function LiveWait({ progress, label, className }: LiveWaitProps): ReactNode {
  const display = jobProgressDisplay(progress);

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
      {display.percent ? <p className="font-mono text-title tabular-nums text-primary">{display.percent}</p> : null}
      {display.count ? <p className="font-mono text-meta tabular-nums text-fg-3">{display.count}</p> : null}
      {label ? <p className="font-display text-body-sm font-bold uppercase text-fg-1">{label}</p> : null}
    </div>
  );
}
