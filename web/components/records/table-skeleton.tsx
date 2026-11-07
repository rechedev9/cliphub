import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';
import { Skeleton } from '@/components/ui/skeleton';

/** Cell widths that cycle across the columns, so the placeholder reads as a table and not a bar chart. */
const CELL_WIDTHS = ['w-32', 'w-16', 'w-24', 'w-12', 'w-20', 'w-14'] as const;

/**
 * The loading shape of a `DataTable` with its `FilterBar`: the search beside the view buttons,
 * one select per filter below, then a header and rows at the table's own heights.
 */
export function TableSkeleton({ label, columns = 4, rows = 6, filters = 0 }: { label: string; columns?: number; rows?: number; filters?: number }): ReactNode {
  const cells = Array.from({ length: columns }, (_, index) => CELL_WIDTHS[index % CELL_WIDTHS.length]);
  return (
    <div role="status" aria-label={label} className="flex flex-col gap-2">
      <div className="flex h-9 items-center gap-2">
        <Skeleton className="h-9 w-80 max-w-full" />
        <Skeleton className="ml-auto h-9 w-20" />
        <Skeleton className="h-9 w-24" />
      </div>
      {filters > 0 ? (
        <div className="flex flex-wrap gap-2">
          {Array.from({ length: filters }, (_, index) => (
            <Skeleton key={index} className="h-9 w-36" />
          ))}
        </div>
      ) : null}
      <div className="studio-panel overflow-hidden">
        <div className="flex h-9 items-center gap-8 border-b border-border bg-surface-2 px-3">
          {cells.map((width, index) => (
            <Skeleton key={index} className={cn('h-2.5 shrink-0 opacity-60 shadow-none', width)} />
          ))}
        </div>
        {Array.from({ length: rows }, (_, row) => (
          <div key={row} className="flex h-10 items-center gap-8 border-b border-border-subtle px-3 last:border-b-0">
            {cells.map((width, index) => (
              <Skeleton key={index} className={cn('h-3 shrink-0 shadow-none', width)} />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}
