import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type StudioPageHeaderProps = {
  title: string;
  description: ReactNode;
  actions?: ReactNode;
  className?: string;
};

/**
 * Consistent title block for every Studio destination: a compact H1 on the
 * type scale and a one-line description, with actions on the right. No section
 * eyebrow: the command strip already names the section a few pixels above.
 */
export function StudioPageHeader({
  title,
  description,
  actions,
  className,
}: StudioPageHeaderProps): ReactNode {
  return (
    <header className={cn('flex flex-col', className)}>
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between lg:gap-8">
        <div className="min-w-0">
          <h1 className="text-section font-semibold text-fg-1">{title}</h1>
          <div className="measure-read mt-1.5 text-body-sm text-fg-2">{description}</div>
        </div>
        {actions ? <div className="shrink-0">{actions}</div> : null}
      </div>
    </header>
  );
}
