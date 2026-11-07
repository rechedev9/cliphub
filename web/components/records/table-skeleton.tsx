import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';
import { Skeleton } from '@/components/ui/skeleton';

/** Cell widths that cycle across the columns, so the placeholder reads as a table and not a bar chart. */
const CELL_WIDTHS = ['w-32', 'w-16', 'w-24', 'w-12', 'w-20', 'w-14'] as const;

/**
 * The loading shape of a `FilterBar` over a `DataTable`: the search field, the view
 * buttons, then a header and rows at the table's own heights, so the list lands in place.
 */
export function TableSkeleton({ label, columns = 4, rows = 6 }: { label: string; columns?: number; rows?: number }): ReactNode {
  const cells = Array.from({ length: columns }, (_, index) => CELL_WIDTHS[index % CELL_WIDTHS.length]);
  return (
    <div role="status" aria-label={label} className="flex flex-col gap-3">
      <Skeleton className="h-9 w-80 max-w-full" />
      <div className="flex flex-col gap-2">
        <div className="flex h-9 justify-end gap-2">
          <Skeleton className="h-9 w-20" />
          <Skeleton className="h-9 w-24" />
        </div>
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
    </div>
  );
}
