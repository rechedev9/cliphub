import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type StudioPageHeaderProps = {
  title: string;
  description: ReactNode;
  actions?: ReactNode;
  className?: string;
};

export function StudioPageHeader({
  title,
  description,
  actions,
  className,
}: StudioPageHeaderProps): ReactNode {
  return (
    <header className={cn('flex flex-col', className)}>
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between lg:gap-8">
        <div className="min-w-0">
          <h1 className="font-display text-display-sm font-bold text-fg-1 sm:text-display">{title}</h1>
          <div className="measure-read mt-3 text-body text-fg-2">{description}</div>
        </div>
        {actions ? <div className="shrink-0">{actions}</div> : null}
      </div>
    </header>
  );
}
