import Link from 'next/link';
import type { ReactNode } from 'react';
import { ChevronRight } from 'lucide-react';
import { cn } from '@/lib/utils';

export type RelatedListProps = {
  /** Plural noun of the related records, such as "Partidas de este equipo". */
  title: string;
  count?: number;
  actions?: ReactNode;
  /** Where the full list lives, when this panel shows only the first rows. */
  viewAll?: { href: string; label: string };
  children: ReactNode;
  className?: string;
};

/** A panel of records related to the one on screen, with its count and a door to the full list. */
export function RelatedList({ title, count, actions, viewAll, children, className }: RelatedListProps): ReactNode {
  return (
    <section aria-label={title} className={cn('studio-panel flex flex-col', className)}>
      <header className="flex min-h-11 items-center justify-between gap-3 border-b border-border px-4 py-2">
        <h2 className="flex items-baseline gap-2 text-body-sm font-semibold text-fg-1">
          {title}
          {count === undefined ? null : <span className="font-normal text-fg-3">({count})</span>}
        </h2>
        {actions ? <div className="flex items-center gap-2">{actions}</div> : null}
      </header>
      <div className="min-w-0">{children}</div>
      {viewAll ? (
        <footer className="border-t border-border px-4 py-2">
          <Link
            href={viewAll.href}
            className="inline-flex items-center gap-1 text-body-sm font-medium text-primary hover:underline"
          >
            {viewAll.label}
            <ChevronRight className="size-3.5" aria-hidden />
          </Link>
        </footer>
      ) : null}
    </section>
  );
}
