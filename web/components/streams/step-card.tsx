import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

/** A titled panel in the active-step column, with an optional control in the head. */
export function StreamStepCard({
  title,
  control,
  children,
  className,
}: {
  title: string;
  control?: ReactNode;
  children: ReactNode;
  className?: string;
}): ReactNode {
  return (
    <section className={cn('studio-panel flex flex-col gap-2.5 p-3.5', className)} aria-label={title}>
      <div className="flex min-h-5 items-center justify-between gap-3">
        <span className="text-meta text-fg-3">{title}</span>
        {control}
      </div>
      {children}
    </section>
  );
}
