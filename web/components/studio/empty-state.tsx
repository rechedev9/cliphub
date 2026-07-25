import type { LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { IconTile } from '@/components/studio/icon-tile';
import { cn } from '@/lib/utils';

export type StudioEmptyStateProps = {
  icon: LucideIcon;
  title: string;
  description: ReactNode;
  actions?: ReactNode;
  note?: ReactNode;
  accent?: 'cyan' | 'magenta';
  compact?: boolean;
  className?: string;
};

/**
 * Actionable, bounded empty state shared by dashboard destinations. Centered
 * inside the remaining content area so an empty page reads as an intentional
 * state instead of content stranded in a large void, and capped at the 760px
 * design.md bound rather than stretching across the workspace.
 */
export function StudioEmptyState({
  icon,
  title,
  description,
  actions,
  note,
  accent = 'cyan',
  compact = false,
  className,
}: StudioEmptyStateProps): ReactNode {
  return (
    <div className="flex min-h-[45vh] w-full items-center justify-center sm:min-h-[55vh]">
      <section
        aria-label={title}
        className={cn(
          'studio-panel studio-panel-raised flex w-full max-w-[47.5rem] flex-col items-center px-6 text-center sm:px-10',
          compact ? 'py-10' : 'py-14 sm:py-16',
          className,
        )}
      >
        <IconTile icon={icon} size="lg" depth="inset" tone={accent === 'magenta' ? 'stream' : 'primary'} />
        <h2 className="mt-5 font-display text-title font-bold uppercase text-fg-1">{title}</h2>
        <div className="mt-2 max-w-xl text-body text-fg-2">{description}</div>
        {actions ? (
          <div className="mt-7 flex w-full flex-col justify-center gap-3 sm:w-auto sm:flex-row">{actions}</div>
        ) : null}
        {note ? (
          <div className="mt-6 border-t border-border-subtle pt-4 font-mono text-meta uppercase text-fg-3">{note}</div>
        ) : null}
      </section>
    </div>
  );
}
